package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AccessPointClientResource{}
var _ resource.ResourceWithImportState = &AccessPointClientResource{}

func NewAccessPointClientResource() resource.Resource {
	return &AccessPointClientResource{
		BaseResource: NewBaseResource("access_point_client"),
	}
}

// AccessPointClientResource defines the resource implementation.
type AccessPointClientResource struct {
	BaseResource
}

// AccessPointClientResourceModel describes the resource data model.
type AccessPointClientResourceModel struct {
	AccessPointClientBaseModel
}

func (r *AccessPointClientResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A client of an access point. It signs in with its `client_id` and a secret, made with `partiteai_access_point_client_secret`, and gets a token from the access point.",
		Attributes: map[string]schema.Attribute{
			"id":   commonSchemas.id,
			"name": commonSchemas.name,
			"access_point_id": schema.StringAttribute{
				MarkdownDescription: "ID of the access point the client signs in to. Changing it replaces the client.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "What the client signs in as",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"claims": schema.StringAttribute{
				MarkdownDescription: "A JSON object copied into every token the client gets. It may not set `iss`, `sub`, `aud`, `exp`, `nbf`, `iat` or `jti`.",
				CustomType:          jsontypes.NormalizedType{},
				Optional:            true,
			},
			"redirect_uris": schema.SetAttribute{
				MarkdownDescription: "Where the access point's authorization endpoint may send the client back to with a code, for an MCP client that only knows the authorization code flow. Each is an https URL, or an http URL to `localhost` or a loopback address, which matches on any port. A client with none can't use the authorization code flow.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *AccessPointClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccessPointClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "access_point_client", "")

	input := pgql.NewAccessPointClient{
		AccessPointID: data.AccessPointId.ValueString(),
		Name:          data.Name.ValueString(),
	}
	if !data.Claims.IsNull() {
		input.Claims = json.RawMessage(data.Claims.ValueString())
	}
	resp.Diagnostics.Append(data.RedirectUris.ElementsAs(ctx, &input.RedirectUris, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createResp, err := pgql.CreateAccessPointClient(ctx, r.GetClient(), &input)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create access point client")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, createResp.CreateAccessPointClient)...)

	logResourceOperation(ctx, "created", "access_point_client", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPointClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccessPointClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "access_point_client", data.Id.ValueString())

	getResp, err := pgql.GetAccessPointClient(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "access_point_client", data.Id.ValueString())
		return
	}
	if getResp.AccessPointClient == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.AccessPointClient)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPointClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData AccessPointClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "access_point_client", planData.Id.ValueString())

	var update pgql.UpdateAccessPointClient
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Claims.Equal(stateData.Claims) {
		if planData.Claims.IsNull() {
			update.Claims = pgql.OptNil[json.RawMessage](nil)
		} else {
			update.Claims = pgql.Opt(json.RawMessage(planData.Claims.ValueString()))
		}
	}

	if !planData.RedirectUris.Equal(stateData.RedirectUris) {
		redirectURIs := []string{}
		resp.Diagnostics.Append(planData.RedirectUris.ElementsAs(ctx, &redirectURIs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		update.RedirectUris = pgql.Opt(redirectURIs)
	}

	updateResp, err := pgql.ModifyAccessPointClient(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "access_point_client", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, updateResp.UpdateAccessPointClient)...)

	logResourceOperation(ctx, "updated", "access_point_client", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *AccessPointClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccessPointClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "access_point_client", data.Id.ValueString())

	_, err := pgql.DeleteAccessPointClient(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "access_point_client", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "access_point_client", data.Id.ValueString())
}
