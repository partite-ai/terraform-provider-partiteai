package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
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
	Configuration   types.Object      `tfsdk:"configuration"`
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
		&m.ModelProviderId, details.Provider.Id,
	)

	// One configuration for every provider. Which settings a model accepts is
	// reported by the provider rather than fixed by the vendor, so there is nothing
	// to discriminate on here.
	config := details.Configuration
	m.Configuration = types.ObjectValueMust(modelProfileConfigAttributes, map[string]attr.Value{
		"model":                  types.StringValue(config.Model),
		"max_tokens":             types.Int64PointerValue(int64Ptr(config.MaxTokens)),
		"temperature":            types.Float64PointerValue(config.Temperature),
		"top_p":                  types.Float64PointerValue(config.TopP),
		"top_k":                  types.Int64PointerValue(int64Ptr(config.TopK)),
		"thinking_budget_tokens": types.Int64PointerValue(int64Ptr(config.ThinkingBudgetTokens)),
		"reasoning_effort":       enumValue(config.ReasoningEffort),
		"thinking_type":          enumValue(config.ThinkingType),
		"thinking_display":       enumValue(config.ThinkingDisplay),
	})

	return diags
}

// enumValue is a GraphQL enum as a Terraform string, null when unset.
func enumValue[E ~string](v *E) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(string(*v))
}

// modelProfileConfigAttributes is the shape of the configuration block.
var modelProfileConfigAttributes = map[string]attr.Type{
	"model":                  types.StringType,
	"max_tokens":             types.Int64Type,
	"temperature":            types.Float64Type,
	"top_p":                  types.Float64Type,
	"top_k":                  types.Int64Type,
	"thinking_budget_tokens": types.Int64Type,
	"reasoning_effort":       types.StringType,
	"thinking_type":          types.StringType,
	"thinking_display":       types.StringType,
}

// modelProfileConfigBlock is what a configuration writes.
type modelProfileConfigBlock struct {
	Model                string        `tfsdk:"model"`
	MaxTokens            types.Int64   `tfsdk:"max_tokens"`
	Temperature          types.Float64 `tfsdk:"temperature"`
	TopP                 types.Float64 `tfsdk:"top_p"`
	TopK                 types.Int64   `tfsdk:"top_k"`
	ThinkingBudgetTokens types.Int64   `tfsdk:"thinking_budget_tokens"`
	ReasoningEffort      types.String  `tfsdk:"reasoning_effort"`
	ThinkingType         types.String  `tfsdk:"thinking_type"`
	ThinkingDisplay      types.String  `tfsdk:"thinking_display"`
}

// modelProfileConfig reads the configuration block into the API's input.
func modelProfileConfig(ctx context.Context, configuration types.Object, diags *diag.Diagnostics) (*pgql.ModelProfileConfigInput, bool) {
	var block modelProfileConfigBlock
	if d := tfsdk.ValueAs(ctx, configuration, &block); d.HasError() {
		diags.Append(d...)
		return nil, false
	}

	return &pgql.ModelProfileConfigInput{
		Model:                block.Model,
		MaxTokens:            pgql.OptPtr(block.MaxTokens.ValueInt64Pointer()),
		Temperature:          pgql.OptPtr(block.Temperature.ValueFloat64Pointer()),
		TopP:                 pgql.OptPtr(block.TopP.ValueFloat64Pointer()),
		TopK:                 pgql.OptPtr(block.TopK.ValueInt64Pointer()),
		ThinkingBudgetTokens: pgql.OptPtr(block.ThinkingBudgetTokens.ValueInt64Pointer()),
		ReasoningEffort:      pgql.OptPtr(block.ReasoningEffort.ValueStringPointer()),
		ThinkingType:         pgql.OptPtr(block.ThinkingType.ValueStringPointer()),
		ThinkingDisplay:      pgql.OptPtr(block.ThinkingDisplay.ValueStringPointer()),
	}, true
}
