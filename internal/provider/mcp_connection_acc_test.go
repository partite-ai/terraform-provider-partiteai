package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMcpConnection_stdio(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: `
resource "partiteai_mcp_connection" "test" {
  name = "acc-stdio"
  stdio_transport = {
    command   = "echo"
    arguments = ["hello"]
    environment = [
      { name = "FOO", value = "bar" },
    ]
  }
}
`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("partiteai_mcp_connection.test", "name", "acc-stdio"),
			resource.TestCheckResourceAttr("partiteai_mcp_connection.test", "stdio_transport.command", "echo"),
		),
	})
}

// TestAccMcpConnection_stdioMinimal pins the case where the optional stdio
// fields are left out entirely.
func TestAccMcpConnection_stdioMinimal(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: `
resource "partiteai_mcp_connection" "test" {
  name = "acc-stdio-minimal"
  stdio_transport = {
    command = "echo"
  }
}
`,
		Check: resource.TestCheckResourceAttr("partiteai_mcp_connection.test", "stdio_transport.command", "echo"),
	})
}

// TestAccMcpConnection_httpCredentials covers the write-only credential blocks.
// OAUTH2_CLIENT_CREDENTIALS is left out on purpose: creating one fetches the
// authorization server's metadata document, so it cannot run without network
// access to the URL in the configuration.
// Write-only values never enter state, so the only thing that can be checked is
// that apply succeeds and the plan settles — which is exactly the regression
// that a mistake here produces.
func TestAccMcpConnection_httpCredentials(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: `
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

resource "partiteai_mcp_connection" "sih" {
  name                            = "acc-sih"
  server_initiated_http_transport = {}
}
`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("partiteai_mcp_connection.basic", "http_transport.authentication_method", "BASIC"),
			resource.TestCheckResourceAttr("partiteai_mcp_connection.header", "http_transport.authentication_method", "HEADER"),
			resource.TestCheckResourceAttrSet("partiteai_mcp_connection.none", "id"),
			resource.TestCheckResourceAttrSet("partiteai_mcp_connection.sih", "id"),
		),
	})
}
