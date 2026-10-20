package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const boundaryHCL = `
resource "partiteai_boundary" "test" {
  name        = "acc-boundary"
  description = "keeps internal tags out"

  rules = [{
    name    = "no-internal-tags"
    action  = "CONVERSATION_CREATE"
    when    = "\"internal\" in conversation.tags"
    effect  = "DENY"
    message = "Tags named internal are reserved."
  }]

  modifications = [{
    name     = "tag-support"
    action   = "CONVERSATION_CREATE"
    set_tags = { team = "\"support\"" }
  }]
}
`

const boundaryUpdatedHCL = baseChain + `
resource "partiteai_boundary" "test" {
  name        = "acc-boundary"
  report_only = true

  rules = [
    {
      name   = "no-internal-tags"
      action = "CONVERSATION_CREATE"
      when   = "\"internal\" in conversation.tags"
      effect = "DENY"
    },
    {
      name        = "no-github"
      action      = "TOOL_CALL"
      when        = "tool.source.kind == \"mcp\" && tool.source.name == \"github\""
      effect      = "ASK"
      report_only = true
    },
  ]
}

resource "partiteai_mesh" "test" {
  name         = "acc-bounded-mesh"
  model_set_id = partiteai_model_set.base.id
}
`

func TestAccBoundary_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: baseChain + boundaryHCL + `
resource "partiteai_mesh" "test" {
  name         = "acc-bounded-mesh"
  model_set_id = partiteai_model_set.base.id
  boundary_ids = [partiteai_boundary.test.id]
}

data "partiteai_boundary" "test" {
  id = partiteai_boundary.test.id
}

data "partiteai_mesh" "test" {
  id = partiteai_mesh.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_boundary.test", "name", "acc-boundary"),
				resource.TestCheckResourceAttr("partiteai_boundary.test", "report_only", "false"),
				resource.TestCheckResourceAttr("partiteai_boundary.test", "rules.#", "1"),
				resource.TestCheckResourceAttr("partiteai_boundary.test", "rules.0.report_only", "false"),
				resource.TestCheckResourceAttr("partiteai_boundary.test", "modifications.0.set_tags.team", `"support"`),
				resource.TestCheckResourceAttr("partiteai_boundary.test", "warnings.#", "0"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_boundary.test", "rules.0.message",
					"partiteai_boundary.test", "rules.0.message"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_boundary.test", "modifications.0.set_tags.team",
					"partiteai_boundary.test", "modifications.0.set_tags.team"),
				resource.TestCheckTypeSetElemAttrPair(
					"partiteai_mesh.test", "boundary_ids.*",
					"partiteai_boundary.test", "id"),
				resource.TestCheckTypeSetElemAttrPair(
					"data.partiteai_mesh.test", "boundary_ids.*",
					"partiteai_boundary.test", "id"),
			),
		},
		// Changing the rules replaces the whole list in place, and leaving
		// boundary_ids out detaches the boundary from the mesh.
		resource.TestStep{
			Config: boundaryUpdatedHCL,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_boundary.test", "report_only", "true"),
				resource.TestCheckNoResourceAttr("partiteai_boundary.test", "description"),
				resource.TestCheckResourceAttr("partiteai_boundary.test", "rules.#", "2"),
				resource.TestCheckResourceAttr("partiteai_boundary.test", "rules.1.effect", "ASK"),
				resource.TestCheckResourceAttr("partiteai_boundary.test", "modifications.#", "0"),
				resource.TestCheckResourceAttr("partiteai_mesh.test", "boundary_ids.#", "0"),
			),
		},
		resource.TestStep{
			Config:            boundaryUpdatedHCL,
			ResourceName:      "partiteai_boundary.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// A rule whose expression doesn't compile is refused when it is saved.
func TestAccBoundary_invalidExpression(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_boundary" "test" {
  name  = "acc-broken"
  rules = [{
    name   = "broken"
    action = "CONVERSATION_CREATE"
    when   = "conversation.tags ==="
    effect = "DENY"
  }]
}
`,
			ExpectError: regexp.MustCompile(`broken`),
		},
	)
}
