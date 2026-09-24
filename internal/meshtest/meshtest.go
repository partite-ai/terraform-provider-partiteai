// Package meshtest runs a real Partite Mesh server in local mode so provider
// tests can exercise the actual GraphQL API instead of a hand-written fake.
//
// Local mode keeps everything in SQLite files under a temp dir and makes no
// outbound calls, so each test gets a clean, hermetic backend.
package meshtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// MeshDirEnvVar points at a mesh checkout. It defaults to a sibling of the
// provider repo.
const MeshDirEnvVar = "PARTITE_MESH_DIR"

// ServerBinEnvVar points at an already-built mesh server binary and skips the
// build step.
const ServerBinEnvVar = "PARTITE_MESH_SERVER"

// Server is a running mesh instance owned by one test.
type Server struct {
	Endpoint string // GraphQL endpoint, e.g. http://127.0.0.1:53211/graphql
	BaseURL  string

	// APIKey and APISecret are a real key minted on this instance, so tests
	// go through the same authentication path as a real deployment.
	APIKey    string
	APISecret string

	cmd     *exec.Cmd
	logPath string
}

var (
	buildOnce sync.Once
	buildBin  string
	buildErr  error
)

// Start brings up a mesh server for the duration of the test. It registers its
// own cleanup and skips the test when no mesh checkout is available.
func Start(t *testing.T) *Server {
	t.Helper()

	if os.Getenv(SkipEnvVar) != "" {
		t.Skipf("%s is set", SkipEnvVar)
	}

	bin, err := serverBinary()
	if err != nil {
		t.Skipf("mesh server unavailable: %v\n"+
			"Set %s to a mesh checkout or %s to a prebuilt binary, "+
			"or set %s=1 to skip backend tests.",
			err, MeshDirEnvVar, ServerBinEnvVar, SkipEnvVar)
	}

	dir := t.TempDir()
	port, err := freePort()
	if err != nil {
		t.Fatalf("pick a port: %v", err)
	}

	logPath := filepath.Join(dir, "mesh.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create log: %v", err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(),
		"CONFIGURATION_MODE=local",
		"REQUIRE_AUTH=false",
		fmt.Sprintf("SERVER_PORT=%d", port),
		"DB_URL="+filepath.Join(dir, "mesh.db"),
		"MCP_QUEUE_DB_URL="+filepath.Join(dir, "mcpqueue.db"),
		"MEMORY_DB_URL="+filepath.Join(dir, "memory.db"),
		"FILESTORE_ROOT_DIR="+filepath.Join(dir, "files"),
		"PARTICLE_BLOB_CACHE_DIR="+filepath.Join(dir, "blobs"),
	)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		t.Fatalf("start mesh: %v", err)
	}

	s := &Server{
		BaseURL:  fmt.Sprintf("http://127.0.0.1:%d", port),
		Endpoint: fmt.Sprintf("http://127.0.0.1:%d/graphql", port),
		cmd:      cmd,
		logPath:  logPath,
	}

	t.Cleanup(func() {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
			_, _ = s.cmd.Process.Wait()
		}
		logFile.Close()
		if t.Failed() {
			if out, err := os.ReadFile(logPath); err == nil {
				t.Logf("mesh server log:\n%s", tail(string(out), 100))
			}
		}
	})

	if err := s.waitReady(30 * time.Second); err != nil {
		if out, readErr := os.ReadFile(logPath); readErr == nil {
			t.Logf("mesh server log:\n%s", tail(string(out), 50))
		}
		t.Fatalf("mesh never became ready: %v", err)
	}

	if err := s.mintAPIKey(); err != nil {
		t.Fatalf("mint API key: %v", err)
	}

	return s
}

// mintAPIKey creates the key the provider authenticates with. The request
// itself carries no credentials, which the API allows, and which is the only
// way to bootstrap a fresh local instance.
func (s *Server) mintAPIKey() error {
	const query = `mutation { createApiKey(name: "terraform-acceptance-tests", scopes: ["configuration:read", "configuration:write", "mesh:converse"]) { apiKey { kid } secret } }`

	var out struct {
		Data struct {
			CreateApiKey struct {
				ApiKey struct {
					Kid string `json:"kid"`
				} `json:"apiKey"`
				Secret string `json:"secret"`
			} `json:"createApiKey"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("decode %s: %w", raw, err)
	}
	if len(out.Errors) > 0 {
		return fmt.Errorf("graphql: %s", out.Errors[0].Message)
	}

	s.APIKey = out.Data.CreateApiKey.ApiKey.Kid
	s.APISecret = out.Data.CreateApiKey.Secret
	if s.APIKey == "" || s.APISecret == "" {
		return fmt.Errorf("empty key in response: %s", raw)
	}
	return nil
}

func (s *Server) Name() string { return "mesh" }

// ProviderConfig is a provider block pointed at this server, to prepend to a
// test's Terraform configuration.
func (s *Server) ProviderConfig() string {
	return fmt.Sprintf(`
provider "partiteai" {
  endpoint   = %q
  api_key    = %q
  api_secret = %q
}
`, s.Endpoint, s.APIKey, s.APISecret)
}

func (s *Server) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	body := strings.NewReader(`{"query":"{__typename}"}`)
	for time.Now().Before(deadline) {
		if s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited() {
			return fmt.Errorf("server exited: %s", s.cmd.ProcessState)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, body)
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				cancel()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
		cancel()
		body.Seek(0, 0) //nolint:errcheck
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}

// serverBinary builds the mesh server once per test binary and returns its path.
func serverBinary() (string, error) {
	buildOnce.Do(func() {
		if bin := os.Getenv(ServerBinEnvVar); bin != "" {
			if _, err := os.Stat(bin); err != nil {
				buildErr = fmt.Errorf("%s=%s: %w", ServerBinEnvVar, bin, err)
				return
			}
			buildBin = bin
			return
		}

		meshDir, err := siblingDir(MeshDirEnvVar, "mesh", filepath.Join("cmd", "server"))
		if err != nil {
			buildErr = err
			return
		}
		buildBin, buildErr = buildBinary(meshDir, "./cmd/server", "mesh-server")
	})
	return buildBin, buildErr
}

// ProviderConfigEnterprise is a provider block pointed at this server that
// claims to be an enterprise deployment, for testing what the flag does rather
// than what the endpoint implies.
func (s *Server) ProviderConfigEnterprise() string {
	return fmt.Sprintf(`
provider "partiteai" {
  endpoint   = %q
  api_key    = %q
  api_secret = %q
  enterprise = true
}
`, s.Endpoint, s.APIKey, s.APISecret)
}
