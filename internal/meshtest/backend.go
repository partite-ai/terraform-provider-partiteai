package meshtest

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Backend is a running Partite API that provider tests can point Terraform at.
// A test does not care which one it got.
type Backend interface {
	// Name identifies the backend in test output, e.g. "mesh" or "enterprise".
	Name() string
	// ProviderConfig is a provider block pointed at this backend, to prepend to
	// a test's Terraform configuration.
	ProviderConfig() string
}

// SkipEnvVar disables every test that needs a backend.
const SkipEnvVar = "PARTITE_SKIP_MESH_TESTS"

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("listener address is %T, not TCP", l.Addr())
	}
	return addr.Port, nil
}

func tail(s string, lines int) string {
	parts := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(parts) <= lines {
		return s
	}
	return strings.Join(parts[len(parts)-lines:], "\n")
}

// buildBinary compiles one command of a Go module into a temp directory.
func buildBinary(moduleDir, pkg, name string) (string, error) {
	out, err := os.MkdirTemp("", "meshtest-bin")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(out, name)

	cmd := exec.Command("go", "build", "-o", bin, pkg)
	cmd.Dir = moduleDir
	if combined, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build %s in %s: %w\n%s", pkg, moduleDir, err, combined)
	}
	return bin, nil
}

// siblingDir finds a checkout named dirName beside this repo, walking up from
// the working directory. envVar overrides the search.
func siblingDir(envVar, dirName, mustContain string) (string, error) {
	if dir := os.Getenv(envVar); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, mustContain)); err != nil {
			return "", fmt.Errorf("%s=%s has no %s: %w", envVar, dir, mustContain, err)
		}
		return dir, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; {
		candidate := filepath.Join(dir, "..", dirName, mustContain)
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(filepath.Join(dir, "..", dirName)), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no %s checkout found near %s", dirName, wd)
}
