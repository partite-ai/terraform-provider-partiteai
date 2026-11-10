package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const routeProd = "partiteai_route.prod"

const routeDataSource = `
data "partiteai_route" "found" {
  id = partiteai_route.prod.id
}
`

func TestAccRoute_basic(t *testing.T) {
	labeled := labelMesh() + labelBlock("a", "stable")
	updated := `
resource "partiteai_route" "prod" {
  mesh_id = partiteai_mesh.base.id
  name    = "prod"
  segments = [
    { mesh_version_id = partiteai_mesh_version_label.a.mesh_version_id, weight = 70 },
    { weight = 30 },
  ]
}
` + routeDataSource
	accTest(t,
		resource.TestStep{
			Config: labeled + `
resource "partiteai_route" "prod" {
  mesh_id     = partiteai_mesh.base.id
  name        = "prod"
  description = "Half and half."
  segments = [
    { mesh_version_id = partiteai_mesh_version_label.a.mesh_version_id, weight = 50 },
    { weight = 50 },
  ]
}
` + routeDataSource,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.partiteai_route.found", "mesh_id", routeProd, "mesh_id"),
				resource.TestCheckResourceAttr("data.partiteai_route.found", "description", "Half and half."),
				resource.TestCheckTypeSetElemAttrPair("data.partiteai_route.found", "segments.*.mesh_version_id", labelA, "mesh_version_id"),
			),
		},
		resource.TestStep{
			Config: labeled + updated,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(routeProd, plancheck.ResourceActionUpdate),
				},
			},
		},
		// A second label freezes the draft. The draft segment still reads as
		// the draft, so the empty-plan check after apply passes.
		resource.TestStep{
			Config: labeled + labelBlock("b", "canary") + updated,
		},
		resource.TestStep{
			Config:            labeled + labelBlock("b", "canary") + updated,
			ResourceName:      routeProd,
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

func TestAccRoute_weightsMustAddUpTo100(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: labelMesh() + `
resource "partiteai_route" "prod" {
  mesh_id  = partiteai_mesh.base.id
  name     = "prod"
  segments = [{ weight = 99 }]
}
`,
		ExpectError: regexp.MustCompile(`(?s)add\s+up\s+to\s+99`),
	})
}

func TestAccRoute_meshChangeReplaces(t *testing.T) {
	route := func(mesh string) string {
		return labelMesh() + `
resource "partiteai_mesh" "other" {
  name         = "other-mesh"
  model_set_id = partiteai_model_set.base.id
}

resource "partiteai_route" "prod" {
  mesh_id  = ` + mesh + `.id
  name     = "prod"
  segments = [{ weight = 100 }]
}
`
	}
	accTest(t,
		resource.TestStep{
			Config: route("partiteai_mesh.base"),
		},
		resource.TestStep{
			Config: route("partiteai_mesh.other"),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(routeProd, plancheck.ResourceActionReplace),
				},
			},
		},
	)
}
