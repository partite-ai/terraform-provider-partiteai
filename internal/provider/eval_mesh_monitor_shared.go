package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// EvalMeshMonitorBaseModel contains all the common fields for both resource
// and data source.
type EvalMeshMonitorBaseModel struct {
	Id         types.String      `tfsdk:"id"`
	MeshId     types.String      `tfsdk:"mesh_id"`
	RubricId   types.String      `tfsdk:"rubric_id"`
	ModelSetId types.String      `tfsdk:"model_set_id"`
	Enabled    types.Bool        `tfsdk:"enabled"`
	SampleRate types.Float64     `tfsdk:"sample_rate"`
	CreatedAt  timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt  timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *EvalMeshMonitorBaseModel) MapFromGraphQL(ctx context.Context, src *pgql.EvalMeshMonitorDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, src.Id,
		&m.MeshId, src.MeshID,
		&m.RubricId, src.RubricID,
		&m.ModelSetId, src.ModelSetID,
		&m.Enabled, src.Enabled,
		&m.CreatedAt, src.CreatedAt,
		&m.UpdatedAt, src.UpdatedAt,
	)
	m.SampleRate = types.Float64Value(src.SampleRate)
	return diags
}
