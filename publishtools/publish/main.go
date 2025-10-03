package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type VersionsFile struct {
	Versions []any `json:"versions"`
}

func main() {
	var bucket, namespace, domain, providerName, directory string

	flag.StringVar(&bucket, "bucket", "", "S3 bucket name")
	flag.StringVar(&namespace, "namespace", "", "Provider namespace")
	flag.StringVar(&domain, "domain", "", "Domain to replace $DOMAIN with")
	flag.StringVar(&providerName, "provider", "", "Provider name")
	flag.StringVar(&directory, "dir", "", "Directory containing provider files")
	flag.Parse()

	if bucket == "" || namespace == "" || domain == "" || providerName == "" || directory == "" {
		log.Fatal("All flags are required: -bucket, -namespace, -domain, -provider, -dir")
	}

	gpgFingerprint := os.Getenv("GPG_FINGERPRINT")
	if gpgFingerprint == "" {
		log.Fatal("GPG_FINGERPRINT environment variable is required")
	}

	gpgKeyFile, err := os.Open(filepath.Join(directory, "gpg.pub"))
	if err != nil {
		log.Fatalf("Failed to open GPG public key file: %v", err)
	}
	defer gpgKeyFile.Close()

	gpgPublicKey, err := io.ReadAll(gpgKeyFile)
	if err != nil {
		log.Fatalf("Failed to read GPG public key file: %v", err)
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	metadata, err := loadMetadata(filepath.Join(directory, "metadata.json"))
	if err != nil {
		log.Fatalf("Failed to load metadata: %v", err)
	}

	artifacts, err := loadArtifacts(filepath.Join(directory, "artifacts.json"))
	if err != nil {
		log.Fatalf("Failed to load artifacts: %v", err)
	}

	versionMeta, err := createVersionMeta(metadata.Version, artifacts)
	if err != nil {
		log.Fatalf("Failed to create version meta: %v", err)
	}

	if err := processVersionMeta(ctx, s3Client, bucket, namespace, providerName, versionMeta); err != nil {
		log.Fatalf("Failed to process version meta: %v", err)
	}

	sigFileName, checksumFileName, err := processSignatures(ctx, s3Client, bucket, namespace, providerName, metadata.Version, directory, artifacts)
	if err != nil {
		log.Fatalf("Failed to process signatures: %v", err)
	}

	packageMetas := createPackageMetas(artifacts, fmt.Sprintf("https://%s/v1/providers/%s/%s/%s", domain, namespace, providerName, metadata.Version), checksumFileName, sigFileName, gpgFingerprint, string(gpgPublicKey))

	if err := processPackages(ctx, s3Client, bucket, namespace, providerName, metadata.Version, directory, packageMetas); err != nil {
		log.Fatalf("Failed to process packages: %v", err)
	}

	fmt.Println("Successfully published provider to S3")
}

func processVersionMeta(ctx context.Context, s3Client *s3.Client, bucket, namespace, providerName string, meta *VersionMeta) error {
	versionsKey := fmt.Sprintf("v1/providers/%s/%s/versions", namespace, providerName)

	// Retry loop for conditional updates
	maxRetries := 10
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
		}

		// Try to get existing versions file with ETag
		var versionsFile VersionsFile
		var etag *string
		getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(versionsKey),
		})

		if err != nil {
			// Check if it's a not found error
			var nsk *types.NoSuchKey
			if !errors.As(err, &nsk) {
				return fmt.Errorf("failed to check versions file: %v", err)
			}
			// File doesn't exist, create new
			versionsFile = VersionsFile{Versions: []any{meta}}
			etag = nil
		} else {
			// File exists, read and append
			etag = getResp.ETag
			existingData, err := io.ReadAll(getResp.Body)
			getResp.Body.Close()
			if err != nil {
				return fmt.Errorf("failed to read existing versions file: %v", err)
			}

			if err := json.Unmarshal(existingData, &versionsFile); err != nil {
				return fmt.Errorf("failed to parse existing versions file: %v", err)
			}

			versionsFile.Versions = append(versionsFile.Versions, meta)
		}

		// Upload updated versions file with conditional request
		updatedData, err := json.Marshal(versionsFile)
		if err != nil {
			return fmt.Errorf("failed to marshal versions file: %v", err)
		}

		putInput := &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(versionsKey),
			Body:   strings.NewReader(string(updatedData)),
		}

		// Add conditional request headers
		if etag != nil {
			putInput.IfMatch = etag
		} else {
			putInput.IfNoneMatch = aws.String("*")
		}

		_, err = s3Client.PutObject(ctx, putInput)
		if err != nil {
			// Check if it's a precondition failed error (concurrent modification)
			var ae smithy.APIError
			if errors.As(err, &ae) && ae.ErrorCode() == "PreconditionFailed" {
				continue // Retry the operation
			}
			return fmt.Errorf("failed to upload versions file: %v", err)
		}

		// Success
		return nil
	}

	return fmt.Errorf("failed to update versions file after %d attempts due to concurrent modifications", maxRetries)
}

