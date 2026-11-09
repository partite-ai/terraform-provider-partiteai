package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

type RouteBaseModel struct {
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

func (m *RouteBaseModel) MapFromGraphQL(ctx context.Context, route pgql.RouteDetails) diag.Diagnostics {
	return setFields(
		ctx,
		&m.Id, route.Id,
		&m.MeshId, route.MeshID,
		&m.Name, route.Name,
		&m.Description, route.Description,
		&m.Segments, newStructSliceMappable(route.Segments, routeSegmentStruct{}),
	)
}

func (m *RouteBaseModel) segmentInputs(ctx context.Context) ([]pgql.NewRouteSegment, diag.Diagnostics) {
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

// routeWeightsAddUpTo100 finds at plan time the weights that the engine
// refuses on apply. While a weight is unknown, it cannot check the sum.
type routeWeightsAddUpTo100 struct{}

var _ validator.Set = routeWeightsAddUpTo100{}

func (routeWeightsAddUpTo100) Description(context.Context) string {
	return "the segment weights add up to 100"
}

func (v routeWeightsAddUpTo100) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (routeWeightsAddUpTo100) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var segments []routeSegmentStruct
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &segments, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var total int64
	for _, s := range segments {
		if s.Weight.IsUnknown() {
			return
		}
		total += s.Weight.ValueInt64()
	}
	if total != 100 {
		resp.Diagnostics.AddAttributeError(req.Path, "Route weights do not add up to 100",
			fmt.Sprintf("The segment weights add up to %d. They must add up to 100.", total))
	}
}
