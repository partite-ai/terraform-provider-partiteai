package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Workspace scope literals used in schema validators.
const (
	workspaceScopeGlobal       = "GLOBAL"
	workspaceScopeConversation = "CONVERSATION"
)

// WorkspaceBaseModel contains all the common fields for both resource and data source.
type WorkspaceBaseModel struct {
	Id        types.String      `tfsdk:"id"`
	Name      types.String      `tfsdk:"name"`
	Scope     types.String      `tfsdk:"scope"`
	CreatedAt timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *WorkspaceBaseModel) MapFromGraphQL(ctx context.Context, ws *pgql.WorkspaceDetails) diag.Diagnostics {
	return setFields(
		ctx,
		&m.Id, ws.Id,
		&m.Name, ws.Name,
		&m.Scope, string(ws.Scope),
		&m.CreatedAt, ws.CreatedAt,
		&m.UpdatedAt, ws.UpdatedAt,
	)
}
