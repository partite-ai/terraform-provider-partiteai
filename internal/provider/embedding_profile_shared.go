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
	Configuration   types.Object      `tfsdk:"configuration"`
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

	// One configuration for every provider. Which settings a model accepts is the
	// provider's business, so there is nothing to discriminate on here.
	config := details.Configuration
	m.Configuration = types.ObjectValueMust(embeddingProfileConfigAttributes, map[string]attr.Value{
		"model":           types.StringValue(config.Model),
		"dimensions":      types.Int64Value(int64(config.Dimensions)),
		"send_dimensions": types.BoolValue(config.SendDimensions),
		"task_type":       types.StringPointerValue(config.TaskType),
	})

	return diags
}

// embeddingProfileConfigAttributes is the shape of the configuration block.
var embeddingProfileConfigAttributes = map[string]attr.Type{
	"model":           types.StringType,
	"dimensions":      types.Int64Type,
	"send_dimensions": types.BoolType,
	"task_type":       types.StringType,
}

// embeddingProfileConfigBlock is what a configuration writes.
type embeddingProfileConfigBlock struct {
	Model          string       `tfsdk:"model"`
	Dimensions     int64        `tfsdk:"dimensions"`
	SendDimensions types.Bool   `tfsdk:"send_dimensions"`
	TaskType       types.String `tfsdk:"task_type"`
}
