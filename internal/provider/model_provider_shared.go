package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// ModelProviderBaseModel contains the common fields for model provider resources and data sources.
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

// MapFromGraphQL maps a GraphQL ModelProviderDetails response to the ModelProviderBaseModel.
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
		c, ok := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigAnthropic)
		if !ok {
			diags.AddError("Internal Error", "Expected Anthropic provider config type")
			return diags
		}
		anthropicAttrs := m.Anthropic.AttributeTypes(ctx)
		var block = map[string]attr.Value{
			"api_key": types.StringValue(c.ApiKey),
		}
		m.Anthropic = types.ObjectValueMust(anthropicAttrs, block)
	case pgql.ModelProviderTypeOpenAi:
		c, ok := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAI)
		if !ok {
			diags.AddError("Internal Error", "Expected OpenAI provider config type")
			return diags
		}
		openaiAttrs := m.OpenAI.AttributeTypes(ctx)
		var block = map[string]attr.Value{
			"api_key": types.StringValue(c.ApiKey),
		}
		m.OpenAI = types.ObjectValueMust(openaiAttrs, block)
	case pgql.ModelProviderTypeGoogleGenAi:
		c, ok := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigGoogleGenAI)
		if !ok {
			diags.AddError("Internal Error", "Expected GoogleGenAI provider config type")
			return diags
		}
		googleGenAIAttrs := m.GoogleGenAI.AttributeTypes(ctx)
		geminiType, ok := googleGenAIAttrs["gemini"].(types.ObjectType)
		if !ok {
			diags.AddError("Internal Error", "Expected gemini to be ObjectType")
			return diags
		}
		vertexType, ok := googleGenAIAttrs["vertex"].(types.ObjectType)
		if !ok {
			diags.AddError("Internal Error", "Expected vertex to be ObjectType")
			return diags
		}

		block := map[string]attr.Value{}
		// Set only one of gemini or vertex
		if c.Gemini != nil {
			geminiBlock := map[string]attr.Value{
				"api_key": types.StringValue(c.Gemini.ApiKey),
			}
			block["gemini"] = types.ObjectValueMust(geminiType.AttributeTypes(), geminiBlock)
			block["vertex"] = types.ObjectNull(vertexType.AttributeTypes())
		} else if c.Vertex != nil {
			vertexBlock := map[string]attr.Value{
				"service_account_key": types.StringValue(c.Vertex.ServiceAccountKey),
				"project_id":          types.StringValue(c.Vertex.ProjectID),
				"location":            types.StringValue(c.Vertex.Location),
			}
			block["vertex"] = types.ObjectValueMust(vertexType.AttributeTypes(), vertexBlock)
			block["gemini"] = types.ObjectNull(geminiType.AttributeTypes())
		} else {
			block["gemini"] = types.ObjectNull(geminiType.AttributeTypes())
			block["vertex"] = types.ObjectNull(vertexType.AttributeTypes())
		}
		m.GoogleGenAI = types.ObjectValueMust(googleGenAIAttrs, block)
	case pgql.ModelProviderTypeOpenAiCompatible:
		c, ok := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAICompatible)
		if !ok {
			diags.AddError("Internal Error", "Expected OpenAICompatible provider config type")
			return diags
		}
		openAICompatAttrs := m.OpenAICompat.AttributeTypes(ctx)
		block := map[string]attr.Value{
			"url": types.StringValue(c.Url),
		}
		if c.ApiToken != nil {
			block["api_token"] = types.StringValue(*c.ApiToken)
		} else {
			block["api_token"] = types.StringNull()
		}
		m.OpenAICompat = types.ObjectValueMust(openAICompatAttrs, block)
	}

	return diags
}
