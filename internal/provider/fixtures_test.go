package provider

// Configuration fragments shared by acceptance tests. Most resources sit
// downstream of a model set, so tests compose these rather than repeating the
// chain.

// baseChain is a model provider, profile and set. Reference the set as
// partiteai_model_set.base.id.
const baseChain = `
resource "partiteai_model_provider" "base" {
  probe = false
  name = "base-provider"
  openai = {
    api_key_wo = "sk-test"
  }
}

resource "partiteai_model_profile" "base" {
  probe = false
  name              = "base-profile"
  model_provider_id = partiteai_model_provider.base.id
  configuration = {
    model      = "gpt-5"
    max_tokens = 1024
  }
}

resource "partiteai_model_set" "base" {
  name = "base-set"

  default_model_profile = {
    id = partiteai_model_profile.base.id
  }

  rules = [
    {
      model_profile = { id = partiteai_model_profile.base.id }
      complexity    = "LOW"
    },
  ]
}
`

// baseMesh adds a mesh on top of baseChain, as partiteai_mesh.base.
const baseMesh = baseChain + `
resource "partiteai_mesh" "base" {
  name         = "base-mesh"
  model_set_id = partiteai_model_set.base.id
}
`

// baseAgent adds a minimal agent, as partiteai_agent.base.
const baseAgent = `
resource "partiteai_agent" "base" {
  name               = "base-agent"
  model_complexity   = "MEDIUM"
  task_instructions  = "Answer the question."
  input_message_type = jsonencode({ type = "object", properties = {} })
}
`
