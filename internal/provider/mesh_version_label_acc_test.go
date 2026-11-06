package provider

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// labelMesh is a mesh with one input. A change to the description of the
// input's response changes the draft.
func labelMesh(description string) string {
	return labelMeshOn(description, "partiteai_agent.base")
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

func captureID(address string, into *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[address]
		if !ok {
			return fmt.Errorf("%s not in state", address)
		}
		*into = rs.Primary.Attributes["id"]
		return nil
	}
}

const labelA = "partiteai_mesh_version_label.a"
const labelB = "partiteai_mesh_version_label.b"

func TestAccMeshVersionLabel_freezesTheDraft(t *testing.T) {
	config := labelMesh("x") + labelBlock("a", "a-baseline")
	accTest(t,
		resource.TestStep{
			Config: config,
			Check:  resource.TestCheckResourceAttr(labelA, "ordinal", "1"),
		},
		resource.TestStep{
			Config:            config,
			ResourceName:      labelA,
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

func TestAccMeshVersionLabel_twoLabelsInOneApply(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: labelMesh("x") + labelBlock("a", "a-one") + labelBlock("b", "b-two"),
		Check: func(s *terraform.State) error {
			a := s.RootModule().Resources[labelA].Primary.Attributes
			b := s.RootModule().Resources[labelB].Primary.Attributes
			if a["id"] == b["id"] {
				return fmt.Errorf("%s and %s both have version %s", labelA, labelB, a["id"])
			}
			ordinals := []string{a["ordinal"], b["ordinal"]}
			slices.Sort(ordinals)
			if !slices.Equal(ordinals, []string{"1", "2"}) {
				return fmt.Errorf("ordinals are %q, want 1 and 2", ordinals)
			}
			return nil
		},
	})
}

// The last step uses the label again. It succeeds only if the destroy removed
// the label from the first version.
func TestAccMeshVersionLabel_destroyRemovesTheLabel(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: labelMesh("x") + labelBlock("a", "a-baseline"),
		},
		resource.TestStep{
			Config: labelMesh("x"),
		},
		resource.TestStep{
			Config: labelMesh("x") + labelBlock("a", "a-baseline"),
			Check:  resource.TestCheckResourceAttr(labelA, "ordinal", "2"),
		},
	)
}

// The workflow in the resource documentation: label, change the draft, then
// replace the label resource with one at a new address.
func TestAccMeshVersionLabel_swapMovesTheBaseline(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: labelMesh("first") + labelBlock("a", "a-baseline"),
		},
		resource.TestStep{
			Config: labelMesh("second") + labelBlock("a", "a-baseline"),
		},
		resource.TestStep{
			Config: labelMesh("second") + labelBlock("b", "b-openfda-retry"),
			Check:  resource.TestCheckResourceAttr(labelB, "ordinal", "2"),
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
			Config:      labelMesh("x") + labelBlock("a", ""),
			ExpectError: regexp.MustCompile(`(?i)length.*at least 1`),
		},
	)
}

func TestAccMeshVersionLabel_renameInPlace(t *testing.T) {
	var id string
	accTest(t,
		resource.TestStep{
			Config: labelMesh("x") + labelBlock("a", "old"),
			Check:  captureID(labelA, &id),
		},
		resource.TestStep{
			Config: labelMesh("x") + labelBlock("a", "new"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPtr(labelA, "id", &id),
				resource.TestCheckResourceAttr(labelA, "ordinal", "1"),
			),
		},
	)
}

func TestAccMeshVersionLabelDataSource_findsByLabel(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: labelMesh("x") + labelBlock("a", "a-baseline") + `
data "partiteai_mesh_version_label" "found" {
  mesh_id = partiteai_mesh_version_label.a.mesh_id
  label   = partiteai_mesh_version_label.a.label
}
`,
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttrPair("data.partiteai_mesh_version_label.found", "id", labelA, "id"),
			resource.TestCheckResourceAttrPair("data.partiteai_mesh_version_label.found", "ordinal", labelA, "ordinal"),
		),
	})
}

func TestAccMeshVersionLabelDataSource_unknownLabelFails(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: labelMesh("x") + `
data "partiteai_mesh_version_label" "missing" {
  mesh_id = partiteai_mesh.base.id
  label   = "missing"
}
`,
		ExpectError: regexp.MustCompile(`has no version with the label "missing"`),
	})
}
