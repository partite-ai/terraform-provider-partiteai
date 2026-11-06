package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

type MeshVersionLabelModel struct {
	Id            types.String `tfsdk:"id"`
	MeshId        types.String `tfsdk:"mesh_id"`
	Label         types.String `tfsdk:"label"`
	MeshVersionId types.String `tfsdk:"mesh_version_id"`
	Ordinal       types.Int64  `tfsdk:"ordinal"`
}

func (m *MeshVersionLabelModel) MapFromGraphQL(v pgql.MeshVersionDetails) {
	m.Id = types.StringValue(v.Id)
	m.MeshVersionId = types.StringValue(v.Id)
	m.MeshId = types.StringValue(v.Mesh.Id)
	m.Label = types.StringPointerValue(v.Label)
	m.Ordinal = types.Int64Value(int64(v.Ordinal))
}
