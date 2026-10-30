package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// nullBlocks is a model with every provider block null, typed as the schema
// types them, as the framework hands it to Read after an import.
func nullBlocks(t *testing.T, attrs map[string]attr.Type) ModelProviderBaseModel {
	t.Helper()
	block := func(name string) types.Object {
		return types.ObjectNull(objectAttrTypes(t, attrs[name], name))
	}
	tags, ok := attrs["tags"].(types.ListType)
	if !ok {
		t.Fatalf("tags is not a list")
	}
	return ModelProviderBaseModel{
		Tags:         types.ListNull(tags.ElemType),
		Anthropic:    block("anthropic"),
		OpenAI:       block("openai"),
		GoogleGenAI:  block("google_genai"),
		OpenAICompat: block("openai_compatible"),
		Fireworks:    block("fireworks"),
		Cerebras:     block("cerebras"),
		Bedrock:      block("bedrock"),
	}
}

// objectAttrTypes is the attribute types of typ, which must be an object.
func objectAttrTypes(t *testing.T, typ attr.Type, name string) map[string]attr.Type {
	t.Helper()
	ot, ok := typ.(types.ObjectType)
	if !ok {
		t.Fatalf("%s is not an object", name)
	}
	return ot.AttrTypes
}

func resourceAttrTypes(t *testing.T) map[string]attr.Type {
	t.Helper()
	var resp resource.SchemaResponse
	NewModelProviderResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return objectAttrTypes(t, resp.Schema.Type(), "schema")
}

func dataSourceAttrTypes(t *testing.T) map[string]attr.Type {
	t.Helper()
	var resp datasource.SchemaResponse
	NewModelProviderDataSource().Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	return objectAttrTypes(t, resp.Schema.Type(), "schema")
}

// providerDetails is a saved provider of the type with the config.
func providerDetails(typ pgql.ModelProviderType, config pgql.ModelProviderDetailsProviderConfigModelProviderConfig) pgql.ModelProviderDetails {
	return pgql.ModelProviderDetails{
		Id:             "mp_1",
		Name:           "provider",
		ProviderType:   typ,
		ProviderConfig: config,
		CreatedAt:      "2026-10-08T00:00:00Z",
		UpdatedAt:      "2026-10-08T00:00:00Z",
	}
}

func openAIDetails() pgql.ModelProviderDetails {
	return providerDetails(pgql.ModelProviderTypeOpenAi,
		&pgql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAI{ApiKeySet: true})
}

// nestedString is the string at the path of nested attributes in obj, or null
// when any part of it is missing.
func nestedString(obj types.Object, path ...string) types.String {
	for i, name := range path {
		if obj.IsNull() || obj.IsUnknown() {
			return types.StringNull()
		}
		v, ok := obj.Attributes()[name]
		if !ok {
			return types.StringNull()
		}
		if i == len(path)-1 {
			if s, ok := v.(types.String); ok {
				return s
			}
			return types.StringNull()
		}
		if obj, ok = v.(types.Object); !ok {
			return types.StringNull()
		}
	}
	return types.StringNull()
}

// The key is write-only, so state never holds it, whatever the server says.
func TestModelProviderResourceStoresNoSecret(t *testing.T) {
	ctx := context.Background()
	attrs := resourceAttrTypes(t)
	m := nullBlocks(t, attrs)

	if diags := m.MapFromGraphQL(ctx, openAIDetails(), writeOnlySecret); diags.HasError() {
		t.Fatal(diags)
	}
	if m.OpenAI.IsNull() {
		t.Fatal("openai block is null")
	}
	if got := m.OpenAI.Attributes()["api_key_wo"]; !got.IsNull() {
		t.Errorf("api_key_wo = %v, want null", got)
	}
}

// What is not a secret reads back from the server: here, the Fireworks
// account id.
func TestModelProviderFireworks(t *testing.T) {
	ctx := context.Background()
	m := nullBlocks(t, resourceAttrTypes(t))
	account := "acct"
	details := providerDetails(pgql.ModelProviderTypeFireworks,
		&pgql.ModelProviderDetailsProviderConfigModelProviderConfigFireworks{ApiKeySet: true, AccountID: &account})

	if diags := m.MapFromGraphQL(ctx, details, writeOnlySecret); diags.HasError() {
		t.Fatal(diags)
	}
	if got := nestedString(m.Fireworks, "account_id"); got.ValueString() != "acct" {
		t.Errorf("account_id = %v, want acct", got)
	}
	if got := nestedString(m.Fireworks, "api_key_wo"); !got.IsNull() {
		t.Errorf("api_key_wo = %v, want null", got)
	}
}

