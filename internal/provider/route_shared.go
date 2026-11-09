package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

type RouteModel struct {
	Id          types.String `tfsdk:"id"`
	MeshId      types.String `tfsdk:"mesh_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Segments    types.Set    `tfsdk:"segments"`
}

type routeSegmentStruct struct {
	MeshVersionId types.String `tfsdk:"mesh_version_id"`
	Weight        types.Int64  `tfsdk:"weight"`
}

func (s *routeSegmentStruct) FromGQL(src pgql.RouteDetailsSegmentsRouteSegment) diag.Diagnostics {
	s.MeshVersionId = types.StringPointerValue(src.MeshVersionID)
	s.Weight = types.Int64Value(int64(src.Weight))
	return nil
}

func (m *RouteModel) MapFromGraphQL(ctx context.Context, route pgql.RouteDetails) diag.Diagnostics {
	return setFields(
		ctx,
		&m.Id, route.Id,
		&m.MeshId, route.MeshID,
		&m.Name, route.Name,
		&m.Description, route.Description,
		&m.Segments, newStructSliceMappable(route.Segments, routeSegmentStruct{}),
	)
}

func (m *RouteModel) segmentInputs(ctx context.Context) ([]pgql.NewRouteSegment, diag.Diagnostics) {
	var segments []routeSegmentStruct
	diags := m.Segments.ElementsAs(ctx, &segments, false)
	inputs := make([]pgql.NewRouteSegment, len(segments))
	for i, s := range segments {
		inputs[i] = pgql.NewRouteSegment{
			MeshVersionID: pgql.OptPtr(s.MeshVersionId.ValueStringPointer()),
			Weight:        int(s.Weight.ValueInt64()),
		}
	}
	return inputs, diags
}
