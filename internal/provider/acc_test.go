package provider

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/partite-ai/terraform-provider-partiteai/internal/meshtest"
)

// BackendsEnvVar selects which backends the acceptance tests run against:
// "mesh" (the default), "enterprise", or "all".
const BackendsEnvVar = "PARTITE_TEST_BACKENDS"

// TestMain turns acceptance tests on by default. They run against a backend
// this package starts itself, so there is no cloud account, no cost and no
// shared state to protect — the usual reason to gate them behind an opt-in does
// not apply here.
func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC") == "" {
		os.Setenv("TF_ACC", "1")
	}
	code := m.Run()
	// The enterprise stack is shared by the whole binary, so it comes down once
	// everything has finished with it.
	meshtest.ShutdownEnterprise()
	os.Exit(code)
}

// The provider uses write-only attributes, which Terraform gained in 1.11.
var minTerraform = []tfversion.TerraformVersionCheck{
	tfversion.SkipBelow(tfversion.Version1_11_0),
}

// enabledBackends is the set the current run covers.
func enabledBackends() []string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(BackendsEnvVar))) {
	case "", "mesh":
		return []string{"mesh"}
	case "enterprise":
		return []string{"enterprise"}
	case "all", "both":
		return []string{"mesh", "enterprise"}
	default:
		return strings.Split(strings.ToLower(os.Getenv(BackendsEnvVar)), ",")
	}
}

// backendEnabled reports whether this run covers the named backend.
func backendEnabled(name string) bool {
	for _, b := range enabledBackends() {
		if strings.TrimSpace(b) == name {
			return true
		}
	}
	return false
}

// startBackend brings up the named backend for one test.
func startBackend(t *testing.T, name string) meshtest.Backend {
	t.Helper()
	switch strings.TrimSpace(name) {
	case "mesh":
		return meshtest.Start(t)
	case "enterprise":
		return meshtest.StartEnterprise(t)
	default:
		t.Fatalf("unknown backend %q in %s", name, BackendsEnvVar)
		return nil
	}
}

// accTest runs the steps against every enabled backend, each on a backend of
// its own. Every step gets the checks that catch the regressions that actually
// bite a provider: a plan that is not empty after apply, and state that does
// not survive a refresh.
func accTest(t *testing.T, steps ...resource.TestStep) {
	t.Helper()

	for _, name := range enabledBackends() {
		t.Run(name, func(t *testing.T) {
			runAcc(t, startBackend(t, name), steps)
		})
	}
}

// accTestEnterprise runs the steps against the enterprise backend only, for
// the resources that exist nowhere else. It skips when that backend is not in
// this run.
func accTestEnterprise(t *testing.T, steps ...resource.TestStep) {
	t.Helper()

	if !backendEnabled("enterprise") {
		t.Skipf("set %s=enterprise or %s=all to run this", BackendsEnvVar, BackendsEnvVar)
	}
	runAcc(t, meshtest.StartEnterprise(t), steps)
}

func runAcc(t *testing.T, backend meshtest.Backend, steps []resource.TestStep) {
	t.Helper()

	// The steps get a copy: the provider block differs per backend, and a
	// shared slice would carry one backend's endpoint into another's run.
	run := make([]resource.TestStep, len(steps))
	copy(run, steps)

	for i := range run {
		run[i].Config = backend.ProviderConfig() + run[i].Config
		if run[i].ImportState || run[i].ExpectError != nil {
			continue
		}
		run[i].ConfigPlanChecks = resource.ConfigPlanChecks{
			PostApplyPostRefresh: []plancheck.PlanCheck{
				plancheck.ExpectEmptyPlan(),
			},
		}
	}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   minTerraform,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    run,
	})
}
