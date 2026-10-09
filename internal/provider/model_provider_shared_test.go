package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

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
