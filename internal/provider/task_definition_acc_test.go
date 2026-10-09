package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// taskDefinitionAccConfig is a task that asks the user a yes or no question,
// with the given ui_forms line.
func taskDefinitionAccConfig(uiForms string) string {
	return fmt.Sprintf(`
resource "partiteai_task_definition" "confirm" {
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

data "partiteai_task_definition" "confirm" {
  name = partiteai_task_definition.confirm.name
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

func TestAccTaskDefinition_uiForms(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: taskDefinitionAccConfig(confirmForm("Yes")),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_task_definition.confirm", "ui_forms.%", "1"),
				resource.TestMatchResourceAttr("partiteai_task_definition.confirm", "ui_forms.A2UI", regexp.MustCompile(`"text":"Yes"`)),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_task_definition.confirm", "ui_forms.A2UI",
					"partiteai_task_definition.confirm", "ui_forms.A2UI"),
			),
		},
		resource.TestStep{
			Config: taskDefinitionAccConfig(confirmForm("Sure")),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestMatchResourceAttr("partiteai_task_definition.confirm", "ui_forms.A2UI", regexp.MustCompile(`"text":"Sure"`)),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_task_definition.confirm", "ui_forms.A2UI",
					"partiteai_task_definition.confirm", "ui_forms.A2UI"),
			),
		},
		resource.TestStep{
			Config:            taskDefinitionAccConfig(confirmForm("Sure")),
			ResourceName:      "partiteai_task_definition.confirm",
			ImportState:       true,
			ImportStateVerify: true,
		},
		resource.TestStep{
			Config: taskDefinitionAccConfig(""),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_task_definition.confirm", "ui_forms.%", "0"),
				resource.TestCheckResourceAttr("data.partiteai_task_definition.confirm", "ui_forms.%", "0"),
			),
		},
	)
}

func TestAccTaskDefinition_uiFormRefused(t *testing.T) {
	accTest(t,
		resource.TestStep{
			// The button answers with a string where the output wants a boolean.
			Config:      taskDefinitionAccConfig(`ui_forms = { A2UI = jsonencode({ open = { components = [{ id = "root", component = "Button", child = "label", action = { event = { name = "yes", context = { confirmed = "yes" } } } }, { id = "label", component = "Text", text = "Yes" }] } }) }`),
			ExpectError: regexp.MustCompile(`(?i)confirmed`),
		},
	)
}
