package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccMcpConnection_httpCredentials covers the write-only credential blocks.
// OAUTH2_CLIENT_CREDENTIALS is left out on purpose: creating one fetches the
// authorization server's metadata document, so it cannot run without network
// access to the URL in the configuration.
// Write-only values never enter state, so the only thing that can be checked is
// that apply succeeds and the plan settles — which is exactly the regression
// that a mistake here produces.
func TestAccMcpConnection_httpCredentials(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: mcpConnectionConfig,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("partiteai_mcp_connection.basic", "http_transport.authentication_method", "BASIC"),
			resource.TestCheckResourceAttr("partiteai_mcp_connection.header", "http_transport.authentication_method", "HEADER"),
			resource.TestCheckResourceAttrSet("partiteai_mcp_connection.none", "id"),
			resource.TestCheckResourceAttrSet("partiteai_mcp_connection.tunnel", "tunnel_transport.%"),
			resource.TestCheckResourceAttr("partiteai_mcp_connection_subscription.dev", "criteria.0.name", "env"),
			resource.TestCheckResourceAttr("partiteai_mcp_connection_subscription.dev", "criteria.0.value", "dev"),
			// No criteria reads back null rather than an empty set, matching an
			// omitted attribute.
			resource.TestCheckNoResourceAttr("partiteai_mcp_connection_subscription.catch_all", "criteria"),
			// The credential exists once, at create, and is held in state.
			resource.TestCheckResourceAttrSet("partiteai_mcp_connection_tunnel_key.bob", "credential"),
			resource.TestCheckResourceAttr("partiteai_mcp_connection_tunnel_key.bob", "label", "laptop-bob"),
			resource.TestCheckResourceAttrSet("partiteai_mcp_connection.tunnel", "id"),
		),
	}, resource.TestStep{
		// A key imports by its connection and its own id, and finds its
		// subscription. The credential is not readable again.
		Config:                  mcpConnectionConfig,
		ResourceName:            "partiteai_mcp_connection_tunnel_key.bob",
		ImportState:             true,
		ImportStateIdFunc:       childImportID("partiteai_mcp_connection_tunnel_key.bob", "mcp_connection_id"),
		ImportStateVerify:       true,
		ImportStateVerifyIgnore: []string{"credential"},
	})
}

// childImportID builds the id a resource reached through a parent imports by:
// the parent's id, then its own.
func childImportID(name, parentAttr string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("%s not in state", name)
		}
		return rs.Primary.Attributes[parentAttr] + ":" + rs.Primary.ID, nil
	}
}

const mcpConnectionConfig = `
resource "partiteai_mcp_connection" "basic" {
  name = "acc-http-basic"
  http_transport = {
    url                   = "https://example.invalid/basic"
    authentication_method = "BASIC"
    basic_credentials_wo = {
      username = "user"
      password = "pass"
    }
  }
}

resource "partiteai_mcp_connection" "header" {
  name = "acc-http-header"
  http_transport = {
    url                   = "https://example.invalid/header"
    authentication_method = "HEADER"
    header_credentials_wo = {
      header_name  = "X-Api-Key"
      header_value = "secret"
    }
  }
}

resource "partiteai_mcp_connection" "none" {
  name = "acc-http-none"
  http_transport = {
    url                   = "https://example.invalid/none"
    authentication_method = "NONE"
  }
}

resource "partiteai_mcp_connection" "tunnel" {
  name             = "acc-tunnel"
  tunnel_transport = {}
}

# Subscriptions are their own resource, since a key has to reference one and they
# do not rank against each other.
resource "partiteai_mcp_connection_subscription" "dev" {
  mcp_connection_id = partiteai_mcp_connection.tunnel.id
  name              = "dev"
  criteria = [
    {
      name  = "env"
      value = "dev"
    },
  ]
}

# No criteria, so it accepts every conversation on the connection.
resource "partiteai_mcp_connection_subscription" "catch_all" {
  mcp_connection_id = partiteai_mcp_connection.tunnel.id
  name              = "catch-all"
}

resource "partiteai_mcp_connection_tunnel_key" "bob" {
  mcp_connection_id              = partiteai_mcp_connection.tunnel.id
  mcp_connection_subscription_id = partiteai_mcp_connection_subscription.dev.id
  label                          = "laptop-bob"
}
`
