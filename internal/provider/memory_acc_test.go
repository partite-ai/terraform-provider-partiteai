package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const embeddingProfileHCL = `
resource "partiteai_model_provider" "embed" {
  name = "embed-provider"
  openai = { api_key = "sk-test" }
}

resource "partiteai_embedding_profile" "test" {
  name              = "acc-embedding"
  model_provider_id = partiteai_model_provider.embed.id
  openai = {
    model      = "text-embedding-3-small"
    dimensions = 1536
  }
}
`

func TestAccEmbeddingProfile_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: embeddingProfileHCL + `
data "partiteai_embedding_profile" "test" {
  id = partiteai_embedding_profile.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_embedding_profile.test", "openai.model", "text-embedding-3-small"),
				resource.TestCheckResourceAttr("partiteai_embedding_profile.test", "dimensions", "1536"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_embedding_profile.test", "dimensions",
					"partiteai_embedding_profile.test", "dimensions"),
			),
		},
		resource.TestStep{
			Config:            embeddingProfileHCL,
			ResourceName:      "partiteai_embedding_profile.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

const memoryStoreHCL = embeddingProfileHCL + `
resource "partiteai_memory_store" "test" {
  name                 = "acc-memory-store"
  embedding_profile_id = partiteai_embedding_profile.test.id
  ingest_conversations = true

  scopes = [
    {
      kind  = "SLICED"
      key   = "user_id"
      match = "user_id"
    },
  ]
}
`

func TestAccMemoryStore_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: memoryStoreHCL + `
data "partiteai_memory_store" "test" {
  id = partiteai_memory_store.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_memory_store.test", "name", "acc-memory-store"),
				resource.TestCheckResourceAttr("partiteai_memory_store.test", "ingest_conversations", "true"),
				resource.TestCheckResourceAttr("partiteai_memory_store.test", "scopes.#", "1"),
				resource.TestCheckResourceAttr("partiteai_memory_store.test", "scopes.0.kind", "SLICED"),
				resource.TestCheckResourceAttrSet("partiteai_memory_store.test", "scopes.0.id"),
			),
		},
		resource.TestStep{
			Config:            memoryStoreHCL,
			ResourceName:      "partiteai_memory_store.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

const memoryInteractionProfileHCL = `
resource "partiteai_memory_interaction_profile" "test" {
  name            = "acc-interaction"
  allow_expansion = true

  conversation_history = {
    token_budget = 2048
    selection    = "RECENT"
  }

  context_injections = [
    {
      name         = "what_we_know"
      description  = "What the user has told us before."
      token_budget = 512
      record_types = ["FACT"]
      selection = {
        kind = "ALL"
      }
      query = {
        kind = "DERIVED"
      }
    },
  ]

  query_tools = [
    {
      tool_name    = "search_memory"
      description  = "Search what the user told us before."
      token_budget = 1024
      record_types = ["CHUNK", "FACT"]
      selection = {
        kind = "ALL"
      }
    },
  ]
}
`

func TestAccMemoryInteractionProfile_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: memoryInteractionProfileHCL + `
data "partiteai_memory_interaction_profile" "test" {
  id = partiteai_memory_interaction_profile.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_memory_interaction_profile.test", "name", "acc-interaction"),
				resource.TestCheckResourceAttr("partiteai_memory_interaction_profile.test", "conversation_history.token_budget", "2048"),
				resource.TestCheckResourceAttr("partiteai_memory_interaction_profile.test", "query_tools.#", "1"),
			),
		},
		resource.TestStep{
			Config:            memoryInteractionProfileHCL,
			ResourceName:      "partiteai_memory_interaction_profile.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}
