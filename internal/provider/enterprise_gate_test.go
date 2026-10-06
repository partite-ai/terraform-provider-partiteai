package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/partite-ai/terraform-provider-partiteai/internal/meshtest"
)

// TestIsEnterpriseEndpoint pins the default the provider works out from the
// endpoint. It reads the host, not the whole URL, so a domain that merely
// contains the suffix does not pass for one.
func TestIsEnterpriseEndpoint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		endpoint string
		want     bool
	}{
		{"https://api.partite.ai/graphql", true},
		{"https://eu.api.partite.ai/graphql", true},
		{"https://API.PARTITE.AI/graphql", true},
		{"https://api.partite.ai:8443/graphql", true},

		{"http://127.0.0.1:8080/graphql", false},
		{"http://localhost:8080/graphql", false},
		{"https://mesh.example.com/graphql", false},
		// The apex on its own does not end with the suffix.
		{"https://partite.ai/graphql", false},
		// A host that merely ends with the same letters is not the same host.
		{"https://notpartite.ai/graphql", false},
		{"https://evil-partite.ai/graphql", false},
		// The suffix in the path or as a prefix of a longer host is not it.
		{"https://example.com/partite.ai/graphql", false},
		{"https://api.partite.ai.example.com/graphql", false},
		{"", false},
		{"://not a url", false},
	}

	for _, c := range cases {
		if got := isEnterpriseEndpoint(c.endpoint); got != c.want {
			t.Errorf("isEnterpriseEndpoint(%q) = %v, want %v", c.endpoint, got, c.want)
		}
	}
}

// TestAccEval_needsEnterprise pins what happens when an evaluation resource is
// pointed at a deployment that does not serve one: the provider says so itself,
// rather than letting the API answer with a field it has never heard of.
func TestAccEval_needsEnterprise(t *testing.T) {
	if !backendEnabled("mesh") {
		t.Skipf("set %s=mesh or %s=all to run this", BackendsEnvVar, BackendsEnvVar)
	}

	server := meshtest.Start(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   minTerraform,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: server.ProviderConfig() + `
resource "partiteai_rubric" "test" {
  name    = "no-enterprise"
  metrics = [{ computed_metric_key = "latency" }]
}
`,
				ExpectError: regexp.MustCompile(`(?s)partiteai_rubric is only available`),
			},
			{
				Config: server.ProviderConfig() + `
data "partiteai_computed_metric" "test" {
  key = "latency"
}
`,
				ExpectError: regexp.MustCompile(`(?s)partiteai_computed_metric is only available`),
			},
			{
				Config: server.ProviderConfig() + `
resource "partiteai_judge_metric" "test" {
  name       = "no-enterprise"
  category   = "QUALITY"
  scope      = "HOP"
  prompt     = "p"
  complexity = "LOW"
}
`,
				ExpectError: regexp.MustCompile(`(?s)partiteai_judge_metric is only available`),
			},
		},
	})
}

// TestAccEval_enterpriseFlagOverrides pins that the flag, not the host name,
// has the last word: a local deployment that says it is an enterprise one gets
// past the gate and reaches the API.
func TestAccEval_enterpriseFlagOverrides(t *testing.T) {
	if !backendEnabled("mesh") {
		t.Skipf("set %s=mesh or %s=all to run this", BackendsEnvVar, BackendsEnvVar)
	}

	server := meshtest.Start(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   minTerraform,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: server.ProviderConfigEnterprise() + `
data "partiteai_computed_metric" "test" {
  key = "latency"
}
`,
			// Past the gate, so the failure now comes from the API, which on a
			// mesh has no such field at all.
			ExpectError: regexp.MustCompile(`(?s)API Error`),
		}},
	})
}