// The secret to send comes from config, the only place a write-only value is.
func TestModelProviderConfigInputReadsTheSecretFromConfig(t *testing.T) {
	ctx := context.Background()
	attrs := resourceAttrTypes(t)
	config := ModelProviderResourceModel{ModelProviderBaseModel: nullBlocks(t, attrs)}
	config.OpenAICompat = types.ObjectValueMust(objectAttrTypes(t, attrs["openai_compatible"], "openai_compatible"), map[string]attr.Value{
		"url":          types.StringValue("https://example.invalid/v1"),
		"api_token_wo": types.StringValue("tok"),
	})

	var diags diag.Diagnostics
	typ, input := providerConfigInput(ctx, &config, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if typ != pgql.ModelProviderTypeOpenAiCompatible || input.OpenAICompatible == nil {
		t.Fatalf("type %s, input %+v", typ, input)
	}
	if got := input.OpenAICompatible.ApiToken; !got.Set || got.Value != "tok" {
		t.Errorf("api token = %+v, want tok", got)
	}
}

// An update sends the provider config, and the secret with it, only when the
// version or a field Terraform can see changes. Left out, the server keeps
// the secret it has.
func TestModelProviderConfigChanged(t *testing.T) {
	attrs := resourceAttrTypes(t)
	compatType := objectAttrTypes(t, attrs["openai_compatible"], "openai_compatible")
	model := func(version int64, url string) *ModelProviderResourceModel {
		m := ModelProviderResourceModel{ModelProviderBaseModel: nullBlocks(t, attrs), CredentialsVersion: types.Int64Value(version)}
		m.OpenAICompat = types.ObjectValueMust(compatType, map[string]attr.Value{
			"url":          types.StringValue(url),
			"api_token_wo": types.StringNull(),
		})
		return &m
	}

	if providerConfigChanged(model(1, "https://a"), model(1, "https://a")) {
		t.Error("nothing changed, but the config would be sent")
	}
	if !providerConfigChanged(model(2, "https://a"), model(1, "https://a")) {
		t.Error("credentials_version changed, but the config would not be sent")
	}
	if !providerConfigChanged(model(1, "https://b"), model(1, "https://a")) {
		t.Error("url changed, but the config would not be sent")
	}

	bedrockType := objectAttrTypes(t, attrs["bedrock"], "bedrock")
	staticKeysType := objectAttrTypes(t, bedrockType["static_keys"], "static_keys")
	bedrock := func(staticKeys bool) *ModelProviderResourceModel {
		m := ModelProviderResourceModel{ModelProviderBaseModel: nullBlocks(t, attrs), CredentialsVersion: types.Int64Value(1)}
		keys := types.ObjectNull(staticKeysType)
		if staticKeys {
			keys = types.ObjectValueMust(staticKeysType, map[string]attr.Value{
				"access_key_id":        types.StringValue("AKIATEST"),
				"secret_access_key_wo": types.StringNull(),
			})
		}
		m.Bedrock = types.ObjectValueMust(bedrockType, map[string]attr.Value{
			"region":      types.StringValue("us-east-1"),
			"api_key_wo":  types.StringNull(),
			"static_keys": keys,
		})
		return &m
	}
	if !providerConfigChanged(bedrock(true), bedrock(false)) {
		t.Error("bedrock credential kind changed, but the config would not be sent")
	}
}

// The data source says whether each secret is set.
func TestModelProviderDataSourceReportsSecretSet(t *testing.T) {
	ctx := context.Background()
	m := nullBlocks(t, dataSourceAttrTypes(t))
	details := providerDetails(pgql.ModelProviderTypeGoogleGenAi,
		&pgql.ModelProviderDetailsProviderConfigModelProviderConfigGoogleGenAI{
			Vertex: &pgql.ModelProviderDetailsProviderConfigModelProviderConfigGoogleGenAIVertex{
				ServiceAccountKeySet: true,
				ProjectID:            "proj",
				Location:             "us-central1",
			},
		})

	if diags := m.MapFromGraphQL(ctx, details, secretIsSet); diags.HasError() {
		t.Fatal(diags)
	}
	vertex, ok := m.GoogleGenAI.Attributes()["vertex"].(types.Object)
	if !ok || vertex.IsNull() {
		t.Fatalf("vertex = %v", m.GoogleGenAI)
	}
	if got := vertex.Attributes()["service_account_key_set"]; !got.Equal(types.BoolValue(true)) {
		t.Errorf("service_account_key_set = %v, want true", got)
	}
	if got := vertex.Attributes()["project_id"]; !got.Equal(types.StringValue("proj")) {
		t.Errorf("project_id = %v, want proj", got)
	}
}

// Read from the data source, a Cerebras provider says its key is set, and no
// other provider block is filled in.
func TestModelProviderDataSourceCerebras(t *testing.T) {
	ctx := context.Background()
	m := nullBlocks(t, dataSourceAttrTypes(t))
	details := providerDetails(pgql.ModelProviderTypeCerebras,
		&pgql.ModelProviderDetailsProviderConfigModelProviderConfigCerebras{ApiKeySet: true})

	if diags := m.MapFromGraphQL(ctx, details, secretIsSet); diags.HasError() {
		t.Fatal(diags)
	}
	if got := m.Cerebras.Attributes()["api_key_set"]; !got.Equal(types.BoolValue(true)) {
		t.Errorf("api_key_set = %v, want true", got)
	}
	if !m.OpenAI.IsNull() || !m.Fireworks.IsNull() {
		t.Errorf("other blocks are set: openai %v, fireworks %v", m.OpenAI, m.Fireworks)
	}
}

// bedrockConfig is a config with the bedrock block set. An empty string leaves
// the credential it names out.
func bedrockConfig(t *testing.T, apiKey, accessKeyID, secretAccessKey string) ModelProviderResourceModel {
	t.Helper()
	attrs := resourceAttrTypes(t)
	bedrockType := objectAttrTypes(t, attrs["bedrock"], "bedrock")
	staticKeysType := objectAttrTypes(t, bedrockType["static_keys"], "static_keys")
	apiKeyValue := types.StringNull()
	if apiKey != "" {
		apiKeyValue = types.StringValue(apiKey)
	}
	staticKeys := types.ObjectNull(staticKeysType)
	if accessKeyID != "" {
		staticKeys = types.ObjectValueMust(staticKeysType, map[string]attr.Value{
			"access_key_id":        types.StringValue(accessKeyID),
			"secret_access_key_wo": types.StringValue(secretAccessKey),
		})
	}
	config := ModelProviderResourceModel{ModelProviderBaseModel: nullBlocks(t, attrs)}
	config.Bedrock = types.ObjectValueMust(bedrockType, map[string]attr.Value{
		"region":      types.StringValue("us-east-2"),
		"api_key_wo":  apiKeyValue,
		"static_keys": staticKeys,
	})
	return config
}

// With an API key, the server stores no static keys, so the block reads back
// with the region and a null static_keys.
func TestModelProviderBedrockAPIKeyReadsBack(t *testing.T) {
	ctx := context.Background()
	m := nullBlocks(t, resourceAttrTypes(t))
	details := providerDetails(pgql.ModelProviderTypeBedrock,
		&pgql.ModelProviderDetailsProviderConfigModelProviderConfigBedrock{
			Region: "us-east-2",
			ApiKey: &pgql.ModelProviderDetailsProviderConfigModelProviderConfigBedrockApiKey{ApiKeySet: true},
		})

	if diags := m.MapFromGraphQL(ctx, details, writeOnlySecret); diags.HasError() {
		t.Fatal(diags)
	}
	if got := nestedString(m.Bedrock, "region"); got.ValueString() != "us-east-2" {
		t.Errorf("region = %v, want us-east-2", got)
	}
	if got := m.Bedrock.Attributes()["api_key_wo"]; !got.IsNull() {
		t.Errorf("api_key_wo = %v, want null", got)
	}
	if got := m.Bedrock.Attributes()["static_keys"]; !got.IsNull() {
		t.Errorf("static_keys = %v, want null", got)
	}
	if !m.Anthropic.IsNull() || !m.OpenAI.IsNull() || !m.GoogleGenAI.IsNull() || !m.OpenAICompat.IsNull() ||
		!m.Fireworks.IsNull() || !m.Cerebras.IsNull() {
		t.Error("another provider block is set")
	}
}

func TestModelProviderBedrockStaticKeysReadBack(t *testing.T) {
	ctx := context.Background()
	m := nullBlocks(t, resourceAttrTypes(t))
	details := providerDetails(pgql.ModelProviderTypeBedrock,
		&pgql.ModelProviderDetailsProviderConfigModelProviderConfigBedrock{
			Region: "us-east-2",
			StaticKeys: &pgql.ModelProviderDetailsProviderConfigModelProviderConfigBedrockStaticKeys{
				AccessKeyID:        "AKIATEST",
				SecretAccessKeySet: true,
			},
		})

	if diags := m.MapFromGraphQL(ctx, details, writeOnlySecret); diags.HasError() {
		t.Fatal(diags)
	}
	if got := nestedString(m.Bedrock, "static_keys", "access_key_id"); got.ValueString() != "AKIATEST" {
		t.Errorf("access_key_id = %v, want AKIATEST", got)
	}
	if got := nestedString(m.Bedrock, "static_keys", "secret_access_key_wo"); !got.IsNull() {
		t.Errorf("secret_access_key_wo = %v, want null", got)
	}
	if got := m.Bedrock.Attributes()["api_key_wo"]; !got.IsNull() {
		t.Errorf("api_key_wo = %v, want null", got)
	}
}

// The data source says which of the two credentials is set.
func TestModelProviderDataSourceBedrockReportsWhichSecretIsSet(t *testing.T) {
	ctx := context.Background()

	m := nullBlocks(t, dataSourceAttrTypes(t))
	staticKeys := providerDetails(pgql.ModelProviderTypeBedrock,
		&pgql.ModelProviderDetailsProviderConfigModelProviderConfigBedrock{
			Region: "us-east-2",
			StaticKeys: &pgql.ModelProviderDetailsProviderConfigModelProviderConfigBedrockStaticKeys{
				AccessKeyID:        "AKIATEST",
				SecretAccessKeySet: true,
			},
		})
	if diags := m.MapFromGraphQL(ctx, staticKeys, secretIsSet); diags.HasError() {
		t.Fatal(diags)
	}
	if got := m.Bedrock.Attributes()["api_key_set"]; !got.Equal(types.BoolValue(false)) {
		t.Errorf("api_key_set = %v, want false", got)
	}
	keys, ok := m.Bedrock.Attributes()["static_keys"].(types.Object)
	if !ok || keys.IsNull() {
		t.Fatalf("static_keys = %v", m.Bedrock.Attributes()["static_keys"])
	}
	if got := keys.Attributes()["secret_access_key_set"]; !got.Equal(types.BoolValue(true)) {
		t.Errorf("secret_access_key_set = %v, want true", got)
	}

	m = nullBlocks(t, dataSourceAttrTypes(t))
	apiKey := providerDetails(pgql.ModelProviderTypeBedrock,
		&pgql.ModelProviderDetailsProviderConfigModelProviderConfigBedrock{
			Region: "us-east-2",
			ApiKey: &pgql.ModelProviderDetailsProviderConfigModelProviderConfigBedrockApiKey{ApiKeySet: true},
		})
	if diags := m.MapFromGraphQL(ctx, apiKey, secretIsSet); diags.HasError() {
		t.Fatal(diags)
	}
	if got := m.Bedrock.Attributes()["api_key_set"]; !got.Equal(types.BoolValue(true)) {
		t.Errorf("api_key_set = %v, want true", got)
	}
	if got := m.Bedrock.Attributes()["static_keys"]; !got.IsNull() {
		t.Errorf("static_keys = %v, want null", got)
	}
}

// The secrets to send come from config, the only place a write-only value is.
func TestModelProviderBedrockInputReadsTheSecretsFromConfig(t *testing.T) {
	ctx := context.Background()

	var diags diag.Diagnostics
	config := bedrockConfig(t, "k", "", "")
	typ, input := providerConfigInput(ctx, &config, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if typ != pgql.ModelProviderTypeBedrock || input.Bedrock == nil {
		t.Fatalf("type %s, input %+v", typ, input)
	}
	if input.Bedrock.Region != "us-east-2" {
		t.Errorf("region = %q, want us-east-2", input.Bedrock.Region)
	}
	if input.Bedrock.ApiKey == nil || input.Bedrock.ApiKey.ApiKey != "k" {
		t.Errorf("api key = %+v, want k", input.Bedrock.ApiKey)
	}
	if input.Bedrock.StaticKeys != nil {
		t.Errorf("static keys = %+v, want nil", input.Bedrock.StaticKeys)
	}

	config = bedrockConfig(t, "", "AKIATEST", "secret")
	_, input = providerConfigInput(ctx, &config, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	keys := input.Bedrock.StaticKeys
	if keys == nil || keys.AccessKeyID != "AKIATEST" || keys.SecretAccessKey != "secret" {
		t.Errorf("static keys = %+v, want AKIATEST and secret", keys)
	}
	if input.Bedrock.ApiKey != nil {
		t.Errorf("api key = %+v, want nil", input.Bedrock.ApiKey)
	}
}

func TestModelProviderBedrockInputRefusesBothCredentials(t *testing.T) {
	var diags diag.Diagnostics
	config := bedrockConfig(t, "k", "AKIATEST", "secret")
	_, input := providerConfigInput(context.Background(), &config, &diags)
	if input != nil {
		t.Errorf("input = %+v, want nil", input)
	}
	if !diags.HasError() || diags.Errors()[0].Summary() != "Invalid bedrock config" || diags.Errors()[0].Detail() != "Only one of api_key_wo or static_keys should be set, not both." {
		t.Errorf("diags = %v, want Invalid bedrock config: Only one of api_key_wo or static_keys should be set, not both.", diags)
	}
}

func TestModelProviderBedrockInputRefusesNoCredential(t *testing.T) {
	var diags diag.Diagnostics
	config := bedrockConfig(t, "", "", "")
	_, input := providerConfigInput(context.Background(), &config, &diags)
	if input != nil {
		t.Errorf("input = %+v, want nil", input)
	}
	if !diags.HasError() || diags.Errors()[0].Summary() != "Invalid bedrock config" || diags.Errors()[0].Detail() != "Either api_key_wo or static_keys must be provided." {
		t.Errorf("diags = %v, want Invalid bedrock config: Either api_key_wo or static_keys must be provided.", diags)
	}
}

// The schema refuses a bedrock block that holds both credentials or neither.
func TestModelProviderBedrockSchemaRequiresExactlyOneCredential(t *testing.T) {
	ctx := context.Background()
	var resp resource.SchemaResponse
	NewModelProviderResource().Schema(ctx, resource.SchemaRequest{}, &resp)
	bedrock, ok := resp.Schema.Attributes["bedrock"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("bedrock is not a nested attribute")
	}
	staticKeys, ok := bedrock.Attributes["static_keys"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("static_keys is not a nested attribute")
	}

	validate := func(config ModelProviderResourceModel) diag.Diagnostics {
		raw := tfsdk.State{Schema: resp.Schema, Raw: tftypes.NewValue(resp.Schema.Type().TerraformType(ctx), nil)}
		if diags := raw.Set(ctx, &config); diags.HasError() {
			t.Fatal(diags)
		}
		keys, ok := config.Bedrock.Attributes()["static_keys"].(types.Object)
		if !ok {
			t.Fatal("static_keys is not an object")
		}
		var out diag.Diagnostics
		for _, v := range staticKeys.Validators {
			r := validator.ObjectResponse{}
			v.ValidateObject(ctx, validator.ObjectRequest{
				Path:           path.Root("bedrock").AtName("static_keys"),
				PathExpression: path.MatchRoot("bedrock").AtName("static_keys"),
				Config:         tfsdk.Config{Schema: resp.Schema, Raw: raw.Raw},
				ConfigValue:    keys,
			}, &r)
			out.Append(r.Diagnostics...)
		}
		return out
	}

	if d := validate(bedrockConfig(t, "k", "AKIATEST", "secret")); !d.HasError() {
		t.Error("both credentials: want an error")
	}
	if d := validate(bedrockConfig(t, "", "", "")); !d.HasError() {
		t.Error("no credential: want an error")
	}
	if d := validate(bedrockConfig(t, "", "AKIATEST", "secret")); d.HasError() {
		t.Errorf("static keys only: %v", d)
	}
	if d := validate(bedrockConfig(t, "k", "", "")); d.HasError() {
		t.Errorf("API key only: %v", d)
	}
}
