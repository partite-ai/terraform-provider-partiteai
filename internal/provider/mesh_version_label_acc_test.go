package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// labelMesh is a mesh with one input.
func labelMesh() string {
	return labelMeshOn("x", "partiteai_agent.base")
}

// labelMeshOn is labelMesh with a mesh that depends on the given agents. A
// frozen version keeps references to the agent versions it uses, so a destroy
// must delete the mesh, and with it its frozen versions, before the agents.
func labelMeshOn(description string, agents ...string) string {
	return baseChain + baseAgent + fmt.Sprintf(`
resource "partiteai_mesh" "base" {
  name         = "base-mesh"
  model_set_id = partiteai_model_set.base.id
  depends_on   = [%s]
}

resource "partiteai_mesh_input" "test" {
  mesh_id  = partiteai_mesh.base.id
  agent_id = partiteai_agent.base.id
  name     = "acc_input"

  responses = [
    {
      name        = "success"
      description = %q
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
`, strings.Join(agents, ", "), description)
}

func labelBlock(name, label string) string {
	return fmt.Sprintf(`
resource "partiteai_mesh_version_label" %q {
  mesh_id    = partiteai_mesh.base.id
  label      = %q
  depends_on = [partiteai_mesh_input.test]
}
`, name, label)
}

const labelA = "partiteai_mesh_version_label.a"

const labelDataSource = `
data "partiteai_mesh_version_label" "found" {
  mesh_id = partiteai_mesh_version_label.a.mesh_id
  label   = partiteai_mesh_version_label.a.label
}
`

func TestAccMeshVersionLabel_basic(t *testing.T) {
	renamed := labelMesh() + labelBlock("a", "new") + labelDataSource
	accTest(t,
		resource.TestStep{
			Config: labelMesh() + labelBlock("a", "old") + labelDataSource,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(labelA, "ordinal", "1"),
				resource.TestCheckResourceAttrPair("data.partiteai_mesh_version_label.found", "id", labelA, "id"),
				resource.TestCheckResourceAttr("data.partiteai_mesh_version_label.found", "ordinal", "1"),
			),
		},
		resource.TestStep{
			Config: renamed,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(labelA, plancheck.ResourceActionUpdate),
				},
			},
			Check: resource.TestCheckResourceAttrPair("data.partiteai_mesh_version_label.found", "id", labelA, "id"),
		},
		resource.TestStep{
			Config:            renamed,
			ResourceName:      labelA,
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// The last step uses the label again. It succeeds only if the destroy removed
// the label from the first version.
func TestAccMeshVersionLabel_destroyRemovesTheLabel(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: labelMesh() + labelBlock("a", "a-baseline"),
		},
		resource.TestStep{
			Config: labelMesh(),
		},
		resource.TestStep{
			Config: labelMesh() + labelBlock("a", "a-baseline"),
		},
	)
}

// After a label, the input and the link read a new draft. The empty-plan
// check that runAcc adds after apply and refresh is the main assertion.
func TestAccMeshVersionLabel_inputsAndLinksPlanCleanAfterALabel(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: labelMeshOn("It worked.", "partiteai_agent.base", "partiteai_agent.to") +
				meshLinkToAgent +
				meshLinkBlock("acc_link", "From one agent to the next.", false) + `
resource "partiteai_mesh_version_label" "test" {
  mesh_id    = partiteai_mesh.base.id
  label      = "graph"
  depends_on = [partiteai_mesh_input.test, partiteai_mesh_link.test]
}
`,
		},
	)
}

func TestAccMeshVersionLabel_emptyLabelFails(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config:      labelMesh() + labelBlock("a", ""),
			ExpectError: regexp.MustCompile(`(?is)length.*at least 1`),
		},
	)
}

func TestAccMeshVersionLabelDataSource_unknownLabelFails(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: labelMesh() + `
data "partiteai_mesh_version_label" "missing" {
  mesh_id = partiteai_mesh.base.id
  label   = "missing"
}
`,
		ExpectError: regexp.MustCompile(`(?s)has no version with the label\s+"missing"`),
	})
}
