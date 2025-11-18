package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// WebhookDestinationBaseModel contains all the common fields for both resource and data source.
type WebhookDestinationBaseModel struct {
	Id               types.String      `tfsdk:"id"`
	Name             types.String      `tfsdk:"name"`
	Description      types.String      `tfsdk:"description"`
	ExternalId       types.String      `tfsdk:"external_id"`
	InvokeUrl        types.String      `tfsdk:"invoke_url"`
	MeshId           types.String      `tfsdk:"mesh_id"`
	MeshVersionLabel types.String      `tfsdk:"mesh_version_label"`
	InputName        types.String      `tfsdk:"input_name"`
	CreatedAt        timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt        timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *WebhookDestinationBaseModel) MapFromGraphQL(ctx context.Context, webhookDetails *pgql.WebhookDestinationDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Append(setFields(
		ctx,
		&m.Id, webhookDetails.Id,
		&m.Name, webhookDetails.Name,
		&m.Description, webhookDetails.Description,
		&m.ExternalId, webhookDetails.ExternalID,
		&m.MeshId, webhookDetails.Mesh.Id,
		&m.MeshVersionLabel, webhookDetails.MeshVersionLabel,
		&m.InputName, webhookDetails.InputName,
		&m.CreatedAt, webhookDetails.CreatedAt,
		&m.UpdatedAt, webhookDetails.UpdatedAt,
	)...)
	return diags
}
