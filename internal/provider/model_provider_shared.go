package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// ModelProviderBaseModel contains the common fields for model provider resources and data sources
type ModelProviderBaseModel struct {
	Id           types.String      `tfsdk:"id"`
	Name         types.String      `tfsdk:"name"`
	Description  types.String      `tfsdk:"description"`
	Tags         types.List        `tfsdk:"tags"`
	CreatedAt    timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt    timetypes.RFC3339 `tfsdk:"updated_at"`
	Anthropic    types.Object      `tfsdk:"anthropic"`
	OpenAI       types.Object      `tfsdk:"openai"`
	GoogleGenAI  types.Object      `tfsdk:"google_genai"`
	OpenAICompat types.Object      `tfsdk:"openai_compatible"`
}

// MapFromGraphQL maps a GraphQL ModelProviderDetails response to the ModelProviderBaseModel
func (m *ModelProviderBaseModel) MapFromGraphQL(ctx context.Context, details pgql.ModelProviderDetails) diag.Diagnostics {
	// Use setFields for standard fields
	diags := setFields(
		ctx,
		&m.Id, details.Id,
		&m.Name, details.Name,
		&m.Description, details.Description,
		&m.CreatedAt, details.CreatedAt,
		&m.UpdatedAt, details.UpdatedAt,
		&m.Tags, details.Tags,
	)

	// Clear all blocks
	m.Anthropic = types.ObjectNull(m.Anthropic.AttributeTypes(ctx))
	m.OpenAI = types.ObjectNull(m.OpenAI.AttributeTypes(ctx))
	m.GoogleGenAI = types.ObjectNull(m.GoogleGenAI.AttributeTypes(ctx))
	m.OpenAICompat = types.ObjectNull(m.OpenAICompat.AttributeTypes(ctx))

	// Set the correct block based on provider type
	switch details.ProviderType {
	case pgql.ModelProviderTypeAnthropic:
		c := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigAnthropic)
		var block = map[string]attr.Value{
			"api_key": types.StringValue(c.ApiKey),
		}
		m.Anthropic = types.ObjectValueMust(m.Anthropic.AttributeTypes(ctx), block)
	case pgql.ModelProviderTypeOpenAi:
		c := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAI)
		var block = map[string]attr.Value{
			"api_key": types.StringValue(c.ApiKey),
		}
		m.OpenAI = types.ObjectValueMust(m.OpenAI.AttributeTypes(ctx), block)
	case pgql.ModelProviderTypeGoogleGenAi:
		c := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigGoogleGenAI)
		block := map[string]attr.Value{}
		// Set only one of gemini or vertex
		if c.Gemini != nil {
			geminiBlock := map[string]attr.Value{
				"api_key": types.StringValue(c.Gemini.ApiKey),
			}
			block["gemini"] = types.ObjectValueMust(m.GoogleGenAI.AttributeTypes(ctx)["gemini"].(types.ObjectType).AttributeTypes(), geminiBlock)
			block["vertex"] = types.ObjectNull(m.GoogleGenAI.AttributeTypes(ctx)["vertex"].(types.ObjectType).AttributeTypes())
		} else if c.Vertex != nil {
			vertexBlock := map[string]attr.Value{
				"service_account_key": types.StringValue(c.Vertex.ServiceAccountKey),
				"project_id":          types.StringValue(c.Vertex.ProjectID),
				"location":            types.StringValue(c.Vertex.Location),
			}
			block["vertex"] = types.ObjectValueMust(m.GoogleGenAI.AttributeTypes(ctx)["vertex"].(types.ObjectType).AttributeTypes(), vertexBlock)
			block["gemini"] = types.ObjectNull(m.GoogleGenAI.AttributeTypes(ctx)["gemini"].(types.ObjectType).AttributeTypes())
		} else {
			block["gemini"] = types.ObjectNull(m.GoogleGenAI.AttributeTypes(ctx)["gemini"].(types.ObjectType).AttributeTypes())
			block["vertex"] = types.ObjectNull(m.GoogleGenAI.AttributeTypes(ctx)["vertex"].(types.ObjectType).AttributeTypes())
		}
		m.GoogleGenAI = types.ObjectValueMust(m.GoogleGenAI.AttributeTypes(ctx), block)
	case pgql.ModelProviderTypeOpenAiCompatible:
		c := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAICompatible)
		block := map[string]attr.Value{
			"url": types.StringValue(c.Url),
		}
		if c.ApiToken != nil {
			block["api_token"] = types.StringValue(*c.ApiToken)
		} else {
			block["api_token"] = types.StringNull()
		}
		m.OpenAICompat = types.ObjectValueMust(m.OpenAICompat.AttributeTypes(ctx), block)
	}

	return diags
}
