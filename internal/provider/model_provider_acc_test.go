package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccModelProvider_openai(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "test" {
  name = "acc-openai"
  openai = {
    api_key = "sk-test-1"
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
			),
		},
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "test" {
  name        = "acc-openai-renamed"
  description = "now with a description"
  openai = {
    api_key = "sk-test-1"
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
  name        = "acc-openai-renamed"
  description = "now with a description"
  openai = {
    api_key = "sk-test-1"
  }
}
`,
			ResourceName:      "partiteai_model_provider.test",
			ImportState:       true,
			ImportStateVerify: true,
			// The API never returns the key it was given.
			ImportStateVerifyIgnore: []string{"openai"},
		},
	)
}

// TestAccModelProvider_variants covers every provider shape in one apply. Each
// is a separate branch in the create and read mapping, and a mistake in any of
// them shows up as a non-empty plan after apply.
func TestAccModelProvider_variants(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: `
resource "partiteai_model_provider" "anthropic" {
  name = "acc-anthropic"
  anthropic = {
    api_key = "sk-ant-test"
  }
}

resource "partiteai_model_provider" "compatible_with_token" {
  name = "acc-compatible-token"
  openai_compatible = {
    url       = "https://example.invalid/v1"
    api_token = "tok"
  }
}

resource "partiteai_model_provider" "compatible_without_token" {
  name = "acc-compatible-no-token"
  openai_compatible = {
    url = "https://example.invalid/v1"
  }
}

resource "partiteai_model_provider" "gemini" {
  name = "acc-gemini"
  google_genai = {
    gemini = {
      api_key = "goog-test"
    }
  }
}

resource "partiteai_model_provider" "vertex" {
  name = "acc-vertex"
  google_genai = {
    vertex = {
      project_id          = "proj"
      location            = "us-central1"
      service_account_key = "{}"
    }
  }
}
`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("partiteai_model_provider.anthropic", "id"),
			resource.TestCheckResourceAttr("partiteai_model_provider.compatible_with_token", "openai_compatible.url", "https://example.invalid/v1"),
			resource.TestCheckResourceAttrSet("partiteai_model_provider.compatible_without_token", "id"),
			resource.TestCheckResourceAttr("partiteai_model_provider.gemini", "google_genai.gemini.api_key", "goog-test"),
			resource.TestCheckResourceAttr("partiteai_model_provider.vertex", "google_genai.vertex.location", "us-central1"),
		),
	})
}

// TestAccModelProvider_tags pins how tags round-trip. Tags default to an empty
// list, so adding and then removing them must land back on empty rather than
// null, which would show as a permanent diff.
func TestAccModelProvider_tags(t *testing.T) {
	const withTags = `
resource "partiteai_model_provider" "test" {
  name = "acc-tags"
  openai = {
    api_key = "sk-test"
  }
  tags = [
    { name = "environment", value = "test" },
    { name = "team", value = "devops" },
  ]
}
`
	const withoutTags = `
resource "partiteai_model_provider" "test" {
  name = "acc-tags"
  openai = {
    api_key = "sk-test"
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
