package provider

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// webhookHandlerAccConfig is webhookConfig's destination with a handler, whose
// code is the file at sourcePath and whose secrets and secrets_version are
// the given HCL.
func webhookHandlerAccConfig(sourcePath, secrets string, secretsVersion int) string {
	return meshInputConfig + fmt.Sprintf(`
resource "partiteai_webhook_destination" "test" {
  name       = "acc-webhook"
  mesh_id    = partiteai_mesh.base.id
  input_name = partiteai_mesh_input.test.name
  handler = {
    type            = "JAVASCRIPT"
    source_path     = %q
    config          = { region = "eu" }
    secrets_wo      = %s
    secrets_version = %d
  }
}

data "partiteai_webhook_destination" "test" {
  id = partiteai_webhook_destination.test.id
}
`, sourcePath, secrets, secretsVersion)
}

// webhookAcceptsToken checks whether the destination's handler accepts a
// request that sends token. The test handlers answer 403 for their token and
// 401 for any other, so no request ever starts a run.
func webhookAcceptsToken(token string, accepted bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["partiteai_webhook_destination.test"]
		if !ok {
			return fmt.Errorf("no webhook destination in state")
		}
		req, err := http.NewRequest(http.MethodPost, rs.Primary.Attributes["invoke_url"], strings.NewReader(`{}`))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Token", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		want := http.StatusUnauthorized
		if accepted {
			want = http.StatusForbidden
		}
		if resp.StatusCode != want {
			return fmt.Errorf("token %q: status %d, want %d", token, resp.StatusCode, want)
		}
		return nil
	}
}

func TestAccWebhookDestination_handler(t *testing.T) {
	v1Hash, err := hashFile("testdata/webhook_handler.js")
	if err != nil {
		t.Fatal(err)
	}
	v2Hash, err := hashFile("testdata/webhook_handler_v2.js")
	if err != nil {
		t.Fatal(err)
	}
	const name = "partiteai_webhook_destination.test"
	const data = "data.partiteai_webhook_destination.test"

	accTest(t,
		resource.TestStep{
			Config: webhookHandlerAccConfig("testdata/webhook_handler.js", `{ token = "t1", unused = "u1" }`, 1),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "handler.type", "JAVASCRIPT"),
				resource.TestCheckResourceAttr(name, "handler.source_hash", v1Hash),
				resource.TestCheckResourceAttr(name, "handler.config.region", "eu"),
				resource.TestCheckResourceAttr(name, "handler.secret_names.#", "2"),
				resource.TestCheckNoResourceAttr(name, "handler.secrets_wo"),
				resource.TestCheckResourceAttrPair(data, "handler.source_hash", name, "handler.source_hash"),
				resource.TestCheckResourceAttr(data, "handler.secret_names.#", "2"),
				webhookAcceptsToken("t1", true),
				webhookAcceptsToken("t2", false),
			),
		},
		// A name removed and another added take effect without a version
		// change, and the kept secret keeps its value.
		resource.TestStep{
			Config: webhookHandlerAccConfig("testdata/webhook_handler.js", `{ token = "t1", added = "a1" }`, 1),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "handler.secret_names.#", "2"),
				resource.TestCheckTypeSetElemAttr(name, "handler.secret_names.*", "token"),
				resource.TestCheckTypeSetElemAttr(name, "handler.secret_names.*", "added"),
				webhookAcceptsToken("t1", true),
			),
		},
		// A changed value is sent when secrets_version changes.
		resource.TestStep{
			Config: webhookHandlerAccConfig("testdata/webhook_handler.js", `{ token = "t2", added = "a1" }`, 2),
			Check: resource.ComposeAggregateTestCheckFunc(
				webhookAcceptsToken("t2", true),
				webhookAcceptsToken("t1", false),
			),
		},
		// New code is uploaded, and the secrets stay.
		resource.TestStep{
			Config: webhookHandlerAccConfig("testdata/webhook_handler_v2.js", `{ token = "t2", added = "a1" }`, 2),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "handler.source_hash", v2Hash),
				webhookAcceptsToken("t2", true),
			),
		},
		resource.TestStep{
			Config:            webhookHandlerAccConfig("testdata/webhook_handler_v2.js", `{ token = "t2", added = "a1" }`, 2),
			ResourceName:      name,
			ImportState:       true,
			ImportStateVerify: true,
			// The server doesn't know the version Terraform numbered the
			// secrets with, so an import starts it at 1.
			ImportStateVerifyIgnore: []string{"handler.secrets_version"},
		},
		// Removing the block removes the handler.
		resource.TestStep{
			Config: webhookConfig,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckNoResourceAttr(name, "handler.type"),
				resource.TestCheckNoResourceAttr(data, "handler.type"),
			),
		},
	)
}
