package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

type MeshVersionLabelBaseModel struct {
	Id      types.String `tfsdk:"id"`
	MeshId  types.String `tfsdk:"mesh_id"`
	Label   types.String `tfsdk:"label"`
	Ordinal types.Int64  `tfsdk:"ordinal"`
}

func (m *MeshVersionLabelBaseModel) MapFromGraphQL(ctx context.Context, v pgql.MeshVersionDetails) diag.Diagnostics {
	return setFields(
		ctx,
		&m.Id, v.Id,
		&m.MeshId, v.Mesh.Id,
		&m.Label, v.Label,
		&m.Ordinal, v.Ordinal,
	)
}
