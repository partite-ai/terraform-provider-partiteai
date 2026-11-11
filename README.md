# Terraform Provider for Partite AI

Manages Partite AI configuration with Terraform: meshes, agents, model
providers and profiles, MCP connections, memory stores, particles, access
points and the rest of what the GraphQL API configures. It works with a
self-hosted mesh and with an enterprise deployment.

Resource and data source docs are in [`docs/`](docs/).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.27, to build it

## Using the provider

```terraform
terraform {
  required_providers {
    partiteai = {
      source = "partiteai/partiteai"
    }
  }
}

provider "partiteai" {
  endpoint  = "https://api.partite.ai/graphql"
  api_token = var.partite_api_token
}
```

`endpoint` and `api_token` can also come from the `PARTITE_AI_ENDPOINT` and
`PARTITE_AI_API_TOKEN` environment variables. Resources that exist only in an
enterprise deployment, such as `partiteai_rubric`, are refused against a
self-hosted mesh; see `enterprise` in [`docs/index.md`](docs/index.md).

## Building

```shell
go install
```

This puts the provider binary in `$GOPATH/bin`. `make` formats, lints, builds,
installs and regenerates the docs.

## Docs

`docs/` is generated from the schema and `examples/`. After changing either,
run:

```shell
make generate
```

## Testing

```shell
make testunit         # schema checks, no backend and no Terraform binary
make test             # everything, against a local mesh
make testenterprise   # the acceptance tests against mesh-enterprise
make testall          # both backends
```

The acceptance tests start their backend themselves. See
[TESTING.md](TESTING.md) for what each layer covers and what it needs.
