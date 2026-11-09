package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const routeProd = "partiteai_route.prod"

// routeBlock is a route on the given mesh. A segment is "<mesh_version_id expression>=<weight>",
// or "draft=<weight>" for the draft.
func routeBlock(meshID, description string, segments ...string) string {
	var lines []string
	for _, s := range segments {
		target, weight, _ := strings.Cut(s, "=")
		if target == "draft" {
			lines = append(lines, fmt.Sprintf("    { weight = %s },", weight))
			continue
		}
		lines = append(lines, fmt.Sprintf("    { mesh_version_id = %s, weight = %s },", target, weight))
	}
	descriptionLine := ""
	if description != "" {
		descriptionLine = fmt.Sprintf("  description = %q\n", description)
	}
	return fmt.Sprintf(`
resource "partiteai_route" "prod" {
  mesh_id  = %s
  name     = "prod"
%s  segments = [
%s
  ]
}
`, meshID, descriptionLine, strings.Join(lines, "\n"))
}

const otherMesh = `
resource "partiteai_mesh" "other" {
  name         = "other-mesh"
  model_set_id = partiteai_model_set.base.id
}
`

func TestAccRoute_draftSegment(t *testing.T) {
	config := labelMesh("x") + routeBlock("partiteai_mesh.base.id", "", "draft=100")
	accTest(t,
		resource.TestStep{
			Config: config,
		},
		resource.TestStep{
			Config:            config,
			ResourceName:      routeProd,
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// The draft segment still reads as the draft after the second label freezes
// the draft, so the route does not change.
func TestAccRoute_draftSegmentFollowsTheDraft(t *testing.T) {
	route := routeBlock("partiteai_mesh.base.id", "", labelA+".id=90", "draft=10")
	accTest(t,
		resource.TestStep{
			Config: labelMesh("x") + labelBlock("a", "stable") + route,
		},
		resource.TestStep{
			Config: labelMesh("x") + labelBlock("a", "stable") + labelBlock("b", "canary") + route,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(routeProd, plancheck.ResourceActionNoop),
				},
			},
		},
	)
}

func TestAccRoute_updateInPlace(t *testing.T) {
	var id string
	accTest(t,
		resource.TestStep{
			Config: labelMesh("x") + labelBlock("a", "stable") +
				routeBlock("partiteai_mesh.base.id", "Half and half.", labelA+".id=50", "draft=50"),
			Check: captureID(routeProd, &id),
		},
		resource.TestStep{
			Config: labelMesh("x") + labelBlock("a", "stable") +
				routeBlock("partiteai_mesh.base.id", "", labelA+".id=70", "draft=30"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPtr(routeProd, "id", &id),
				resource.TestCheckNoResourceAttr(routeProd, "description"),
			),
		},
	)
}

func TestAccRoute_weightsMustAddUpTo100(t *testing.T) {
	accTest(t, resource.TestStep{
		Config:      labelMesh("x") + routeBlock("partiteai_mesh.base.id", "", "draft=99"),
		ExpectError: regexp.MustCompile(`add up to 99`),
	})
}

func TestAccRoute_meshChangeReplaces(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: labelMesh("x") + otherMesh + routeBlock("partiteai_mesh.base.id", "", "draft=100"),
		},
		resource.TestStep{
			Config: labelMesh("x") + otherMesh + routeBlock("partiteai_mesh.other.id", "", "draft=100"),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(routeProd, plancheck.ResourceActionReplace),
				},
			},
		},
	)
}

func TestAccRouteDataSource_findsByID(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: labelMesh("x") + routeBlock("partiteai_mesh.base.id", "All traffic.", "draft=100") + `
data "partiteai_route" "found" {
  id = partiteai_route.prod.id
}
`,
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttrPair("data.partiteai_route.found", "mesh_id", routeProd, "mesh_id"),
			resource.TestCheckResourceAttr("data.partiteai_route.found", "name", "prod"),
			resource.TestCheckResourceAttr("data.partiteai_route.found", "description", "All traffic."),
			resource.TestCheckTypeSetElemNestedAttrs("data.partiteai_route.found", "segments.*", map[string]string{"weight": "100"}),
		),
	})
}
