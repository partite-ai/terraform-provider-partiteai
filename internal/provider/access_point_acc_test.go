package provider

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const accessPointHCL = `
resource "partiteai_boundary" "test" {
  name = "acc-access-point-boundary"
  rules = [{
    name   = "support-meshes-only"
    action = "CONVERSATION_CREATE"
    when   = "mesh.name != \"support\""
    effect = "DENY"
  }]
}

resource "partiteai_access_point" "test" {
  name                          = "acc-portal"
  client_token_lifetime_seconds = 600
  tags                          = [{ name = "tier", value = "gold" }]
  boundary_ids                  = [partiteai_boundary.test.id]

  issuers = [
    {
      issuer   = "https://issuer.example.com"
      jwks_url = "https://issuer.example.com/.well-known/jwks.json"
    },
    {
      issuer         = "backend"
      check_audience = false
      jwks = jsonencode({ keys = [{
        kty = "EC", crv = "P-256", kid = "acc", alg = "ES256", use = "sig"
        x   = "iYpMZmeNXrKzmlnxKff08h5HPPinPRtMN9nGOwja1Ck"
        y   = "D0VIgYn7KKPf5RNxOfCPxk_MqIHsiDOrq7VK0DsdxSc"
      }] })
    },
  ]
}
`

const accessPointUpdatedHCL = `
resource "partiteai_access_point" "test" {
  name        = "acc-portal-renamed"
  description = "for the support portal"

  issuers = [{
    issuer   = "https://issuer.example.com"
    audience = "support-portal"
    jwks_url = "https://issuer.example.com/.well-known/jwks.json"
  }]
}
`

func TestAccAccessPoint_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: accessPointHCL + `
data "partiteai_access_point" "test" {
  id = partiteai_access_point.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_access_point.test", "name", "acc-portal"),
				resource.TestCheckResourceAttrSet("partiteai_access_point.test", "url"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "client_token_lifetime_seconds", "600"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "tags.0.value", "gold"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "issuers.#", "2"),
				// An issuer that leaves audience out gets the access point's URL.
				resource.TestCheckResourceAttrPair(
					"partiteai_access_point.test", "issuers.0.audience",
					"partiteai_access_point.test", "url"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "issuers.0.check_audience", "true"),
				resource.TestCheckNoResourceAttr("partiteai_access_point.test", "issuers.1.audience"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "issuers.1.check_audience", "false"),
				resource.TestCheckResourceAttrSet("partiteai_access_point.test", "issuers.1.jwks"),
				resource.TestCheckTypeSetElemAttrPair(
					"partiteai_access_point.test", "boundary_ids.*",
					"partiteai_boundary.test", "id"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_access_point.test", "url",
					"partiteai_access_point.test", "url"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_access_point.test", "issuers.1.check_audience",
					"partiteai_access_point.test", "issuers.1.check_audience"),
			),
		},
		// Fields left out go back to their defaults, and an issuer can name its
		// own audience.
		resource.TestStep{
			Config: accessPointUpdatedHCL,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_access_point.test", "name", "acc-portal-renamed"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "client_token_lifetime_seconds", "3600"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "tags.#", "0"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "boundary_ids.#", "0"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "issuers.#", "1"),
				resource.TestCheckResourceAttr("partiteai_access_point.test", "issuers.0.audience", "support-portal"),
			),
		},
		resource.TestStep{
			Config:            accessPointUpdatedHCL,
			ResourceName:      "partiteai_access_point.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

const accessPointClientHCL = `
resource "partiteai_access_point" "test" {
  name = "acc-client-portal"
}

resource "partiteai_access_point_client" "test" {
  access_point_id = partiteai_access_point.test.id
  name            = "backend"
  claims          = jsonencode({ team = "ops" })
}
`

func TestAccAccessPointClient_secrets(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: accessPointClientHCL + `
resource "partiteai_access_point_client_secret" "first" {
  access_point_client_id = partiteai_access_point_client.test.id
}

data "partiteai_access_point_client" "test" {
  id = partiteai_access_point_client.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("partiteai_access_point_client.test", "client_id"),
				resource.TestCheckResourceAttr("partiteai_access_point_client.test", "claims", `{"team":"ops"}`),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_access_point_client.test", "client_id",
					"partiteai_access_point_client.test", "client_id"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_access_point_client.test", "access_point_id",
					"partiteai_access_point.test", "id"),
				resource.TestCheckNoResourceAttr("partiteai_access_point_client_secret.first", "expires_at"),
				checkClientSignsIn("partiteai_access_point_client_secret.first"),
			),
		},
		// A second secret replaces the first without changing the client.
		resource.TestStep{
			Config: accessPointClientHCL + `
resource "partiteai_access_point_client_secret" "second" {
  access_point_client_id = partiteai_access_point_client.test.id
  expires_at             = "2099-01-01T00:00:00Z"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_access_point_client_secret.second", "expires_at", "2099-01-01T00:00:00Z"),
				checkClientSignsIn("partiteai_access_point_client_secret.second"),
			),
		},
		resource.TestStep{
			Config: `
resource "partiteai_access_point" "test" {
  name = "acc-client-portal"
}

resource "partiteai_access_point_client" "test" {
  access_point_id = partiteai_access_point.test.id
  name            = "backend-renamed"
}

resource "partiteai_access_point_client_secret" "second" {
  access_point_client_id = partiteai_access_point_client.test.id
  expires_at             = "2099-01-01T00:00:00Z"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_access_point_client.test", "name", "backend-renamed"),
				resource.TestCheckNoResourceAttr("partiteai_access_point_client.test", "claims"),
			),
		},
		resource.TestStep{
			Config: `
resource "partiteai_access_point" "test" {
  name = "acc-client-portal"
}

resource "partiteai_access_point_client" "test" {
  access_point_id = partiteai_access_point.test.id
  name            = "backend-renamed"
}

resource "partiteai_access_point_client_secret" "second" {
  access_point_client_id = partiteai_access_point_client.test.id
  expires_at             = "2099-01-01T00:00:00Z"
}
`,
			ResourceName:      "partiteai_access_point_client.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// checkClientSignsIn gets a token from the access point with the client ID
// and the secret in state.
func checkClientSignsIn(secretResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attribute := func(name, key string) string {
			return s.RootModule().Resources[name].Primary.Attributes[key]
		}
		form := url.Values{
			"grant_type":    {"client_credentials"},
			"client_id":     {attribute("partiteai_access_point_client.test", "client_id")},
			"client_secret": {attribute(secretResource, "secret")},
		}
		accessPointURL := attribute("partiteai_access_point.test", "url")
		// The URL comes from the backend's public base URL. A backend that
		// leaves it at its default would send the secret to the real API.
		if parsed, err := url.Parse(accessPointURL); err != nil || parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
			return fmt.Errorf("access point URL %q is not the test backend's", accessPointURL)
		}
		resp, err := http.PostForm(accessPointURL+"/oauth/token", form)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "access_token") {
			return fmt.Errorf("signing in with %s: status %d: %s", secretResource, resp.StatusCode, body)
		}
		return nil
	}
}
