package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// EmbeddingProfileBaseModel is what an embedding profile resource and data
// source share.
type EmbeddingProfileBaseModel struct {
	Id              types.String      `tfsdk:"id"`
	ModelProviderId types.String      `tfsdk:"model_provider_id"`
	Name            types.String      `tfsdk:"name"`
	Description     types.String      `tfsdk:"description"`
	Dimensions      types.Int64       `tfsdk:"dimensions"`
	OpenAI          types.Object      `tfsdk:"openai"`
	Google          types.Object      `tfsdk:"google"`
	CreatedAt       timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt       timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL reads one embedding profile into the model.
func (m *EmbeddingProfileBaseModel) MapFromGraphQL(
	ctx context.Context,
	details pgql.EmbeddingProfileDetails,
) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, details.Id,
		&m.Name, details.Name,
		&m.Description, details.Description,
		&m.CreatedAt, details.CreatedAt,
		&m.UpdatedAt, details.UpdatedAt,
	)
	m.ModelProviderId = types.StringValue(details.Provider.Id)
	m.Dimensions = types.Int64Value(int64(details.Dimensions))

	m.OpenAI = types.ObjectNull(embeddingProfileOpenAIAttributes)
	m.Google = types.ObjectNull(embeddingProfileGoogleAttributes)

	switch c := details.Configuration.(type) {
	case *pgql.EmbeddingProfileDetailsConfigurationEmbeddingProfileConfigOpenAI:
		m.OpenAI = types.ObjectValueMust(embeddingProfileOpenAIAttributes, map[string]attr.Value{
			"model":           types.StringValue(c.Model),
			"dimensions":      types.Int64Value(int64(c.Dimensions)),
			"send_dimensions": types.BoolValue(c.SendDimensions),
		})
	case *pgql.EmbeddingProfileDetailsConfigurationEmbeddingProfileConfigGoogle:
		m.Google = types.ObjectValueMust(embeddingProfileGoogleAttributes, map[string]attr.Value{
			"model":           types.StringValue(c.Model),
			"dimensions":      types.Int64Value(int64(c.Dimensions)),
			"send_dimensions": types.BoolValue(c.SendDimensions),
			"task_type":       types.StringPointerValue(c.TaskType),
		})
	}

	return diags
}

// embeddingProfileOpenAIAttributes and embeddingProfileGoogleAttributes are the
// settings each kind of provider takes.
var embeddingProfileOpenAIAttributes = map[string]attr.Type{
	"model":           types.StringType,
	"dimensions":      types.Int64Type,
	"send_dimensions": types.BoolType,
}

var embeddingProfileGoogleAttributes = map[string]attr.Type{
	"model":           types.StringType,
	"dimensions":      types.Int64Type,
	"send_dimensions": types.BoolType,
	"task_type":       types.StringType,
}

// embeddingProfileOpenAIBlock and embeddingProfileGoogleBlock are what a
// configuration writes under each provider.
type embeddingProfileOpenAIBlock struct {
	Model          string     `tfsdk:"model"`
	Dimensions     int64      `tfsdk:"dimensions"`
	SendDimensions types.Bool `tfsdk:"send_dimensions"`
}

type embeddingProfileGoogleBlock struct {
	Model          string       `tfsdk:"model"`
	Dimensions     int64        `tfsdk:"dimensions"`
	SendDimensions types.Bool   `tfsdk:"send_dimensions"`
	TaskType       types.String `tfsdk:"task_type"`
}
