package meshtest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// The enterprise backend is not open source, so nothing here describes how it
// runs. That checkout ships a test harness command that owns every detail:
// this file starts it, reads JSON, and asks it for a tenant per test. The
// contract is documented in that repository, under cmd/tf-test-harness.

// EnterpriseDirEnvVar points at a mesh-enterprise checkout. It defaults to a
// sibling of the provider repo.
const EnterpriseDirEnvVar = "PARTITE_MESH_ENTERPRISE_DIR"

// EnterpriseHarnessEnvVar points at an already-built harness binary and skips
// the build step.
const EnterpriseHarnessEnvVar = "PARTITE_TF_TEST_HARNESS"

// harnessPackage is the command the checkout ships for this purpose.
const harnessPackage = "./cmd/tf-test-harness"

// Enterprise is one tenant of a running enterprise backend. Each test gets its
// own, so it sees an empty world however many tests ran before it.
type Enterprise struct {
	Endpoint string
	BaseURL  string

	// TenantID identifies the tenant in test output. It is opaque.
	TenantID  string
	APIKey    string
	APISecret string
}

func (e *Enterprise) Name() string { return "enterprise" }

func (e *Enterprise) ProviderConfig() string {
	// The harness serves an enterprise backend on a local address, which the
	// provider would otherwise take for a self-hosted mesh.
	return fmt.Sprintf(`
provider "partiteai" {
  endpoint   = %q
  api_key    = %q
  api_secret = %q
  enterprise = true
}
`, e.Endpoint, e.APIKey, e.APISecret)
}

// harness is the running harness process. Starting it is the expensive part,
// so one serves the whole test binary and tests isolate themselves by tenant.
type harness struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	errPath string

	endpoint string
	baseURL  string

	// mu serializes requests: there is one pipe, and tests may run in parallel.
	mu sync.Mutex
}

var (
	harnessOnce  sync.Once
	harnessValue *harness
	harnessErr   error
)

// StartEnterprise gives the test its own tenant on a shared enterprise backend,
// starting that backend on first use. It skips the test when the harness is
// unavailable.
func StartEnterprise(t *testing.T) *Enterprise {
	t.Helper()

	if os.Getenv(SkipEnvVar) != "" {
		t.Skipf("%s is set", SkipEnvVar)
	}

	harnessOnce.Do(func() {
		harnessValue, harnessErr = startHarness()
	})
	if harnessErr != nil {
		t.Skipf("enterprise backend unavailable: %v\n"+
			"It needs Docker and a mesh-enterprise checkout (%s), "+
			"or set %s=1 to skip backend tests.",
			harnessErr, EnterpriseDirEnvVar, SkipEnvVar)
	}

	tenant, err := harnessValue.tenant()
	if err != nil {
		t.Fatalf("ask the harness for a tenant: %v\n%s",
			err, tail(harnessValue.readStderr(), 30))
	}
	return tenant
}

// ShutdownEnterprise closes the harness, which destroys everything it created.
// Call it from TestMain.
func ShutdownEnterprise() {
	if harnessValue == nil {
		return
	}
	harnessValue.stop()
	harnessValue = nil
}

func startHarness() (*harness, error) {
	bin, err := harnessBinary()
	if err != nil {
		return nil, err
	}

	dir, err := enterpriseDir()
	if err != nil {
		return nil, err
	}

	errFile, err := os.CreateTemp("", "tfharness-stderr")
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(bin)
	// The harness reads its checkout from the working directory.
	cmd.Dir = dir
	cmd.Stderr = errFile

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start harness: %w", err)
	}

	h := &harness{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		errPath: errFile.Name(),
	}

	// Bringing the backend up takes a while, and the harness says so with one
	// line when it is ready.
	ready, err := h.await(3 * time.Minute)
	if err != nil {
		h.stop()
		return nil, fmt.Errorf("%w\n%s", err, tail(h.readStderr(), 40))
	}
	if ready.Ready == nil {
		h.stop()
		return nil, fmt.Errorf("harness did not report ready\n%s", tail(h.readStderr(), 40))
	}

	h.endpoint = ready.Ready.Endpoint
	h.baseURL = ready.Ready.BaseURL
	return h, nil
}

// harnessResponse is one line the harness writes. Exactly one field is set.
type harnessResponse struct {
	Ready *struct {
		Endpoint string `json:"endpoint"`
		BaseURL  string `json:"base_url"`
	} `json:"ready,omitempty"`
	Tenant *struct {
		ID        string `json:"id"`
		APIKey    string `json:"api_key"`
		APISecret string `json:"api_secret"`
	} `json:"tenant,omitempty"`
	Error string `json:"error,omitempty"`
}

// await reads one response, giving up after timeout.
func (h *harness) await(timeout time.Duration) (*harnessResponse, error) {
	type result struct {
		line []byte
		err  error
	}
	lines := make(chan result, 1)
	go func() {
		line, err := h.stdout.ReadBytes('\n')
		lines <- result{line, err}
	}()

	select {
	case got := <-lines:
		if got.err != nil {
			return nil, fmt.Errorf("read from harness: %w", got.err)
		}
		var resp harnessResponse
		if err := json.Unmarshal(got.line, &resp); err != nil {
			return nil, fmt.Errorf("cannot read harness response %q: %w", got.line, err)
		}
		if resp.Error != "" {
			return nil, fmt.Errorf("harness: %s", resp.Error)
		}
		return &resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("harness did not answer within %s", timeout)
	}
}

func (h *harness) tenant() (*Enterprise, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, err := io.WriteString(h.stdin, `{"request":"tenant"}`+"\n"); err != nil {
		return nil, fmt.Errorf("ask for a tenant: %w", err)
	}

	resp, err := h.await(2 * time.Minute)
	if err != nil {
		return nil, err
	}
	if resp.Tenant == nil {
		return nil, fmt.Errorf("harness answered without a tenant")
	}

	return &Enterprise{
		Endpoint:  h.endpoint,
		BaseURL:   h.baseURL,
		TenantID:  resp.Tenant.ID,
		APIKey:    resp.Tenant.APIKey,
		APISecret: resp.Tenant.APISecret,
	}, nil
}

// stop closes stdin, which is how the harness is told to tear everything down.
func (h *harness) stop() {
	if h.stdin != nil {
		_ = h.stdin.Close()
	}
	if h.cmd == nil || h.cmd.Process == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		_, _ = h.cmd.Process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Minute):
		_ = h.cmd.Process.Kill()
		<-done
	}
}

func (h *harness) readStderr() string {
	if h.errPath == "" {
		return ""
	}
	out, err := os.ReadFile(h.errPath)
	if err != nil {
		return ""
	}
	return string(out)
}

func enterpriseDir() (string, error) {
	return siblingDir(EnterpriseDirEnvVar, "mesh-enterprise", filepath.Join("cmd", "tf-test-harness"))
}

func harnessBinary() (string, error) {
	if bin := os.Getenv(EnterpriseHarnessEnvVar); bin != "" {
		if _, err := os.Stat(bin); err != nil {
			return "", fmt.Errorf("%s=%s: %w", EnterpriseHarnessEnvVar, bin, err)
		}
		return bin, nil
	}

	if _, err := exec.LookPath("docker"); err != nil {
		return "", fmt.Errorf("docker not on PATH: %w", err)
	}

	dir, err := enterpriseDir()
	if err != nil {
		return "", err
	}
	return buildBinary(dir, harnessPackage, "tf-test-harness")
}
