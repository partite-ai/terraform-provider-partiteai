package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMesh_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: baseChain + `
resource "partiteai_mesh" "test" {
  name         = "acc-mesh"
  model_set_id = partiteai_model_set.base.id
}

data "partiteai_mesh" "test" {
  id = partiteai_mesh.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_mesh.test", "name", "acc-mesh"),
				resource.TestCheckResourceAttrSet("partiteai_mesh.test", "draft_version_id"),
				resource.TestCheckResourceAttrPair(
					"partiteai_mesh.test", "model_set_id",
					"partiteai_model_set.base", "id"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_mesh.test", "name",
					"partiteai_mesh.test", "name"),
			),
		},
		resource.TestStep{
			Config: baseChain + `
resource "partiteai_mesh" "test" {
  name         = "acc-mesh-renamed"
  description  = "a described mesh"
  model_set_id = partiteai_model_set.base.id
}
`,
			Check: resource.TestCheckResourceAttr("partiteai_mesh.test", "name", "acc-mesh-renamed"),
		},
		resource.TestStep{
			Config: baseChain + `
resource "partiteai_mesh" "test" {
  name         = "acc-mesh-renamed"
  description  = "a described mesh"
  model_set_id = partiteai_model_set.base.id
}
`,
			ResourceName:      "partiteai_mesh.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

func TestAccModelSet_rules(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: baseChain + `
data "partiteai_model_set" "base" {
  id = partiteai_model_set.base.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_model_set.base", "rules.#", "1"),
				resource.TestCheckResourceAttr("partiteai_model_set.base", "rules.0.complexity", "LOW"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_model_set.base", "name",
					"partiteai_model_set.base", "name"),
			),
		},
		// Adding a rule must not disturb the existing one.
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "base" {
  name = "base-provider"
  openai = { api_key = "sk-test" }
}

resource "partiteai_model_profile" "base" {
  name              = "base-profile"
  model_provider_id = partiteai_model_provider.base.id
  configuration = {
    model      = "gpt-5"
    max_tokens = 1024
  }
}

resource "partiteai_model_profile" "second" {
  name              = "second-profile"
  model_provider_id = partiteai_model_provider.base.id
  configuration = {
    model      = "gpt-5-mini"
    max_tokens = 512
  }
}

resource "partiteai_model_set" "base" {
  name = "base-set"

  default_model_profile = {
    id = partiteai_model_profile.base.id
  }

  rules = [
    {
      model_profile = { id = partiteai_model_profile.base.id }
      complexity    = "LOW"
    },
    {
      model_profile = { id = partiteai_model_profile.second.id }
      complexity    = "HIGH"
    },
  ]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_model_set.base", "rules.#", "2"),
				resource.TestCheckResourceAttr("partiteai_model_set.base", "rules.1.complexity", "HIGH"),
			),
		},
	)
}

func TestAccModelProfile_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "test" {
  name = "profile-provider"
  anthropic = { api_key = "sk-ant" }
}

resource "partiteai_model_profile" "test" {
  name              = "acc-profile"
  model_provider_id = partiteai_model_provider.test.id
  configuration = {
    model                  = "claude-opus-5"
    max_tokens             = 4096
    temperature            = 0.5
    thinking_budget_tokens = 1024
  }
}

data "partiteai_model_profile" "test" {
  id = partiteai_model_profile.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_model_profile.test", "configuration.model", "claude-opus-5"),
				resource.TestCheckResourceAttr("partiteai_model_profile.test", "configuration.max_tokens", "4096"),
				resource.TestCheckResourceAttr("partiteai_model_profile.test", "configuration.temperature", "0.5"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_model_profile.test", "configuration.model",
					"partiteai_model_profile.test", "configuration.model"),
			),
		},
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "test" {
  name = "profile-provider"
  anthropic = { api_key = "sk-ant" }
}

resource "partiteai_model_profile" "test" {
  name              = "acc-profile"
  model_provider_id = partiteai_model_provider.test.id
  configuration = {
    model                  = "claude-opus-5"
    max_tokens             = 4096
    temperature            = 0.5
    thinking_budget_tokens = 1024
  }
}
`,
			ResourceName:      "partiteai_model_profile.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}
