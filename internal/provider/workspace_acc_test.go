package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const workspaceHCL = `
resource "partiteai_workspace" "test" {
  name  = "acc-workspace"
  scope = "GLOBAL"
}
`

func TestAccWorkspace_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: workspaceHCL + `
data "partiteai_workspace" "by_id" {
  id = partiteai_workspace.test.id
}

data "partiteai_workspace" "by_name" {
  name = partiteai_workspace.test.name
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_workspace.test", "name", "acc-workspace"),
				resource.TestCheckResourceAttr("partiteai_workspace.test", "scope", "GLOBAL"),
				resource.TestCheckResourceAttrSet("partiteai_workspace.test", "id"),
				resource.TestCheckResourceAttrSet("partiteai_workspace.test", "created_at"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_workspace.by_id", "scope",
					"partiteai_workspace.test", "scope"),
				// Looking one up by name has to land on the same workspace.
				resource.TestCheckResourceAttrPair(
					"data.partiteai_workspace.by_name", "id",
					"partiteai_workspace.test", "id"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_workspace.by_name", "scope",
					"partiteai_workspace.test", "scope"),
			),
		},
		// A rename is the one update the API accepts, so it must not replace.
		resource.TestStep{
			Config: `
resource "partiteai_workspace" "test" {
  name  = "acc-workspace-renamed"
  scope = "GLOBAL"
}
`,
			Check: resource.TestCheckResourceAttr("partiteai_workspace.test", "name", "acc-workspace-renamed"),
		},
		resource.TestStep{
			Config: `
resource "partiteai_workspace" "test" {
  name  = "acc-workspace-renamed"
  scope = "GLOBAL"
}
`,
			ResourceName:      "partiteai_workspace.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// TestAccWorkspace_scopeForcesReplacement pins that changing scope replaces the
// workspace rather than trying to update it, which the API cannot do.
func TestAccWorkspace_scopeForcesReplacement(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_workspace" "test" {
  name  = "acc-scope"
  scope = "GLOBAL"
}
`,
			Check: resource.TestCheckResourceAttr("partiteai_workspace.test", "scope", "GLOBAL"),
		},
		resource.TestStep{
			Config: `
resource "partiteai_workspace" "test" {
  name  = "acc-scope"
  scope = "CONVERSATION"
}
`,
			Check: resource.TestCheckResourceAttr("partiteai_workspace.test", "scope", "CONVERSATION"),
		},
	)
}

// TestAccWorkspace_dataSourceNeedsOneKey pins that the data source takes an id
// or a name and not both, since either alone identifies one workspace.
func TestAccWorkspace_dataSourceNeedsOneKey(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: workspaceHCL + `
data "partiteai_workspace" "both" {
  id   = partiteai_workspace.test.id
  name = partiteai_workspace.test.name
}
`,
			ExpectError: regexp.MustCompile(`(?s)Exactly one of these attributes must be configured`),
		},
		resource.TestStep{
			Config: workspaceHCL + `
data "partiteai_workspace" "neither" {
}
`,
			ExpectError: regexp.MustCompile(`(?s)Exactly one of these attributes must be configured`),
		},
		// The steps above never applied, so leave behind a configuration the
		// test framework can tear down.
		resource.TestStep{Config: workspaceHCL},
	)
}

// TestAccWorkspace_dataSourceMissingName reports a name that matches nothing
// rather than returning an empty workspace.
func TestAccWorkspace_dataSourceMissingName(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
data "partiteai_workspace" "missing" {
  name = "no-workspace-has-this-name"
}
`,
			ExpectError: regexp.MustCompile(`(?s)Workspace Not Found`),
		},
		resource.TestStep{Config: workspaceHCL},
	)
}
