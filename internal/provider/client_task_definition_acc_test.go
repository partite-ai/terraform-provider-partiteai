package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// clientTaskDefinitionAccConfig is a task that asks the user a yes or no question,
// with the given ui_forms line.
func clientTaskDefinitionAccConfig(uiForms string) string {
	return fmt.Sprintf(`
resource "partiteai_client_task_definition" "confirm" {
  name        = "acc_confirm"
  description = "Ask the user to confirm."
  input_message_type = jsonencode({
    type       = "object"
    properties = { question = { type = "string", description = "The question to ask." } }
    required   = ["question"]
  })
  output_message_type = jsonencode({
    type       = "object"
    properties = { confirmed = { type = "boolean", description = "Whether the user said yes." } }
    required   = ["confirmed"]
  })
  %s
}

data "partiteai_client_task_definition" "confirm" {
  name = partiteai_client_task_definition.confirm.name
}
`, uiForms)
}

// confirmForm is an A2UI form with a button for each answer.
func confirmForm(yesLabel string) string {
	return fmt.Sprintf(`ui_forms = {
    A2UI = jsonencode({
      open = {
        components = [
          { id = "root", component = "Column", children = ["question", "yes", "no"] },
          { id = "question", component = "Text", text = { path = "/question" } },
          { id = "yes", component = "Button", child = "yes_label", action = { event = { name = "yes", context = { confirmed = true } } } },
          { id = "yes_label", component = "Text", text = %q },
          { id = "no", component = "Button", child = "no_label", action = { event = { name = "no", context = { confirmed = false } } } },
          { id = "no_label", component = "Text", text = "No" },
        ]
      }
    })
  }`, yesLabel)
}

func TestAccClientTaskDefinition_uiForms(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: clientTaskDefinitionAccConfig(confirmForm("Yes")),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_client_task_definition.confirm", "ui_forms.%", "1"),
				resource.TestMatchResourceAttr("partiteai_client_task_definition.confirm", "ui_forms.A2UI", regexp.MustCompile(`"text":"Yes"`)),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_client_task_definition.confirm", "ui_forms.A2UI",
					"partiteai_client_task_definition.confirm", "ui_forms.A2UI"),
			),
		},
		resource.TestStep{
			Config: clientTaskDefinitionAccConfig(confirmForm("Sure")),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestMatchResourceAttr("partiteai_client_task_definition.confirm", "ui_forms.A2UI", regexp.MustCompile(`"text":"Sure"`)),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_client_task_definition.confirm", "ui_forms.A2UI",
					"partiteai_client_task_definition.confirm", "ui_forms.A2UI"),
			),
		},
		resource.TestStep{
			Config:            clientTaskDefinitionAccConfig(confirmForm("Sure")),
			ResourceName:      "partiteai_client_task_definition.confirm",
			ImportState:       true,
			ImportStateVerify: true,
		},
		resource.TestStep{
			Config: clientTaskDefinitionAccConfig(""),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_client_task_definition.confirm", "ui_forms.%", "0"),
				resource.TestCheckResourceAttr("data.partiteai_client_task_definition.confirm", "ui_forms.%", "0"),
			),
		},
	)
}

func TestAccClientTaskDefinition_uiFormRefused(t *testing.T) {
	accTest(t,
		resource.TestStep{
			// The button answers with a string where the output wants a boolean.
			Config:      clientTaskDefinitionAccConfig(`ui_forms = { A2UI = jsonencode({ open = { components = [{ id = "root", component = "Button", child = "label", action = { event = { name = "yes", context = { confirmed = "yes" } } } }, { id = "label", component = "Text", text = "Yes" }] } }) }`),
			ExpectError: regexp.MustCompile(`(?i)confirmed`),
		},
	)
}

// providerWithOldTaskDefinition is the provider with partiteai_task_definition
// added back, as it was before the rename, so a test can make state under the
// old name.
type providerWithOldTaskDefinition struct {
	*PartiteAIProvider
}

func (p providerWithOldTaskDefinition) Resources(ctx context.Context) []func() frameworkresource.Resource {
	return append(p.PartiteAIProvider.Resources(ctx), func() frameworkresource.Resource {
		return &ClientTaskDefinitionResource{BaseResource: NewBaseResource("task_definition")}
	})
}

// moveTaskDefinitionConfig is a client task definition under the given
// resource type, with extra lines after it.
func moveTaskDefinitionConfig(resourceType, extra string) string {
	return fmt.Sprintf(`
resource %q "confirm" {
  name                = "acc_move_confirm"
  description         = "Ask the user to confirm."
  input_message_type  = jsonencode({ type = "object", properties = { question = { type = "string", description = "The question to ask." } } })
  output_message_type = jsonencode({ type = "object", properties = { confirmed = { type = "boolean", description = "Whether the user said yes." } } })
}
%s`, resourceType, extra)
}

// TestAccClientTaskDefinition_movedFromOldName checks that a moved block takes
// a partiteai_task_definition's state over in place, with nothing replaced.
func TestAccClientTaskDefinition_movedFromOldName(t *testing.T) {
	oldProvider := map[string]func() (tfprotov6.ProviderServer, error){
		"partiteai": providerserver.NewProtocol6WithError(providerWithOldTaskDefinition{New("test")().(*PartiteAIProvider)}),
	}
	for _, name := range enabledBackends() {
		t.Run(name, func(t *testing.T) {
			backend := startBackend(t, name)
			resource.Test(t, resource.TestCase{
				TerraformVersionChecks: minTerraform,
				Steps: []resource.TestStep{
					{
						ProtoV6ProviderFactories: oldProvider,
						Config:                   backend.ProviderConfig() + moveTaskDefinitionConfig("partiteai_task_definition", ""),
					},
					{
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						Config: backend.ProviderConfig() + moveTaskDefinitionConfig("partiteai_client_task_definition", `
moved {
  from = partiteai_task_definition.confirm
  to   = partiteai_client_task_definition.confirm
}
`),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectResourceAction("partiteai_client_task_definition.confirm", plancheck.ResourceActionNoop),
							},
						},
						Check: resource.TestCheckResourceAttr("partiteai_client_task_definition.confirm", "name", "acc_move_confirm"),
					},
				},
			})
		})
	}
}
