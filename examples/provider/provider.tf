provider "partiteai" {
  endpoint  = "https://api.partite.ai/graphql"
  api_token = var.partite_api_token
}

variable "partite_api_token" {
  type      = string
  sensitive = true
}
