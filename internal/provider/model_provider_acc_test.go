package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccModelProvider_openai(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "test" {
  probe = false
  name = "acc-openai"
  openai = {
    api_key_wo = "sk-test-1"
  }
}

data "partiteai_model_provider" "test" {
  id = partiteai_model_provider.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_model_provider.test", "name", "acc-openai"),
				resource.TestCheckResourceAttrSet("partiteai_model_provider.test", "id"),
				resource.TestCheckResourceAttrSet("partiteai_model_provider.test", "created_at"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_model_provider.test", "name",
					"partiteai_model_provider.test", "name"),
				// The key is write-only, so state never holds it, and the data
				// source only says that one is set.
				resource.TestCheckNoResourceAttr("partiteai_model_provider.test", "openai.api_key_wo"),
				resource.TestCheckResourceAttr("data.partiteai_model_provider.test", "openai.api_key_set", "true"),
				resource.TestCheckNoResourceAttr("partiteai_model_provider.test", "probe"),
			),
		},
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "test" {
  probe = false
  name        = "acc-openai-renamed"
  description = "now with a description"
  openai = {
    api_key_wo = "sk-test-1"
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_model_provider.test", "name", "acc-openai-renamed"),
				resource.TestCheckResourceAttr("partiteai_model_provider.test", "description", "now with a description"),
			),
		},
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "test" {
  probe = false
  name        = "acc-openai-renamed"
  description = "now with a description"
  openai = {
    api_key_wo = "sk-test-1"
  }
}
`,
			ResourceName:      "partiteai_model_provider.test",
			ImportState:       true,
			ImportStateVerify: true,
			// Nothing to ignore: the key is write-only, so neither side holds
			// it, and an import starts at credentials_version 1.
		},
		resource.TestStep{
			// A new key is sent by changing credentials_version with it.
			Config: `
resource "partiteai_model_provider" "test" {
  probe = false
  name                = "acc-openai-renamed"
  description         = "now with a description"
  credentials_version = 2
  openai = {
    api_key_wo = "sk-test-2"
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_model_provider.test", "credentials_version", "2"),
				resource.TestCheckNoResourceAttr("partiteai_model_provider.test", "openai.api_key_wo"),
			),
		},
	)
}

