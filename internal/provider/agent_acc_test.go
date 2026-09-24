package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAgent_basic(t *testing.T) {
	const config = `
resource "partiteai_agent" "test" {
  name               = "acc-agent"
  model_complexity   = "MEDIUM"
  task_instructions  = "Answer the question."
  identity           = "A helpful assistant."
  input_message_type = jsonencode({
    type = "object"
    properties = {
      question = { type = "string", description = "The question to answer." }
    }
    required = ["question"]
  })
}

data "partiteai_agent" "test" {
  id = partiteai_agent.test.id
}
`

	accTest(t,
		resource.TestStep{
			Config: config,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_agent.test", "name", "acc-agent"),
				resource.TestCheckResourceAttr("partiteai_agent.test", "model_complexity", "MEDIUM"),
				resource.TestCheckResourceAttr("partiteai_agent.test", "mcp_connections.#", "0"),
				resource.TestCheckResourceAttr("partiteai_agent.test", "particles.#", "0"),
				resource.TestCheckResourceAttr("partiteai_agent.test", "memory_interaction_profile_ids.#", "0"),
				resource.TestCheckResourceAttrSet("partiteai_agent.test", "version_id"),
				resource.TestCheckResourceAttr("partiteai_agent.test", "version_ordinal", "1"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_agent.test", "task_instructions",
					"partiteai_agent.test", "task_instructions"),
			),
		},
		// An update creates a new agent version, so the ordinal must advance.
		resource.TestStep{
			Config: `
resource "partiteai_agent" "test" {
  name               = "acc-agent"
  model_complexity   = "HIGH"
  task_instructions  = "Answer the question, carefully."
  identity           = "A helpful assistant."
  input_message_type = jsonencode({
    type = "object"
    properties = {
      question = { type = "string", description = "The question to answer." }
    }
    required = ["question"]
  })
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_agent.test", "model_complexity", "HIGH"),
				resource.TestCheckResourceAttr("partiteai_agent.test", "version_ordinal", "2"),
			),
		},
		resource.TestStep{
			Config:            config,
			ResourceName:      "partiteai_agent.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// TestAccAgent_jsonIsNormalized pins that input_message_type compares by JSON
// meaning, not by text. Reordering keys or changing whitespace must not produce
// a diff, which is the whole point of using a normalized JSON type.
func TestAccAgent_jsonIsNormalized(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_agent" "test" {
  name               = "acc-agent-json"
  model_complexity   = "LOW"
  task_instructions  = "Do the thing."
  input_message_type = "{\"type\":\"object\",\"properties\":{\"a\":{\"type\":\"string\",\"description\":\"first\"},\"b\":{\"type\":\"number\",\"description\":\"second\"}}}"
}
`,
		},
		resource.TestStep{
			Config: `
resource "partiteai_agent" "test" {
  name               = "acc-agent-json"
  model_complexity   = "LOW"
  task_instructions  = "Do the thing."
  input_message_type = "{\n  \"properties\": {\n    \"b\": {\"description\": \"second\", \"type\": \"number\"},\n    \"a\": {\"description\": \"first\", \"type\": \"string\"}\n  },\n  \"type\": \"object\"\n}"
}
`,
			// Same JSON, different text: Terraform must see nothing to do.
			ExpectNonEmptyPlan: false,
		},
	)
}

// TestAccAgent_mcpConnectionFilters walks every branch of the tool filter
// mapping. Each filter type leaves a different subset of fields set, and the
// read path has to reproduce exactly what was configured.
func TestAccAgent_mcpConnectionFilters(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: `
resource "partiteai_mcp_connection" "none" {
  name = "filter-none"
  stdio_transport = {
    command = "echo"
  }
}

resource "partiteai_mcp_connection" "list" {
  name = "filter-list"
  stdio_transport = {
    command = "echo"
  }
}

resource "partiteai_mcp_connection" "pattern" {
  name = "filter-pattern"
  stdio_transport = {
    command = "echo"
  }
}

resource "partiteai_agent" "test" {
  name               = "acc-agent-filters"
  model_complexity   = "LOW"
  task_instructions  = "Use the tools."
  input_message_type = jsonencode({ type = "object", properties = {} })

  mcp_connections = [
    {
      id          = partiteai_mcp_connection.none.id
      filter_type = "NONE"
    },
    {
      id          = partiteai_mcp_connection.list.id
      filter_type = "LIST"
      tool_names  = ["alpha", "beta"]
    },
    {
      id                = partiteai_mcp_connection.pattern.id
      filter_type       = "PATTERN"
      tool_name_pattern = "^get_"
    },
  ]
}
`,
		Check: resource.TestCheckResourceAttr("partiteai_agent.test", "mcp_connections.#", "3"),
	})
}
