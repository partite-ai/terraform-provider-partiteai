package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &RouteResource{}
var _ resource.ResourceWithImportState = &RouteResource{}

func NewRouteResource() resource.Resource {
	return &RouteResource{
		BaseResource: NewBaseResource("route"),
	}
}

type RouteResource struct {
	BaseResource
}

type RouteResourceModel struct {
	RouteBaseModel
}

func (r *RouteResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Splits the traffic of a mesh across its versions by weight.\n\n" +
			"* A segment without `mesh_version_id` sends its share of the traffic to the mesh's current draft. When a label freezes the draft, the segment follows the new draft.\n" +
			"* To send traffic to a frozen version, use the `id` of a `partiteai_mesh_version_label` resource.\n" +
			"* The weights must add up to 100. A segment with a weight of 0 gets no traffic.\n" +
			"* A route that a webhook destination or a schedule uses cannot be deleted.",
		Attributes: map[string]schema.Attribute{
			"id": commonSchemas.id,
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the mesh. Changing it replaces the route.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the route.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The description of the route.",
			},
			"segments": schema.SetNestedAttribute{
				Required:            true,
				MarkdownDescription: "The shares of the traffic. The weights must add up to 100.",
				Validators:          []validator.Set{routeWeightsAddUpTo100{}},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"mesh_version_id": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "The ID of the mesh version that gets this share of the traffic. If you leave it out, the share goes to the mesh's draft.",
						},
						"weight": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "The share of the traffic, as a percentage.",
							Validators:          []validator.Int64{int64validator.AtLeast(0)},
						},
					},
				},
			},
		},
	}
}

func (r *RouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RouteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "route", "")

	segments, diags := data.segmentInputs(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := pgql.CreateRoute(ctx, r.GetClient(), &pgql.NewRoute{
		MeshID:      data.MeshId.ValueString(),
		Name:        data.Name.ValueString(),
		Description: pgql.OptPtr(data.Description.ValueStringPointer()),
		Segments:    segments,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create route")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, createResp.CreateRoute)...)
	logResourceOperation(ctx, "created", "route", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RouteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "route", data.Id.ValueString())

	routeResp, err := pgql.GetRoute(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "route", data.Id.ValueString())
		return
	}
	if routeResp.Route == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *routeResp.Route)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RouteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "route", data.Id.ValueString())

	segments, diags := data.segmentInputs(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := pgql.ModifyRoute(ctx, r.GetClient(), data.Id.ValueString(), &pgql.UpdateRoute{
		Name:        pgql.Opt(data.Name.ValueString()),
		Description: pgql.OptNil(data.Description.ValueStringPointer()),
		Segments:    pgql.Opt(segments),
	})
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "route", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, updateResp.UpdateRoute)...)
	logResourceOperation(ctx, "updated", "route", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RouteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "route", data.Id.ValueString())

	if _, err := pgql.DeleteRoute(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "route", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "route", data.Id.ValueString())
}
