package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/partite-ai/terraform-provider-partiteai/internal/meshtest"
)

// TestAccEnterprise_tenantsAreIsolated proves the harness really does hand each
// test an empty world, which is what lets one enterprise backend serve the
// whole test binary. Two tenants each create a resource under the same name,
// which one tenant could not do twice, and neither may see the other's.
//
// It goes over the API rather than through Terraform because a single Terraform
// run cannot hold two provider configurations here — see the note on provider
// aliases in TESTING.md.
func TestAccEnterprise_tenantsAreIsolated(t *testing.T) {
	if !backendEnabled("enterprise") {
		t.Skipf("set %s=enterprise or %s=all to run this", BackendsEnvVar, BackendsEnvVar)
	}

	first := meshtest.StartEnterprise(t)
	second := meshtest.StartEnterprise(t)

	if first.TenantID == second.TenantID {
		t.Fatalf("both tenants landed in %s", first.TenantID)
	}
	if first.APIKey == second.APIKey {
		t.Fatalf("both tenants share API key %s", first.APIKey)
	}
	if first.Endpoint != second.Endpoint {
		t.Fatalf("tenants are on different servers: %s and %s", first.Endpoint, second.Endpoint)
	}

	const create = `mutation { createModelProvider(input: {name: "shared-name", providerType: OPEN_AI, providerConfig: {openAI: {apiKey: "sk-test"}}}) { id name } }`

	var firstCreated, secondCreated struct {
		CreateModelProvider struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"createModelProvider"`
	}

	graphQL(t, first, create, &firstCreated)
	graphQL(t, second, create, &secondCreated)

	if firstCreated.CreateModelProvider.Id == secondCreated.CreateModelProvider.Id {
		t.Fatalf("both tenants got the same resource id %s", firstCreated.CreateModelProvider.Id)
	}

	// Each tenant sees exactly the one row it made.
	for _, tenant := range []*meshtest.Enterprise{first, second} {
		var counted struct {
			ModelProviders struct {
				TotalCount int `json:"totalCount"`
			} `json:"modelProviders"`
		}
		graphQL(t, tenant, `{ modelProviders(first: 50) { totalCount } }`, &counted)
		if got := counted.ModelProviders.TotalCount; got != 1 {
			t.Errorf("tenant %s sees %d model providers, want only its own", tenant.TenantID, got)
		}
	}
}

// TestAccEnterprise_apiKeyIsRequired pins that the enterprise API is not
// reachable without credentials. The local mesh runs with authorization off, so
// this is the only backend that can check it.
func TestAccEnterprise_apiKeyIsRequired(t *testing.T) {
	if !backendEnabled("enterprise") {
		t.Skipf("set %s=enterprise or %s=all to run this", BackendsEnvVar, BackendsEnvVar)
	}

	tenant := meshtest.StartEnterprise(t)

	body, err := json.Marshal(map[string]string{"query": `{ modelProviders(first: 1) { totalCount } }`})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tenant.Endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(raw, []byte("configuration:read")) {
		t.Errorf("an unauthenticated read should be refused for want of a scope, got: %s", raw)
	}
}

// TestAccEnterprise_meshChain runs the whole configuration chain against the
// enterprise backend in one apply, so the multi-tenant write path is covered
// end to end and not only resource by resource.
func TestAccEnterprise_meshChain(t *testing.T) {
	if !backendEnabled("enterprise") {
		t.Skipf("set %s=enterprise or %s=all to run this", BackendsEnvVar, BackendsEnvVar)
	}

	tenant := meshtest.StartEnterprise(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   minTerraform,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tenant.ProviderConfig() + baseMesh + baseAgent + `
resource "partiteai_mesh_input" "entry" {
  mesh_id  = partiteai_mesh.base.id
  agent_id = partiteai_agent.base.id
  name     = "entry"

  responses = [
    {
      name        = "done"
      description = "Finished."
      message_type = jsonencode({
        type       = "object"
        properties = { answer = { type = "string", description = "The answer." } }
        required   = ["answer"]
      })
    },
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("partiteai_mesh.base", "draft_version_id"),
					resource.TestCheckResourceAttr("partiteai_mesh_input.entry", "name", "entry"),
				),
			},
		},
	})
}

// graphQL runs one query as a tenant and decodes its data into out.
func graphQL(t *testing.T, tenant *meshtest.Enterprise, query string, out any) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tenant.Endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(tenant.APIKey, tenant.APISecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if len(envelope.Errors) > 0 {
		t.Fatalf("graphql: %s", envelope.Errors[0].Message)
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		t.Fatalf("decode data %s: %v", envelope.Data, err)
	}
}
