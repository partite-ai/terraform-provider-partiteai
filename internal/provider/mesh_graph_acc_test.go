package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// meshInputImportID builds the composite id the mesh_input resource imports by.
func meshInputImportID(name string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("%s not in state", name)
		}
		return rs.Primary.Attributes["mesh_id"] + ":" + rs.Primary.Attributes["name"], nil
	}
}

// meshLinkImportID builds the composite id the mesh_link resource imports by.
func meshLinkImportID(name string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("%s not in state", name)
		}
		return rs.Primary.Attributes["mesh_id"] + ":" + rs.Primary.Attributes["id"], nil
	}
}

const meshInputConfig = baseMesh + baseAgent + `
resource "partiteai_mesh_input" "test" {
  mesh_id  = partiteai_mesh.base.id
  agent_id = partiteai_agent.base.id
  name     = "acc_input"

  responses = [
    {
      name        = "success"
      description = "It worked."
      message_type = jsonencode({
        type = "object"
        properties = {
          answer = { type = "string", description = "The answer." }
        }
        required = ["answer"]
      })
    },
  ]
}
`

func TestAccMeshInput_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: meshInputConfig,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_mesh_input.test", "name", "acc_input"),
				resource.TestCheckResourceAttr("partiteai_mesh_input.test", "responses.#", "1"),
			),
		},
		resource.TestStep{
			Config:            meshInputConfig,
			ResourceName:      "partiteai_mesh_input.test",
			ImportState:       true,
			ImportStateIdFunc: meshInputImportID("partiteai_mesh_input.test"),
			ImportStateVerify: true,
			// mesh_input has no id of its own, so tell the framework which
			// attributes identify the imported object.
			ImportStateVerifyIdentifierAttribute: "name",
		},
	)
}

// A link's source agent has to already be in the mesh, so the fixture reaches
// the mesh through an input on the source agent first. meshLinkBase is
// everything around the link, so a step can drop the link on its own.
const meshLinkBase = meshInputConfig + `
resource "partiteai_agent" "to" {
  name               = "link-to"
  model_complexity   = "LOW"
  task_instructions  = "Answer."
  input_message_type = jsonencode({ type = "object", properties = {} })
}
`

func meshLinkConfigWith(name, description string, transfersControl bool) string {
	return meshLinkBase + fmt.Sprintf(`
resource "partiteai_mesh_link" "test" {
  # The source agent reaches the mesh through the input, on the way in and on
  # the way out. Terraform cannot infer that from the attributes alone.
  depends_on = [partiteai_mesh_input.test]

  mesh_id             = partiteai_mesh.base.id
  from_agent_id       = partiteai_agent.base.id
  to_agent_version_id = partiteai_agent.to.version_id

  name              = %q
  description       = %q
  transfers_control = %t

  responses = [
    {
      name        = "done"
      description = "Finished."
      message_type = jsonencode({
        type = "object"
        properties = {
          answer = { type = "string", description = "The answer." }
        }
        required = ["answer"]
      })
    },
  ]
}
`, name, description, transfersControl)
}

var meshLinkConfig = meshLinkConfigWith("acc_link", "From one agent to the next.", false)

func TestAccMeshLink_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: meshLinkConfig,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_mesh_link.test", "name", "acc_link"),
				resource.TestCheckResourceAttr("partiteai_mesh_link.test", "transfers_control", "false"),
				resource.TestCheckResourceAttr("partiteai_mesh_link.test", "responses.#", "1"),
				resource.TestCheckResourceAttrSet("partiteai_mesh_link.test", "id"),
			),
		},
		// A link is updated by its stable id, which is a different query from
		// the one that reads it.
		resource.TestStep{
			Config: meshLinkConfigWith("acc_link_renamed", "Now it transfers control.", true),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_mesh_link.test", "name", "acc_link_renamed"),
				resource.TestCheckResourceAttr("partiteai_mesh_link.test", "description", "Now it transfers control."),
				resource.TestCheckResourceAttr("partiteai_mesh_link.test", "transfers_control", "true"),
			),
		},
		resource.TestStep{
			Config:            meshLinkConfigWith("acc_link_renamed", "Now it transfers control.", true),
			ResourceName:      "partiteai_mesh_link.test",
			ImportState:       true,
			ImportStateIdFunc: meshLinkImportID("partiteai_mesh_link.test"),
			ImportStateVerify: true,
		},
		// Dropping the link while its mesh stays is the delete path on its own.
		// It is worth a step of its own because a delete that fails only at
		// teardown reports as "dangling resources" against the whole test
		// rather than against the thing that broke.
		resource.TestStep{
			Config: meshLinkBase,
			Check:  resourceAbsent("partiteai_mesh_link.test"),
		},
	)
}

const webhookConfig = meshInputConfig + `
resource "partiteai_webhook_destination" "test" {
  name       = "acc-webhook"
  mesh_id    = partiteai_mesh.base.id
  input_name = partiteai_mesh_input.test.name
}

data "partiteai_webhook_destination" "test" {
  id = partiteai_webhook_destination.test.id
}
`

func TestAccWebhookDestination_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: webhookConfig,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_webhook_destination.test", "name", "acc-webhook"),
				resource.TestCheckResourceAttrSet("partiteai_webhook_destination.test", "external_id"),
				resource.TestCheckResourceAttrSet("partiteai_webhook_destination.test", "invoke_url"),
				// The data source has to expose the same computed URL.
				resource.TestCheckResourceAttrPair(
					"data.partiteai_webhook_destination.test", "invoke_url",
					"partiteai_webhook_destination.test", "invoke_url"),
			),
		},
		resource.TestStep{
			Config:            webhookConfig,
			ResourceName:      "partiteai_webhook_destination.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

func TestAccApiKey_basic(t *testing.T) {
	config := func(name string) string {
		return fmt.Sprintf(`
resource "partiteai_api_key" "test" {
  name   = %q
  scopes = ["configuration:read", "configuration:write"]
}

data "partiteai_api_key" "test" {
  id = partiteai_api_key.test.id
}
`, name)
	}
	accTest(t,
		resource.TestStep{
			Config: config("acc-key"),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_api_key.test", "scopes.#", "2"),
				resource.TestCheckResourceAttrSet("partiteai_api_key.test", "kid"),
				resource.TestCheckResourceAttrSet("partiteai_api_key.test", "secret"),
				apiKeyTokenMatchesKidAndSecret("partiteai_api_key.test"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_api_key.test", "kid",
					"partiteai_api_key.test", "kid"),
			),
		},
		// The secret and token come back only on create, so an update keeps
		// what state has.
		resource.TestStep{
			Config: config("acc-key-renamed"),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_api_key.test", "name", "acc-key-renamed"),
				apiKeyTokenMatchesKidAndSecret("partiteai_api_key.test"),
			),
		},
	)
}

// apiKeyTokenMatchesKidAndSecret checks that the token in state is the one
// made from the key ID and secret.
func apiKeyTokenMatchesKidAndSecret(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s is not in state", name)
		}
		attrs := rs.Primary.Attributes
		want := "prt_ak_" + attrs["kid"] + "_" + attrs["secret"]
		if attrs["kid"] == "" || attrs["secret"] == "" || attrs["token"] != want {
			return fmt.Errorf("%s: token does not match prt_ak_<kid>_<secret>", name)
		}
		return nil
	}
}

// resourceAbsent asserts a resource is gone from state, which is what a
// successful delete leaves behind.
func resourceAbsent(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[name]; ok {
			return fmt.Errorf("%s is still in state", name)
		}
		return nil
	}
}
