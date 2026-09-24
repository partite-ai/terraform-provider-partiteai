package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The artifact resource uploads a bundle over multipart, which is the only
// place the provider talks to the API outside of genqlient.
const particleArtifactHCL = `
resource "partiteai_particle_artifact" "test" {
  source_path = "testdata/hello.particle"
}
`

func TestAccParticleArtifact_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: particleArtifactHCL + `
data "partiteai_particle_artifact" "test" {
  id = partiteai_particle_artifact.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("partiteai_particle_artifact.test", "id"),
				resource.TestCheckResourceAttrSet("partiteai_particle_artifact.test", "name"),
				resource.TestCheckResourceAttrSet("partiteai_particle_artifact.test", "version"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_particle_artifact.test", "name",
					"partiteai_particle_artifact.test", "name"),
			),
		},
		resource.TestStep{
			Config:            particleArtifactHCL,
			ResourceName:      "partiteai_particle_artifact.test",
			ImportState:       true,
			ImportStateVerify: true,
			// source_path is a local path the API never returns, and an imported
			// artifact records source_hash as the sentinel "imported" because the
			// bundle it was built from is not on this machine.
			ImportStateVerifyIgnore: []string{"source_path", "source_hash"},
		},
	)
}

const installedParticleHCL = particleArtifactHCL + `
resource "partiteai_installed_particle" "test" {
  name        = "acc-install"
  artifact_id = partiteai_particle_artifact.test.id
}
`

func TestAccInstalledParticle_basic(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: installedParticleHCL + `
data "partiteai_installed_particle" "test" {
  id = partiteai_installed_particle.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_installed_particle.test", "name", "acc-install"),
				resource.TestCheckResourceAttrSet("partiteai_installed_particle.test", "active_artifact.id"),
				resource.TestCheckResourceAttrPair(
					"partiteai_installed_particle.test", "active_artifact.id",
					"partiteai_particle_artifact.test", "id"),
			),
		},
		resource.TestStep{
			Config:            installedParticleHCL,
			ResourceName:      "partiteai_installed_particle.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// TestAccInstalledParticle_credentials exercises the write-only credential
// list. The fixture manifest names its method "login", which is deliberately
// not one of the credential kinds: the API matches `method` against the
// manifest's own method names, and the kind comes from whichever _wo block the
// configuration sets. A schema that treated the two as the same thing could
// not configure this credential at all.
func TestAccInstalledParticle_credentials(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: capableArtifactHCL + `
resource "partiteai_installed_particle" "test" {
  name            = "acc-install-creds"
  artifact_id     = partiteai_particle_artifact.capable.id
  kv_workspace_id = partiteai_workspace.kv.id

  credentials = [
    {
      name   = "service"
      method = "login"
      basic_wo = {
        username = "user"
        password = "pass"
      }
    },
  ]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_installed_particle.test", "credentials.#", "1"),
				resource.TestCheckResourceAttr("partiteai_installed_particle.test", "credentials.0.name", "service"),
				resource.TestCheckResourceAttr("partiteai_installed_particle.test", "credentials.0.method", "login"),
			),
		},
		// Rotating the secret is an update, and the values never enter state,
		// so the only thing to check is that it settles.
		resource.TestStep{
			Config: capableArtifactHCL + `
resource "partiteai_installed_particle" "test" {
  name            = "acc-install-creds"
  artifact_id     = partiteai_particle_artifact.capable.id
  kv_workspace_id = partiteai_workspace.kv.id

  credentials = [
    {
      name    = "service"
      method  = "login"
      version = 2
      basic_wo = {
        username = "user"
        password = "rotated"
      }
    },
  ]
}
`,
			Check: resource.TestCheckResourceAttr("partiteai_installed_particle.test", "credentials.0.version", "2"),
		},
	)
}

// capableArtifactHCL is a fixture artifact whose manifest grants the KV
// capability and declares one filesystem mount, which is what lets an install
// bind workspaces. testdata/hello.particle grants neither.
const capableArtifactHCL = `
resource "partiteai_particle_artifact" "capable" {
  source_path = "testdata/capable.particle"
}

resource "partiteai_workspace" "kv" {
  name  = "acc-install-kv"
  scope = "GLOBAL"
}

resource "partiteai_workspace" "data" {
  name  = "acc-install-data"
  scope = "GLOBAL"
}
`

// TestAccInstalledParticle_workspaces covers the two attributes that bind an
// install to workspaces. Both are replace-all on update, so the test changes
// each of them and checks the result settles.
func TestAccInstalledParticle_workspaces(t *testing.T) {
	const withBindings = capableArtifactHCL + `
resource "partiteai_installed_particle" "test" {
  name            = "acc-install-bound"
  artifact_id     = partiteai_particle_artifact.capable.id
  kv_workspace_id = partiteai_workspace.kv.id

  mounts = [
    {
      mount_name   = "data"
      workspace_id = partiteai_workspace.data.id
    },
  ]
}

data "partiteai_installed_particle" "test" {
  id = partiteai_installed_particle.test.id
}
`

	accTest(t,
		resource.TestStep{
			Config: withBindings,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair(
					"partiteai_installed_particle.test", "kv_workspace_id",
					"partiteai_workspace.kv", "id"),
				resource.TestCheckResourceAttr("partiteai_installed_particle.test", "mounts.#", "1"),
				resource.TestCheckResourceAttr("partiteai_installed_particle.test", "mounts.0.mount_name", "data"),
				resource.TestCheckResourceAttrPair(
					"partiteai_installed_particle.test", "mounts.0.workspace_id",
					"partiteai_workspace.data", "id"),
				// The data source reads both back.
				resource.TestCheckResourceAttrPair(
					"data.partiteai_installed_particle.test", "kv_workspace_id",
					"partiteai_workspace.kv", "id"),
				resource.TestCheckResourceAttr("data.partiteai_installed_particle.test", "mounts.#", "1"),
			),
		},
		// Repointing the KV binding and clearing the mounts are both updates
		// rather than replacements.
		resource.TestStep{
			Config: capableArtifactHCL + `
resource "partiteai_installed_particle" "test" {
  name            = "acc-install-bound"
  artifact_id     = partiteai_particle_artifact.capable.id
  kv_workspace_id = partiteai_workspace.data.id
  mounts          = []
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair(
					"partiteai_installed_particle.test", "kv_workspace_id",
					"partiteai_workspace.data", "id"),
				resource.TestCheckResourceAttr("partiteai_installed_particle.test", "mounts.#", "0"),
			),
		},
		resource.TestStep{
			Config:            withBindings,
			ResourceName:      "partiteai_installed_particle.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// TestAccInstalledParticle_noBindings pins the other side: an artifact that
// grants nothing leaves both attributes empty, and an install of it must not
// drift.
func TestAccInstalledParticle_noBindings(t *testing.T) {
	accTest(t, resource.TestStep{
		Config: installedParticleHCL,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckNoResourceAttr("partiteai_installed_particle.test", "kv_workspace_id"),
			resource.TestCheckResourceAttr("partiteai_installed_particle.test", "mounts.#", "0"),
		),
	})
}

// TestAccInstalledParticle_credentialKindIsRequired pins that a credential
// names exactly one kind. Nothing else can tell the provider which kind it is,
// so neither zero blocks nor two of them is usable.
func TestAccInstalledParticle_credentialKindIsRequired(t *testing.T) {
	accTest(t,
		resource.TestStep{
			Config: capableArtifactHCL + `
resource "partiteai_installed_particle" "test" {
  name        = "acc-install-nokind"
  artifact_id = partiteai_particle_artifact.capable.id

  credentials = [
    {
      name   = "service"
      method = "login"
    },
  ]
}
`,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
		},
		resource.TestStep{
			Config: capableArtifactHCL + `
resource "partiteai_installed_particle" "test" {
  name        = "acc-install-twokinds"
  artifact_id = partiteai_particle_artifact.capable.id

  credentials = [
    {
      name   = "service"
      method = "login"
      basic_wo = {
        username = "user"
        password = "pass"
      }
      raw_wo = {
        value = "also this"
      }
    },
  ]
}
`,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
		},
	)
}
