package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// TaskDefinitionBaseModel contains all the common fields for both resource and data source.
type TaskDefinitionBaseModel struct {
	Id                types.String         `tfsdk:"id"`
	Name              types.String         `tfsdk:"name"`
	Description       types.String         `tfsdk:"description"`
	InputMessageType  jsontypes.Normalized `tfsdk:"input_message_type"`
	OutputMessageType jsontypes.Normalized `tfsdk:"output_message_type"`
	CreatedAt         timetypes.RFC3339    `tfsdk:"created_at"`
	UpdatedAt         timetypes.RFC3339    `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *TaskDefinitionBaseModel) MapFromGraphQL(ctx context.Context, td *pgql.TaskDefinitionDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Append(setFields(
		ctx,
		&m.Id, td.Id,
		&m.Name, td.Name,
		&m.Description, td.Description,
		&m.InputMessageType, td.InputMessageType,
		&m.OutputMessageType, td.OutputMessageType,
		&m.CreatedAt, td.CreatedAt,
		&m.UpdatedAt, td.UpdatedAt,
	)...)
	return diags
}
