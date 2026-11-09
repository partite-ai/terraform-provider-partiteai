package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ datasource.DataSource = &RouteDataSource{}

func NewRouteDataSource() datasource.DataSource {
	return &RouteDataSource{
		BaseDataSource: NewBaseDataSource("route"),
	}
}

type RouteDataSource struct {
	BaseDataSource
}

func (d *RouteDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds a route by its ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the route.",
			},
			"mesh_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the mesh.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the route.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The description of the route.",
			},
			"segments": schema.SetNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The shares of the traffic.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"mesh_version_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the mesh version that gets this share of the traffic. If it is null, the share goes to the mesh's draft.",
						},
						"weight": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The share of the traffic, as a percentage.",
						},
					},
				},
			},
		},
	}
}

func (d *RouteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RouteModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "route", data.Id.ValueString())

	routeResp, err := pgql.GetRoute(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "route", data.Id.ValueString())
		return
	}
	if routeResp.Route == nil {
		resp.Diagnostics.AddError("Route not found", fmt.Sprintf("No route has the ID %s.", data.Id.ValueString()))
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *routeResp.Route)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