func processSignatures(ctx context.Context, s3Client *s3.Client, bucket, namespace, providerName, version, directory string, artifacts []Artifact) (signatureFile, checksumFile string, err error) {
	for _, artifact := range artifacts {
		var isSignature, isChecksum bool

		// Identify signature and checksum files by type
		if artifact.Type == "Signature" {
			isSignature = true
			signatureFile = artifact.Name
		} else if artifact.Type == "Checksum" {
			isChecksum = true
			checksumFile = artifact.Name
		}

		if isSignature || isChecksum {
			// Read file content
			filePath := filepath.Join(directory, artifact.Name)
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				return "", "", fmt.Errorf("failed to read file %s: %v", artifact.Name, err)
			}

			// Upload to S3
			s3Key := fmt.Sprintf("v1/providers/%s/%s/%s/%s", namespace, providerName, version, artifact.Name)
			_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(s3Key),
				Body:   strings.NewReader(string(fileContent)),
			})
			if err != nil {
				return "", "", fmt.Errorf("failed to upload %s to S3: %v", artifact.Name, err)
			}
		}
	}

	if signatureFile == "" || checksumFile == "" {
		return "", "", fmt.Errorf("signature or checksum file not found in artifacts")
	}

	return signatureFile, checksumFile, nil
}

func processPackages(ctx context.Context, s3Client *s3.Client, bucket, namespace, providerName, version, directory string, packageMetas []*PackageAndArtifact) error {
	for _, packageMeta := range packageMetas {
		// Upload artifact file
		filePath := filepath.Join(directory, packageMeta.Artifact.Name)
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read artifact file %s: %v", packageMeta.Artifact.Name, err)
		}

		// Upload artifact to S3
		artifactKey := fmt.Sprintf("v1/providers/%s/%s/%s/%s", namespace, providerName, version, packageMeta.Artifact.Name)
		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(artifactKey),
			Body:   strings.NewReader(string(fileContent)),
		})
		if err != nil {
			return fmt.Errorf("failed to upload artifact %s to S3: %v", packageMeta.Artifact.Name, err)
		}

		// Upload package metadata as JSON
		metaJSON, err := json.Marshal(packageMeta.PackageMeta)
		if err != nil {
			return fmt.Errorf("failed to marshal package metadata for %s: %v", packageMeta.Artifact.Name, err)
		}

		metaKey := fmt.Sprintf("v1/providers/%s/%s/%s/download/%s/%s", namespace, providerName, version, packageMeta.PackageMeta.OS, packageMeta.PackageMeta.Arch)
		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(metaKey),
			Body:   strings.NewReader(string(metaJSON)),
		})
		if err != nil {
			return fmt.Errorf("failed to upload package metadata for %s/%s to S3: %v", packageMeta.PackageMeta.OS, packageMeta.PackageMeta.Arch, err)
		}
	}

	return nil
}
