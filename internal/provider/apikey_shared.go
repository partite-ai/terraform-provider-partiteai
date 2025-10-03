package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// ApiKeyBaseModel contains all the common fields for both resource and data source
type ApiKeyBaseModel struct {
	Id          types.String      `tfsdk:"id"`
	Name        types.String      `tfsdk:"name"`
	Description types.String      `tfsdk:"description"`
	Scopes      types.Set         `tfsdk:"scopes"`
	Kid         types.String      `tfsdk:"kid"`
	CreatedAt   timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt   timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model
func (m *ApiKeyBaseModel) MapFromGraphQL(ctx context.Context, apiKeyDetails *pgql.ApiKeyDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Append(setFields(
		ctx,
		&m.Id, apiKeyDetails.Id,
		&m.Name, apiKeyDetails.Name,
		&m.Description, apiKeyDetails.Description,
		&m.Kid, apiKeyDetails.Kid,
		&m.CreatedAt, apiKeyDetails.CreatedAt,
		&m.UpdatedAt, apiKeyDetails.UpdatedAt,
		&m.Scopes, apiKeyDetails.Scopes,
	)...)

	return diags
}
