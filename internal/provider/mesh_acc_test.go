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
  probe = false
  name = "base-provider"
  openai = { api_key_wo = "sk-test" }
}

resource "partiteai_model_profile" "base" {
  probe = false
  name              = "base-profile"
  model_provider_id = partiteai_model_provider.base.id
  configuration = {
    model      = "gpt-5"
    max_tokens = 1024
  }
}

resource "partiteai_model_profile" "second" {
  probe = false
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

// thinkingProfileConfig moves the profile to adaptive thinking that shows
// only progress updates.
const thinkingProfileConfig = `
resource "partiteai_model_provider" "test" {
  probe = false
  name = "profile-provider"
  anthropic = { api_key_wo = "sk-ant" }
}

resource "partiteai_model_profile" "test" {
  probe = false
  name              = "acc-profile"
  model_provider_id = partiteai_model_provider.test.id
  configuration = {
    model            = "claude-opus-5-5"
    max_tokens       = 4096
    reasoning_effort = "HIGH"
    thinking_type    = "ADAPTIVE"
    thinking_display = "UPDATES"
  }
}
`

// probe = false writes the profile without asking the provider, and being
// write-only it is never stored.
func TestAccModelProfile_probeOff(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: `
resource "partiteai_model_provider" "test" {
  probe = false
  name = "probe-provider"
  anthropic = { api_key_wo = "sk-ant" }
}

resource "partiteai_model_profile" "test" {
  name              = "acc-probe-off"
  model_provider_id = partiteai_model_provider.test.id
  probe             = false
  configuration = {
    model      = "a-model-the-provider-does-not-serve-yet"
    max_tokens = 1024
  }
}
`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("partiteai_model_profile.test", "configuration.model", "a-model-the-provider-does-not-serve-yet"),
			resource.TestCheckNoResourceAttr("partiteai_model_profile.test", "probe"),
		),
	})
}

func TestAccModelProfile_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_model_provider" "test" {
  probe = false
  name = "profile-provider"
  anthropic = { api_key_wo = "sk-ant" }
}

resource "partiteai_model_profile" "test" {
  probe = false
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
			Config: thinkingProfileConfig,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_model_profile.test", "configuration.thinking_type", "ADAPTIVE"),
				resource.TestCheckResourceAttr("partiteai_model_profile.test", "configuration.thinking_display", "UPDATES"),
				resource.TestCheckResourceAttr("partiteai_model_profile.test", "configuration.reasoning_effort", "HIGH"),
				resource.TestCheckNoResourceAttr("partiteai_model_profile.test", "configuration.thinking_budget_tokens"),
			),
		},
		resource.TestStep{
			Config:            thinkingProfileConfig,
			ResourceName:      "partiteai_model_profile.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}
