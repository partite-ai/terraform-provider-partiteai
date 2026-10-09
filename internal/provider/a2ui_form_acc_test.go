package provider

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// a2uiExportedFormAccConfig is the form of a2uiBasicFormAccConfig as
// mesh-enterprise's tfexport writes it from its definition.
const a2uiExportedFormAccConfig = `
data "partiteai_a2ui_basic_form" "exported" {
  open {
    component {
      id = "root"
      column {
        align    = "center"
        children = ["question", "colors", "sure", "tabs", "pick"]
      }
    }
    component {
      id = "question"
      accessibility {
        label = "Question"
      }
      text {
        text_path = "/question"
        variant   = "h2"
      }
    }
    component {
      id = "colors"
      choice_picker {
        value_path = "/color"
        variant    = "multipleSelection"
        check {
          condition_call = jsonencode({
            args = {
              value = {
                path = "/color"
              }
            }
            call       = "required"
            returnType = "boolean"
          })
          message = "Pick a color."
        }
        option {
          label = "Red"
          value = "red"
        }
      }
      overlay {
        pointer    = "/options"
        expression = "input.colors.map(c, {'label': c, 'value': c})"
      }
    }
    component {
      id = "sure"
      check_box {
        label      = "I'm sure"
        value_path = "/sure"
      }
    }
    component {
      id = "tabs"
      tabs {
        tab {
          child = "help"
          title = "Help"
        }
      }
    }
    component {
      id = "help"
      icon {
        name = "help"
      }
    }
    component {
      id = "pick"
      button {
        child   = "pick_label"
        variant = "primary"
        action {
          event {
            name = "pick"
            context = jsonencode({
              color = {
                path = "/color"
              }
              sure = {
                path = "/sure"
              }
            })
          }
        }
      }
    }
    component {
      id = "pick_label"
      text {
        text = "Pick"
      }
    }
  }
  completed {
    component {
      id = "root"
      list {
        children_template {
          component_id = "picked"
          path         = "/color"
        }
      }
    }
    component {
      id = "picked"
      text {
        text_path = "item"
      }
    }
  }
}
`

// a2uiFormAccTask is a task whose form comes from the given data source.
func a2uiFormAccTask(definition string) string {
	return fmt.Sprintf(`
resource "partiteai_task_definition" "pick" {
  name        = "acc_pick"
  description = "Ask the user to pick a color."
  input_message_type = jsonencode({
    type = "object"
    properties = {
      question = { type = "string", description = "The question to ask." }
      colors   = { type = "array", items = { type = "string" }, description = "The colors to pick from." }
    }
    required = ["question", "colors"]
  })
  output_message_type = jsonencode({
    type = "object"
    properties = {
      color = { type = "array", items = { type = "string" }, description = "The colors picked." }
      sure  = { type = "boolean", description = "Whether the user is sure." }
    }
    required = ["color", "sure"]
  })
  ui_forms = { A2UI = %s }
}
`, definition)
}

const a2uiBasicFormAccConfig = `
data "partiteai_a2ui_basic_form" "pick" {
  open {
    component {
      id = "root"
      column {
        children = ["question", "colors", "sure", "tabs", "pick"]
        align    = "center"
      }
    }
    component {
      id = "question"
      text {
        text_path = "/question"
        variant   = "h2"
      }
      accessibility {
        label = "Question"
      }
    }
    component {
      id = "colors"
      choice_picker {
        value_path = "/color"
        variant    = "multipleSelection"
        option {
          label = "Red"
          value = "red"
        }
        check {
          condition_call = jsonencode({ call = "required", args = { value = { path = "/color" } }, returnType = "boolean" })
          message        = "Pick a color."
        }
      }
      overlay {
        pointer    = "/options"
        expression = "input.colors.map(c, {'label': c, 'value': c})"
      }
    }
    component {
      id = "sure"
      check_box {
        label      = "I'm sure"
        value_path = "/sure"
      }
    }
    component {
      id = "tabs"
      tabs {
        tab {
          title = "Help"
          child = "help"
        }
      }
    }
    component {
      id = "help"
      icon {
        name = "help"
      }
    }
    component {
      id = "pick"
      button {
        child   = "pick_label"
        variant = "primary"
        action {
          event {
            name    = "pick"
            context = jsonencode({ color = { path = "/color" }, sure = { path = "/sure" } })
          }
        }
      }
    }
    component {
      id = "pick_label"
      text {
        text = "Pick"
      }
    }
  }
  completed {
    component {
      id = "root"
      list {
        children_template {
          component_id = "picked"
          path         = "/color"
        }
      }
    }
    component {
      id = "picked"
      text {
        text_path = "item"
      }
    }
  }
}
`

