package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// ModelProfileBaseModel contains the common fields for model profile resources and data sources.
type ModelProfileBaseModel struct {
	Id              types.String      `tfsdk:"id"`
	ModelProviderId types.String      `tfsdk:"model_provider_id"`
	Name            types.String      `tfsdk:"name"`
	Description     types.String      `tfsdk:"description"`
	Tags            types.List        `tfsdk:"tags"`
	CreatedAt       timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt       timetypes.RFC3339 `tfsdk:"updated_at"`
	Anthropic       types.Object      `tfsdk:"anthropic"`
	OpenAI          types.Object      `tfsdk:"openai"`
	Google          types.Object      `tfsdk:"google"`
}

// MapFromGraphQL maps a GraphQL ModelProfileDetails response to the ModelProfileBaseModel.
func (m *ModelProfileBaseModel) MapFromGraphQL(ctx context.Context, details pgql.ModelProfileDetails) diag.Diagnostics {
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
	m.Google = types.ObjectNull(m.Google.AttributeTypes(ctx))

	// Set the correct block based on config type
	switch c := details.Configuration.(type) {
	case *pgql.ModelProfileDetailsConfigurationModelProfileConfigAnthropic:
		block := map[string]attr.Value{
			"model":                  types.StringValue(c.Model),
			"max_tokens":             types.Int64PointerValue(int64Ptr(c.MaxTokens)),
			"temperature":            types.Float64PointerValue(c.Temperature),
			"top_p":                  types.Float64PointerValue(c.TopP),
			"top_k":                  types.Int64PointerValue(int64Ptr(c.TopK)),
			"thinking_budget_tokens": types.Int64PointerValue(int64Ptr(c.ThinkingBudgetTokens)),
		}
		m.Anthropic = types.ObjectValueMust(m.Anthropic.AttributeTypes(ctx), block)
	case *pgql.ModelProfileDetailsConfigurationModelProfileConfigOpenAI:
		block := map[string]attr.Value{
			"model":       types.StringValue(c.Model),
			"max_tokens":  types.Int64PointerValue(int64Ptr(c.MaxTokens)),
			"temperature": types.Float64PointerValue(c.Temperature),
			"top_p":       types.Float64PointerValue(c.TopP),
			"reasoning_effort": func() types.String {
				if c.ReasoningEffort != nil {
					return types.StringValue(string(*c.ReasoningEffort))
				}
				return types.StringNull()
			}(),
		}
		m.OpenAI = types.ObjectValueMust(m.OpenAI.AttributeTypes(ctx), block)
	case *pgql.ModelProfileDetailsConfigurationModelProfileConfigGoogle:
		block := map[string]attr.Value{
			"model":                  types.StringValue(c.Model),
			"max_tokens":             types.Int64PointerValue(int64Ptr(c.MaxTokens)),
			"temperature":            types.Float64PointerValue(c.Temperature),
			"top_p":                  types.Float64PointerValue(c.TopP),
			"top_k":                  types.Int64PointerValue(int64Ptr(c.TopK)),
			"thinking_budget_tokens": types.Int64PointerValue(int64Ptr(c.ThinkingBudgetTokens)),
		}
		m.Google = types.ObjectValueMust(m.Google.AttributeTypes(ctx), block)
	}

	return diags
}
