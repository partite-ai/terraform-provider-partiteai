package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// MeshBaseModel contains the common fields for mesh resources and data sources.
type MeshBaseModel struct {
	Id             types.String      `tfsdk:"id"`
	Name           types.String      `tfsdk:"name"`
	Description    types.String      `tfsdk:"description"`
	Tags           types.List        `tfsdk:"tags"`
	CreatedAt      timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt      timetypes.RFC3339 `tfsdk:"updated_at"`
	ModelSetId     types.String      `tfsdk:"model_set_id"`
	DraftVersionId types.String      `tfsdk:"draft_version_id"`
	MemoryStoreId  types.String      `tfsdk:"memory_store_id"`
	BoundaryIds    types.Set         `tfsdk:"boundary_ids"`
}

// MapFromGraphQL maps a GraphQL MeshDetails response to the MeshBaseModel.
func (m *MeshBaseModel) MapFromGraphQL(ctx context.Context, meshDetails pgql.MeshDetails) diag.Diagnostics {
	m.MemoryStoreId = types.StringNull()
	if meshDetails.MemoryStore != nil {
		m.MemoryStoreId = types.StringValue(meshDetails.MemoryStore.Id)
	}
	return setFields(
		ctx,
		&m.Id, meshDetails.Id,
		&m.Name, meshDetails.Name,
		&m.Description, meshDetails.Description,
		&m.Tags, meshDetails.Tags,
		&m.CreatedAt, meshDetails.CreatedAt,
		&m.UpdatedAt, meshDetails.UpdatedAt,
		&m.DraftVersionId, meshDetails.DraftVersion.Id,
		&m.ModelSetId, meshDetails.DraftVersion.ModelSet.Id,
		&m.BoundaryIds, boundaryIDs(meshDetails.Boundaries),
	)
}