// TestAccModelProvider_variants covers every provider shape in one apply. Each
// is a separate branch in the create and read mapping, and a mistake in any of
// them shows up as a non-empty plan after apply.
func TestAccModelProvider_variants(t *testing.T) {
	config := fmt.Sprintf(`
resource "partiteai_model_provider" "anthropic" {
  probe = false
  name = "acc-anthropic"
  anthropic = {
    api_key_wo = "sk-ant-test"
  }
}

resource "partiteai_model_provider" "compatible_with_token" {
  probe = false
  name = "acc-compatible-token"
  openai_compatible = {
    url       = "https://example.invalid/v1"
    api_token_wo = "tok"
  }
}

resource "partiteai_model_provider" "compatible_without_token" {
  probe = false
  name = "acc-compatible-no-token"
  openai_compatible = {
    url = "https://example.invalid/v1"
  }
}

resource "partiteai_model_provider" "gemini" {
  probe = false
  name = "acc-gemini"
  google_genai = {
    gemini = {
      api_key_wo = "goog-test"
    }
  }
}

resource "partiteai_model_provider" "fireworks" {
  probe = false
  name = "acc-fireworks"
  fireworks = {
    api_key_wo = "fw-test"
    account_id = "acct"
  }
}

resource "partiteai_model_provider" "cerebras" {
  probe = false
  name = "acc-cerebras"
  cerebras = {
    api_key_wo = "csk-test"
  }
}

resource "partiteai_model_provider" "vertex" {
  probe = false
  name = "acc-vertex"
  google_genai = {
    vertex = {
      project_id          = "proj"
      location            = "us-central1"
      service_account_key_wo = %q
    }
  }
}

resource "partiteai_model_provider" "bedrock_api_key" {
  probe = false
  name  = "acc-bedrock-api-key"
  bedrock = {
    region     = "us-east-2"
    api_key_wo = "bedrock-api-key-test"
  }
}

resource "partiteai_model_provider" "bedrock_static_keys" {
  probe = false
  name  = "acc-bedrock-static-keys"
  bedrock = {
    region = "us-east-2"
    static_keys = {
      access_key_id        = "AKIATESTTESTTESTTEST"
      secret_access_key_wo = "test-secret"
    }
  }
}
`, serviceAccountKey(t))
	accTest(t, resource.TestStep{
		Config: config,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("partiteai_model_provider.anthropic", "id"),
			resource.TestCheckResourceAttr("partiteai_model_provider.compatible_with_token", "openai_compatible.url", "https://example.invalid/v1"),
			resource.TestCheckResourceAttrSet("partiteai_model_provider.compatible_without_token", "id"),
			resource.TestCheckNoResourceAttr("partiteai_model_provider.gemini", "google_genai.gemini.api_key_wo"),
			resource.TestCheckResourceAttr("partiteai_model_provider.vertex", "google_genai.vertex.location", "us-central1"),
			resource.TestCheckResourceAttr("partiteai_model_provider.fireworks", "fireworks.account_id", "acct"),
			resource.TestCheckNoResourceAttr("partiteai_model_provider.fireworks", "fireworks.api_key_wo"),
			resource.TestCheckResourceAttr("partiteai_model_provider.bedrock_api_key", "bedrock.region", "us-east-2"),
			resource.TestCheckResourceAttr("partiteai_model_provider.bedrock_static_keys", "bedrock.region", "us-east-2"),
			resource.TestCheckResourceAttr("partiteai_model_provider.bedrock_static_keys", "bedrock.static_keys.access_key_id", "AKIATESTTESTTESTTEST"),
		),
	},
		resource.TestStep{
			Config:            config,
			ResourceName:      "partiteai_model_provider.bedrock_api_key",
			ImportState:       true,
			ImportStateVerify: true,
		},
		resource.TestStep{
			Config:            config,
			ResourceName:      "partiteai_model_provider.bedrock_static_keys",
			ImportState:       true,
			ImportStateVerify: true,
		},
		resource.TestStep{
			Config: config + `
data "partiteai_model_provider" "bedrock_static_keys" {
  id = partiteai_model_provider.bedrock_static_keys.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.partiteai_model_provider.bedrock_static_keys", "bedrock.api_key_set", "false"),
				resource.TestCheckResourceAttr("data.partiteai_model_provider.bedrock_static_keys", "bedrock.static_keys.secret_access_key_set", "true"),
			),
		},
	)
}

// TestAccModelProvider_bedrockCredentials pins the plan-time check that a
// bedrock block holds exactly one credential.
func TestAccModelProvider_bedrockCredentials(t *testing.T) {
	accTest(t,
		resource.TestStep{
			PlanOnly: true,
			Config: `
resource "partiteai_model_provider" "both" {
  probe = false
  name  = "acc-bedrock-both"
  bedrock = {
    region     = "us-east-2"
    api_key_wo = "bedrock-api-key-test"
    static_keys = {
      access_key_id        = "AKIATESTTESTTESTTEST"
      secret_access_key_wo = "test-secret"
    }
  }
}
`,
			ExpectError: regexp.MustCompile(`(?s)2\s+attributes\s+specified\s+when\s+one\s+\(and\s+only\s+one\)\s+of\s+\[bedrock\.static_keys,bedrock\.api_key_wo\]`),
		},
		resource.TestStep{
			PlanOnly: true,
			Config: `
resource "partiteai_model_provider" "neither" {
  probe = false
  name  = "acc-bedrock-neither"
  bedrock = {
    region = "us-east-2"
  }
}
`,
			ExpectError: regexp.MustCompile(`(?s)No\s+attribute\s+specified\s+when\s+one\s+\(and\s+only\s+one\)\s+of\s+\[bedrock\.static_keys,bedrock\.api_key_wo\]`),
		},
	)
}

// TestAccModelProvider_tags pins how tags round-trip. Tags default to an empty
// list, so adding and then removing them must land back on empty rather than
// null, which would show as a permanent diff.
func TestAccModelProvider_tags(t *testing.T) {
	const withTags = `
resource "partiteai_model_provider" "test" {
  probe = false
  name = "acc-tags"
  openai = {
    api_key_wo = "sk-test"
  }
  tags = [
    { name = "environment", value = "test" },
    { name = "team", value = "devops" },
  ]
}
`
	const withoutTags = `
resource "partiteai_model_provider" "test" {
  probe = false
  name = "acc-tags"
  openai = {
    api_key_wo = "sk-test"
  }
}
`

	accTest(t,
		resource.TestStep{
			Config: withTags,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_model_provider.test", "tags.#", "2"),
				resource.TestCheckResourceAttr("partiteai_model_provider.test", "tags.0.name", "environment"),
				resource.TestCheckResourceAttr("partiteai_model_provider.test", "tags.1.value", "devops"),
			),
		},
		resource.TestStep{
			Config: withoutTags,
			Check:  resource.TestCheckResourceAttr("partiteai_model_provider.test", "tags.#", "0"),
		},
	)
}

// serviceAccountKey is a well-formed service account document with a key made
// for the test. The server parses the key when it validates a vertex config,
// so the key must be a real RSA key.
func serviceAccountKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "proj",
		"private_key_id": "k1",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
		"client_email":   "sa@proj.iam.gserviceaccount.com",
		"client_id":      "1",
		"token_uri":      "https://oauth2.googleapis.com/token",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(doc)
}
