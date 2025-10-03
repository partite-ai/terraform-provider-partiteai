package main

import "strings"

type PackageMeta struct {
	Protocols           []string    `json:"protocols"`
	OS                  string      `json:"os"`
	Arch                string      `json:"arch"`
	Filename            string      `json:"filename"`
	DownloadURL         string      `json:"download_url"`
	ShasumsURL          string      `json:"shasums_url"`
	ShasumsSignatureURL string      `json:"shasums_signature_url"`
	Shasum              string      `json:"shasum"`
	SigningKeys         SigningKeys `json:"signing_keys"`
}

type SigningKeys struct {
	GPGPublicKeys []GPGPublicKey `json:"gpg_public_keys"`
}

type GPGPublicKey struct {
	KeyID      string `json:"key_id"`
	AsciiArmor string `json:"ascii_armor"`
}

type PackageAndArtifact struct {
	PackageMeta PackageMeta
	Artifact    Artifact
}

func createPackageMetas(
	artifacts []Artifact,
	urlPrefix string,
	checksumName string,
	signatureName string,
	keyID string,
	publicKey string,
) []*PackageAndArtifact {
	var metas []*PackageAndArtifact
	for _, artifact := range artifacts {
		if artifact.Type != "Archive" {
			continue
		}

		shasum := strings.TrimPrefix(artifact.Extra.Checksum, "sha256:")

		metas = append(metas, &PackageAndArtifact{
			PackageMeta: PackageMeta{
				Protocols:           []string{"6.0"},
				OS:                  artifact.OS,
				Arch:                artifact.Arch,
				Filename:            artifact.Name,
				DownloadURL:         urlPrefix + "/" + artifact.Name,
				ShasumsURL:          urlPrefix + "/" + checksumName,
				ShasumsSignatureURL: urlPrefix + "/" + signatureName,
				Shasum:              shasum,
				SigningKeys: SigningKeys{
					GPGPublicKeys: []GPGPublicKey{
						{
							KeyID:      keyID,
							AsciiArmor: publicKey,
						},
					},
				},
			},
			Artifact: artifact,
		})
	}
	return metas
}
