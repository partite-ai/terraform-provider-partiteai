package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// ModelSetBaseModel contains the common fields for model set resources and data sources.
type ModelSetBaseModel struct {
	Id                  types.String      `tfsdk:"id"`
	Name                types.String      `tfsdk:"name"`
	Description         types.String      `tfsdk:"description"`
	Tags                types.List        `tfsdk:"tags"`
	DefaultModelProfile types.Object      `tfsdk:"default_model_profile"`
	Rules               types.List        `tfsdk:"rules"`
	CreatedAt           timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt           timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps a GraphQL ModelSetDetails response to the ModelSetBaseModel.
func (m *ModelSetBaseModel) MapFromGraphQL(ctx context.Context, details pgql.ModelSetDetails) diag.Diagnostics {
	// Use setFields for standard fields
	diags := setFields(
		ctx,
		&m.Id, details.Id,
		&m.Name, details.Name,
		&m.Description, details.Description,
		&m.CreatedAt, details.CreatedAt,
		&m.UpdatedAt, details.UpdatedAt,
		&m.Tags, details.Tags,
		&m.Rules, newStructSliceMappable(details.Rules, modelSetRuleStruct{}),
	)

	var defaultModelProfile modelProfileRefStruct
	diags = append(diags, setFields(
		ctx,
		&defaultModelProfile.ModelProfileId, details.DefaultModelProfile.Id,
		&defaultModelProfile.ModelProviderId, details.DefaultModelProfile.Provider.Id,
		&defaultModelProfile.ModelProfileName, details.DefaultModelProfile.Name,
	)...)

	diags = append(diags, setFields(
		ctx,
		&m.DefaultModelProfile, defaultModelProfile,
	)...)
	return diags
}
