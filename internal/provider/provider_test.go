package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

// configureProvider runs Configure with the given string attributes set and
// every other attribute null, and returns what it produced.
func configureProvider(t *testing.T, attrs map[string]string) *provider.ConfigureResponse {
	t.Helper()

	p := &PartiteAIProvider{}
	var schemaResp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &schemaResp)

	objectType, ok := schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatalf("the provider schema is a %T, not an object", schemaResp.Schema.Type().TerraformType(context.Background()))
	}
	values := map[string]tftypes.Value{}
	for name, attrType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	for name, value := range attrs {
		values[name] = tftypes.NewValue(tftypes.String, value)
	}

	req := provider.ConfigureRequest{
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(objectType, values),
		},
	}
	var resp provider.ConfigureResponse
	p.Configure(context.Background(), req, &resp)
	return &resp
}

// sentAuthorization configures the provider, sends one request through its
// HTTP client, and returns the Authorization header the server saw.
func sentAuthorization(t *testing.T, attrs map[string]string) string {
	t.Helper()

	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	attrs["endpoint"] = srv.URL + "/graphql"
	resp := configureProvider(t, attrs)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure: %v", resp.Diagnostics)
	}

	data, ok := resp.ResourceData.(*providerData)
	if !ok {
		t.Fatalf("resource data is a %T, not *providerData", resp.ResourceData)
	}
	res, err := data.httpClient.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return got
}

func TestProviderSendsAPITokenAsBearer(t *testing.T) {
	for _, env := range []string{"PARTITE_AI_API_TOKEN", "PARTITE_AI_API_KEY", "PARTITE_AI_API_SECRET"} {
		t.Setenv(env, "")
	}

	got := sentAuthorization(t, map[string]string{"api_token": "prt_ak_abc_def"})
	if got != "Bearer prt_ak_abc_def" {
		t.Errorf("Authorization = %q", got)
	}
}

func TestProviderBuildsTokenFromDeprecatedKeyAndSecret(t *testing.T) {
	for _, env := range []string{"PARTITE_AI_API_TOKEN", "PARTITE_AI_API_KEY", "PARTITE_AI_API_SECRET"} {
		t.Setenv(env, "")
	}

	got := sentAuthorization(t, map[string]string{"api_key": "abc", "api_secret": "def"})
	if got != "Bearer prt_ak_abc_def" {
		t.Errorf("Authorization = %q", got)
	}
}

func TestProviderReadsAPITokenFromEnv(t *testing.T) {
	t.Setenv("PARTITE_AI_API_TOKEN", "prt_ak_abc_def")
	t.Setenv("PARTITE_AI_API_KEY", "")
	t.Setenv("PARTITE_AI_API_SECRET", "")

	got := sentAuthorization(t, map[string]string{})
	if got != "Bearer prt_ak_abc_def" {
		t.Errorf("Authorization = %q", got)
	}
}

// Configuration wins over the environment, and within each a token wins over
// the old key and secret.
func TestProviderCredentialPrecedence(t *testing.T) {
	cases := map[string]struct {
		attrs map[string]string
		env   map[string]string
	}{
		"config token, env pair": {attrs: map[string]string{"api_token": "prt_ak_abc_def"}, env: map[string]string{"PARTITE_AI_API_KEY": "old", "PARTITE_AI_API_SECRET": "pair"}},
		"env token, env pair":    {attrs: map[string]string{}, env: map[string]string{"PARTITE_AI_API_TOKEN": "prt_ak_abc_def", "PARTITE_AI_API_KEY": "old", "PARTITE_AI_API_SECRET": "pair"}},
		"config pair, env token": {attrs: map[string]string{"api_key": "abc", "api_secret": "def"}, env: map[string]string{"PARTITE_AI_API_TOKEN": "prt_ak_old_pair"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, env := range []string{"PARTITE_AI_API_TOKEN", "PARTITE_AI_API_KEY", "PARTITE_AI_API_SECRET"} {
				t.Setenv(env, tc.env[env])
			}
			got := sentAuthorization(t, tc.attrs)
			if got != "Bearer prt_ak_abc_def" {
				t.Errorf("Authorization = %q", got)
			}
		})
	}
}

func TestProviderRejectsBadCredentials(t *testing.T) {
	cases := map[string]struct {
		attrs map[string]string
		env   map[string]string
		want  string
	}{
		"token and key":         {attrs: map[string]string{"api_token": "prt_ak_abc_def", "api_key": "abc"}, want: "Conflicting API credentials"},
		"token and secret":      {attrs: map[string]string{"api_token": "prt_ak_abc_def", "api_secret": "def"}, want: "Conflicting API credentials"},
		"key without secret":    {attrs: map[string]string{"api_key": "abc"}, want: "Missing API Secret"},
		"secret without key":    {attrs: map[string]string{"api_secret": "def"}, want: "Missing API Key"},
		"no credentials at all": {attrs: map[string]string{}, want: "Missing API Token"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, env := range []string{"PARTITE_AI_API_TOKEN", "PARTITE_AI_API_KEY", "PARTITE_AI_API_SECRET"} {
				t.Setenv(env, tc.env[env])
			}
			tc.attrs["endpoint"] = "http://127.0.0.1:1/graphql"
			resp := configureProvider(t, tc.attrs)
			errs := resp.Diagnostics.Errors()
			if len(errs) != 1 || errs[0].Summary() != tc.want {
				t.Errorf("diagnostics = %v, want one error %q", errs, tc.want)
			}
		})
	}
}