const a2uiGenericFormAccConfig = `
data "partiteai_a2ui_form" "pick" {
  open {
    component {
      id         = "root"
      type       = "Column"
      properties = jsonencode({ children = ["question", "colors", "sure", "tabs", "pick"], align = "center" })
    }
    component {
      id         = "question"
      type       = "Text"
      properties = jsonencode({ text = { path = "/question" }, variant = "h2", accessibility = { label = "Question" } })
    }
    component {
      id   = "colors"
      type = "ChoicePicker"
      properties = jsonencode({
        value   = { path = "/color" }
        variant = "multipleSelection"
        options = [{ label = "Red", value = "red" }]
        checks  = [{ condition = { call = "required", args = { value = { path = "/color" } }, returnType = "boolean" }, message = "Pick a color." }]
      })
      overlay {
        pointer    = "/options"
        expression = "input.colors.map(c, {'label': c, 'value': c})"
      }
    }
    component {
      id         = "sure"
      type       = "CheckBox"
      properties = jsonencode({ label = "I'm sure", value = { path = "/sure" } })
    }
    component {
      id         = "tabs"
      type       = "Tabs"
      properties = jsonencode({ tabs = [{ title = "Help", child = "help" }] })
    }
    component {
      id         = "help"
      type       = "Icon"
      properties = jsonencode({ name = "help" })
    }
    component {
      id   = "pick"
      type = "Button"
      properties = jsonencode({
        child   = "pick_label"
        variant = "primary"
        action  = { event = { name = "pick", context = { color = { path = "/color" }, sure = { path = "/sure" } } } }
      })
    }
    component {
      id         = "pick_label"
      type       = "Text"
      properties = jsonencode({ text = "Pick" })
    }
  }
  completed {
    component {
      id         = "root"
      type       = "List"
      properties = jsonencode({ children = { componentId = "picked", path = "/color" } })
    }
    component {
      id         = "picked"
      type       = "Text"
      properties = jsonencode({ text = { path = "item" } })
    }
  }
}
`

func TestAccA2UIForms(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: a2uiBasicFormAccConfig + a2uiGenericFormAccConfig + a2uiFormAccTask("data.partiteai_a2ui_basic_form.pick.definition"),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair(
					"data.partiteai_a2ui_basic_form.pick", "definition",
					"data.partiteai_a2ui_form.pick", "definition"),
				resource.TestCheckResourceAttrPair(
					"partiteai_task_definition.pick", "ui_forms.A2UI",
					"data.partiteai_a2ui_basic_form.pick", "definition"),
				resource.TestCheckResourceAttrWith("data.partiteai_a2ui_basic_form.pick", "definition", func(value string) error {
					var form struct {
						Open struct {
							Components []map[string]any          `json:"components"`
							Overlays   map[string]map[string]any `json:"overlays"`
						} `json:"open"`
					}
					if err := json.Unmarshal([]byte(value), &form); err != nil {
						return err
					}
					want := map[string]any{"id": "question", "component": "Text", "text": map[string]any{"path": "/question"}, "variant": "h2", "accessibility": map[string]any{"label": "Question"}}
					if !reflect.DeepEqual(form.Open.Components[1], want) {
						return fmt.Errorf("question component = %v", form.Open.Components[1])
					}
					if form.Open.Overlays["colors"]["/options"] == nil {
						return fmt.Errorf("overlays = %v", form.Open.Overlays)
					}
					return nil
				}),
			),
		},
		resource.TestStep{
			Config: a2uiBasicFormAccConfig + a2uiExportedFormAccConfig,
			Check: resource.TestCheckResourceAttrPair(
				"data.partiteai_a2ui_basic_form.exported", "definition",
				"data.partiteai_a2ui_basic_form.pick", "definition"),
		},
		resource.TestStep{
			Config: a2uiBasicFormAccConfig + a2uiGenericFormAccConfig + a2uiFormAccTask("data.partiteai_a2ui_form.pick.definition"),
			Check: resource.TestCheckResourceAttrPair(
				"partiteai_task_definition.pick", "ui_forms.A2UI",
				"data.partiteai_a2ui_form.pick", "definition"),
		},
	)
}

// a2uiBasicFormWith is a basic form whose root is the given component, with a
// button so the form can complete its task.
func a2uiBasicFormWith(root string) string {
	return fmt.Sprintf(`
data "partiteai_a2ui_basic_form" "bad" {
  open {
    component {
      id = "root"
      %s
    }
    component {
      id = "ok"
      button {
        child = "root"
        action {
          event { name = "ok" }
        }
      }
    }
  }
}
`, root)
}

func TestAccA2UIFormsRefused(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config:      a2uiBasicFormWith("text { text = \"a\" }\n divider {}"),
			ExpectError: regexp.MustCompile(`exactly one component type block`),
		},
		resource.TestStep{
			Config:      a2uiBasicFormWith("text {\n text = \"a\"\n text_path = \"/a\"\n }"),
			ExpectError: regexp.MustCompile(`Set only one of text, text_call, text_path`),
		},
		resource.TestStep{
			Config:      a2uiBasicFormWith(`text { variant = "h1" }`),
			ExpectError: regexp.MustCompile(`Set one of text, text_call, text_path`),
		},
		resource.TestStep{
			Config:      a2uiBasicFormWith("text {\n text = \"a\"\n variant = \"h9\"\n }"),
			ExpectError: regexp.MustCompile(`(?s)value must be one of`),
		},
		resource.TestStep{
			Config:      a2uiBasicFormWith(`column { children = ["nope"] }`),
			ExpectError: regexp.MustCompile(`child "nope", which is not a component`),
		},
		resource.TestStep{
			Config: `
data "partiteai_a2ui_form" "bad" {
  open {
    component {
      id         = "root"
      type       = "Txet"
      properties = jsonencode({ text = "a" })
    }
  }
}
`,
			ExpectError: regexp.MustCompile(`the catalog has no component "Txet"`),
		},
		resource.TestStep{
			Config: `
data "partiteai_a2ui_form" "bad" {
  open {
    component {
      id         = "root"
      type       = "Text"
      properties = jsonencode({ text = "a", colour = "red" })
    }
  }
}
`,
			ExpectError: regexp.MustCompile(`Text has no property "colour"`),
		},
	)
}
