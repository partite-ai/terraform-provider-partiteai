package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories wires the provider under test into the
// Terraform test framework. No registry or installed binary is involved.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"partiteai": providerserver.NewProtocol6WithError(New("test")()),
}

// providerSchema returns the full protocol schema the provider advertises,
// failing the test on any schema diagnostic.
func providerSchema(t *testing.T) *tfprotov6.GetProviderSchemaResponse {
	t.Helper()

	srv, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatalf("create provider server: %v", err)
	}

	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %v", err)
	}

	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema error: %s\n%s", d.Summary, d.Detail)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	return resp
}
