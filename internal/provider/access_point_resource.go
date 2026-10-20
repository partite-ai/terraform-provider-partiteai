package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AccessPointResource{}
var _ resource.ResourceWithImportState = &AccessPointResource{}

func NewAccessPointResource() resource.Resource {
	return &AccessPointResource{
		BaseResource: NewBaseResource("access_point"),
	}
}

// AccessPointResource defines the resource implementation.
type AccessPointResource struct {
	BaseResource
}

// AccessPointResourceModel describes the resource data model.
type AccessPointResourceModel struct {
	AccessPointBaseModel
}

var accessPointIssuerType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"issuer": schema.StringAttribute{
			MarkdownDescription: "The `iss` a token from this issuer carries",
			Required:            true,
		},
		"audience": schema.StringAttribute{
			MarkdownDescription: "Must be in a token's `aud`. Left out, it is the access point's URL.",
			Optional:            true,
			Computed:            true,
		},
		"check_audience": schema.BoolAttribute{
			MarkdownDescription: "Whether a token's `aud` is checked. Turning it off accepts tokens the issuer made for other services and ID tokens, and skips the audience check MCP clients expect.",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(true),
		},
		"jwks_url": schema.StringAttribute{
			MarkdownDescription: "Where the issuer publishes its key set. Set exactly one of `jwks_url` and `jwks`.",
			Optional:            true,
		},
		"jwks": schema.StringAttribute{
			MarkdownDescription: "A key set, as JSON, for an issuer that publishes none",
			CustomType:          jsontypes.NormalizedType{},
			Optional:            true,
		},
	},
}

func (r *AccessPointResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An access point: a way into meshes for callers who sign in with a token from a trusted issuer rather than an API key.",
		Attributes: standardSchemaAttributes(map[string]schema.Attribute{
			"url": schema.StringAttribute{
				MarkdownDescription: "Where callers reach the access point. Tokens it issues to its clients carry it as their `iss` and `aud`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"authorization_server_url": schema.StringAttribute{
				MarkdownDescription: "Advertised to MCP clients as where to log in",
				Optional:            true,
			},
			"client_token_lifetime_seconds": schema.Int64Attribute{
				MarkdownDescription: "How long a token issued to a client lasts, from 300 to 86400 seconds",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(3600),
				Validators:          []validator.Int64{int64validator.Between(300, 86400)},
			},
			"issuers": schema.ListNestedAttribute{
				MarkdownDescription: "The issuers whose tokens the access point accepts",
				NestedObject:        accessPointIssuerType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(accessPointIssuerType.Type(), nil)),
			},
			"boundary_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of the boundaries conversations created through the access point are checked against",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			},
		}),
	}
}

func (r *AccessPointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccessPointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "access_point", "")

	var tags []*pgql.Tag
	resp.Diagnostics.Append(data.Tags.ElementsAs(ctx, &tags, false)...)
	issuers, diags := accessPointIssuerInputs(ctx, data.Issuers)
	resp.Diagnostics.Append(diags...)
	boundaries, diags := boundaryAssociations(ctx, data.BoundaryIds)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := pgql.CreateAccessPoint(ctx, r.GetClient(), &pgql.NewAccessPoint{
		Name:                       data.Name.ValueString(),
		Description:                pgql.OptPtr(data.Description.ValueStringPointer()),
		Tags:                       tags,
		AuthorizationServerURL:     pgql.OptPtr(data.AuthorizationServerUrl.ValueStringPointer()),
		ClientTokenLifetimeSeconds: pgql.Opt(int(data.ClientTokenLifetimeSeconds.ValueInt64())),
		Issuers:                    issuers,
		Boundaries:                 boundaries,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create access point")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, createResp.CreateAccessPoint)...)

	logResourceOperation(ctx, "created", "access_point", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccessPointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "access_point", data.Id.ValueString())

	getResp, err := pgql.GetAccessPoint(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "access_point", data.Id.ValueString())
		return
	}
	if getResp.AccessPoint == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.AccessPoint)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData AccessPointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "access_point", planData.Id.ValueString())

	var update pgql.UpdateAccessPoint
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}
	if !planData.Tags.Equal(stateData.Tags) {
		var tags []*pgql.Tag
		resp.Diagnostics.Append(planData.Tags.ElementsAs(ctx, &tags, false)...)
		update.Tags = pgql.Opt(tags)
	}
	if !planData.AuthorizationServerUrl.Equal(stateData.AuthorizationServerUrl) {
		update.AuthorizationServerURL = pgql.OptNil(planData.AuthorizationServerUrl.ValueStringPointer())
	}
	if !planData.ClientTokenLifetimeSeconds.Equal(stateData.ClientTokenLifetimeSeconds) {
		update.ClientTokenLifetimeSeconds = pgql.Opt(int(planData.ClientTokenLifetimeSeconds.ValueInt64()))
	}
	if !planData.Issuers.Equal(stateData.Issuers) {
		issuers, diags := accessPointIssuerInputs(ctx, planData.Issuers)
		resp.Diagnostics.Append(diags...)
		update.Issuers = pgql.Opt(issuers)
	}
	if !planData.BoundaryIds.Equal(stateData.BoundaryIds) {
		boundaries, diags := boundaryAssociations(ctx, planData.BoundaryIds)
		resp.Diagnostics.Append(diags...)
		update.Boundaries = pgql.Opt(boundaries)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := pgql.ModifyAccessPoint(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "access_point", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, updateResp.UpdateAccessPoint)...)

	logResourceOperation(ctx, "updated", "access_point", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *AccessPointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccessPointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "access_point", data.Id.ValueString())

	_, err := pgql.DeleteAccessPoint(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "access_point", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "access_point", data.Id.ValueString())
}
