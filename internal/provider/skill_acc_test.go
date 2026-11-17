package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const skillAccAgent = `
resource "partiteai_agent" "poet" {
  name               = "acc-skill-poet"
  model_complexity   = "MEDIUM"
  task_instructions  = "Write poems."
  input_message_type = jsonencode({
    type = "object"
    properties = {
      topic = { type = "string", description = "What to write about." }
    }
    required = ["topic"]
  })
  skills             = [{ id = partiteai_skill.haiku.id }]
}
`

const skillAccRenamed = `
resource "partiteai_skill" "haiku" {
  name = "acc-haiku-classic"
  files = {
    "SKILL.md" = <<-EOT
      ---
      name: haiku
      description: Write haiku about nature.
      ---

      Five, seven, five.
    EOT
  }
}
`

func TestAccSkill_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_skill" "haiku" {
  name = "acc-haiku"
  files = {
    "SKILL.md" = <<-EOT
      ---
      name: haiku
      description: Write haiku.
      license: MIT
      ---

      Five, seven, five. Read examples/pond.md first.
    EOT
    "examples/pond.md" = "An old silent pond, $${a frog} jumps in"
  }
}

data "partiteai_skill" "haiku" {
  name = partiteai_skill.haiku.name
}
` + skillAccAgent,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "name", "acc-haiku"),
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "declared_name", "haiku"),
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "description", "Write haiku."),
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "instructions", "Five, seven, five. Read examples/pond.md first."),
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "files.%", "2"),
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "files.examples/pond.md", "An old silent pond, ${a frog} jumps in"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_skill.haiku", "id",
					"partiteai_skill.haiku", "id"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_skill.haiku", "files.SKILL.md",
					"partiteai_skill.haiku", "files.SKILL.md"),
				resource.TestCheckResourceAttr("partiteai_agent.poet", "skills.#", "1"),
				resource.TestCheckResourceAttrPair(
					"partiteai_agent.poet", "skills.0.id",
					"partiteai_skill.haiku", "id"),
			),
		},
		// A rename and new files change the skill in place, and the agent keeps
		// using it.
		resource.TestStep{
			Config: skillAccRenamed + skillAccAgent,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "name", "acc-haiku-classic"),
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "description", "Write haiku about nature."),
				resource.TestCheckResourceAttr("partiteai_skill.haiku", "files.%", "1"),
				resource.TestCheckResourceAttr("partiteai_agent.poet", "skills.#", "1"),
			),
		},
		resource.TestStep{
			Config:            skillAccRenamed + skillAccAgent,
			ResourceName:      "partiteai_skill.haiku",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// A path the API would store differently is refused before anything is sent,
// since the configuration would otherwise differ from the skill on every plan.
func TestAccSkill_pathMustBeClean(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_skill" "haiku" {
  name  = "acc-unclean"
  files = { "./SKILL.md" = "---\nname: haiku\ndescription: d\n---\nbody\n" }
}
`,
			ExpectError: regexp.MustCompile(`Invalid skill file path`),
		},
	)
}

// An agent knows a skill by the name it declares, so it can't use two that
// declare the same one.
func TestAccSkill_agentRefusesTwoOfOneDeclaredName(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_skill" "haiku" {
  name  = "acc-haiku-a"
  files = { "SKILL.md" = "---\nname: haiku\ndescription: d\n---\nbody\n" }
}

resource "partiteai_skill" "other_haiku" {
  name  = "acc-haiku-b"
  files = { "SKILL.md" = "---\nname: haiku\ndescription: d\n---\nother body\n" }
}

resource "partiteai_agent" "poet" {
  name               = "acc-skill-two-haiku"
  model_complexity   = "MEDIUM"
  task_instructions  = "Write poems."
  input_message_type = jsonencode({
    type = "object"
    properties = {
      topic = { type = "string", description = "What to write about." }
    }
    required = ["topic"]
  })
  skills = [
    { id = partiteai_skill.haiku.id },
    { id = partiteai_skill.other_haiku.id },
  ]
}
`,
			ExpectError: regexp.MustCompile(`both declare the name`),
		},
	)
}

// A file too large for its content to be returned is read back from its
// download URL, so it shows no drift and imports whole.
func TestAccSkill_largeFile(t *testing.T) {
	const largeConfig = `
resource "partiteai_skill" "large" {
  name = "acc-large"
  files = {
    "SKILL.md" = <<-EOT
      ---
      name: large
      description: Has a large file.
      ---

      Read large.md.
    EOT
    "large.md" = format("%70000s", "end")
  }
}
`
	accTest(t,
		resource.TestStep{
			Config: largeConfig,
			Check: resource.TestCheckResourceAttrWith("partiteai_skill.large", "files.large.md", func(v string) error {
				if len(v) != 70000 || !strings.HasSuffix(v, "end") {
					return fmt.Errorf("large.md is %d bytes, want 70000 ending in end", len(v))
				}
				return nil
			}),
		},
		resource.TestStep{
			Config:            largeConfig,
			ResourceName:      "partiteai_skill.large",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}
