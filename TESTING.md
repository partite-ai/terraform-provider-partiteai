# Testing the provider

The suite has two layers. The first needs nothing installed and runs in under a
second. The second drives real Terraform against a real backend and takes tens
of seconds.

```
make testunit         # fast layer, no backend, no Terraform binary
make test             # everything, against a local mesh
make testenterprise   # the same acceptance tests against mesh-enterprise
make testall          # both backends
```

## Fast layer

`internal/provider/schema_test.go` and `helpers_test.go` run against the
provider in-process:

- **Schema snapshot.** `testdata/provider_schema.json` is the exact schema
  Terraform sees, taken straight off the plugin protocol. Any change to any
  attribute — type, required, computed, sensitive, write-only, nested shape —
  shows up as a diff. This catches the accidental rename or removal that breaks
  every configuration in the wild.
- **Schema/model consistency.** Every `tfsdk` field on a model has a matching
  schema attribute and the reverse. A mismatch otherwise surfaces as a runtime
  conversion error on whichever code path touches it.
- **Resource/data source parity**, and the import contract that every resource
  has a computed `id`.
- **Mapping helpers.** `setFields` is the seam every read path runs through, so
  the null, empty, pointer and JSON cases are pinned directly.

When a schema change is intended:

```
make updategolden
```

Then read the diff before committing it. The diff *is* the review.

## Acceptance layer

`accTest` in `acc_test.go` starts a backend, points the provider at it, and runs
the configuration through Terraform. It runs the same steps against every
backend `PARTITE_TEST_BACKENDS` selects, as a subtest each, so a test written
once covers both products.

| `PARTITE_TEST_BACKENDS` | Runs against |
| --- | --- |
| unset or `mesh` | local mesh (the default) |
| `enterprise` | mesh-enterprise |
| `all` | both |

### mesh

A Partite Mesh server in local mode. It keeps everything in SQLite under a temp
dir and makes no outbound calls, so each test gets a clean backend with no cloud
account and no cost. One server per test.

Every step gets two checks beyond whatever the test asserts:

- **An empty plan after apply and refresh.** This is the single most valuable
  check for a provider. It catches the whole family of mapping bugs where what
  the read path produces differs from what the configuration asked for, which
  users experience as a resource that never stops wanting to change.
- **Import round-trips.** `ImportStateVerify` compares imported state against
  applied state attribute by attribute, which catches fields the read path
  forgets to populate.

### mesh-enterprise

The enterprise backend is not open source, so nothing in this repository
describes how it runs. That checkout ships a test harness command that owns
every detail of bringing it up; `internal/meshtest/enterprise.go` starts that
command, reads JSON from it and asks it for a tenant. The contract is
documented in the mesh-enterprise repository under `cmd/tf-test-harness`.

What this side relies on, and all it relies on:

- The harness prints one line when its backend is ready, with an endpoint the
  provider can be pointed at. This takes a while, so one harness serves the
  whole test binary.
- Asking for a tenant returns credentials that reach that tenant and no other.
  Each test gets its own, so it sees an empty world however many tests ran
  before it. `TestAccEnterprise_tenantsAreIsolated` is the check on that claim.
- Closing the harness's stdin destroys everything it created, and a crashed run
  cannot leave state behind.

Set `PARTITE_TF_HARNESS_INSTANCE` to different values to run two test runs on
one machine at the same time.

If that contract needs to grow — a fixture a test cannot build through the
provider, say — add it to the protocol rather than reaching around it from
here.

### Finding a backend

The harness looks for a mesh checkout, in order:

1. `PARTITE_MESH_SERVER` — path to an already-built server binary.
2. `PARTITE_MESH_DIR` — path to a mesh checkout, which it builds once per run.
3. A `mesh` directory beside this repo, walking up from the working directory.

For enterprise it looks at `PARTITE_TF_TEST_HARNESS` (an already-built
harness), then `PARTITE_MESH_ENTERPRISE_DIR`, then a `mesh-enterprise`
directory beside this repo. It needs Docker on `PATH`.

With none of those, the acceptance tests **skip** with a message saying so
rather than fail. `PARTITE_SKIP_MESH_TESTS=1` skips them outright.

Terraform itself does not need to be installed; the test framework downloads it
when it is missing. Write-only attributes need Terraform 1.11 or newer, and
tests below that version skip.

### Writing one

```go
func TestAccThing_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: `
resource "partiteai_thing" "test" {
  name = "acc-thing"
}
`,
			Check: resource.TestCheckResourceAttr("partiteai_thing.test", "name", "acc-thing"),
		},
		resource.TestStep{
			Config:            sameConfig,
			ResourceName:      "partiteai_thing.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}
```

Most resources sit downstream of a model set. `fixtures_test.go` holds the
chain as `baseChain`, `baseMesh` and `baseAgent` so tests compose them rather
than repeating it.

A test that only makes sense on one backend guards itself with
`backendEnabled("enterprise")` and starts that backend directly, as the tests in
`enterprise_acc_test.go` do.

### Provider aliases do not work here

`ProtoV6ProviderFactories` hands Terraform one provider server for the whole
run, so a configuration with two `provider "partiteai"` blocks under aliases
ends up with both of them using whichever was configured last. There is no
error; resources simply go to the wrong backend. A test that needs to talk to
two tenants at once has to go over the API directly, which is why
`TestAccEnterprise_tenantsAreIsolated` does.

## Known gaps

- MCP connections using `OAUTH2_CLIENT_CREDENTIALS` are not covered, because
  creating one fetches the authorization server's metadata over the network.
- Evaluation resources (`partiteai_rubric`, `partiteai_user_metric`,
  `partiteai_eval_mesh_monitor`) and the `partiteai_system_metric` data source
  exist only on an enterprise deployment, so their acceptance tests run against
  that backend alone, through `accTestEnterprise`. `TestAccEval_needsEnterprise`
  covers what the provider says anywhere else.
- Eval suites, cases, fixtures, plans and their runs have no provider resources.
  A case is captured from a recorded trace rather than declared, so it does not
  fit Terraform without an import-only shape.
