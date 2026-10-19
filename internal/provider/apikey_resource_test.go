package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// State written before the token attribute existed has only the key ID and
// secret, which make up the token.
func TestApiKeyTokenFilledFromSecret(t *testing.T) {
	m := ApiKeyResourceModel{
		ApiKeyBaseModel: ApiKeyBaseModel{Kid: types.StringValue("abc")},
		Secret:          types.StringValue("def"),
		Token:           types.StringNull(),
	}
	m.fillTokenFromSecret()
	if got := m.Token.ValueString(); got != "prt_ak_abc_def" {
		t.Errorf("token = %q", got)
	}

	m.Token = types.StringValue("prt_ak_from_server")
	m.fillTokenFromSecret()
	if got := m.Token.ValueString(); got != "prt_ak_from_server" {
		t.Errorf("token = %q, want the one already in state", got)
	}

	imported := ApiKeyResourceModel{
		ApiKeyBaseModel: ApiKeyBaseModel{Kid: types.StringValue("abc")},
		Secret:          types.StringNull(),
		Token:           types.StringUnknown(),
	}
	imported.fillTokenFromSecret()
	if !imported.Token.IsNull() {
		t.Errorf("token = %v, want null when there is no secret", imported.Token)
	}
}
