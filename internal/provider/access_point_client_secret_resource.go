package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AccessPointClientSecretResource{}
var _ resource.ResourceWithImportState = &AccessPointClientSecretResource{}

func NewAccessPointClientSecretResource() resource.Resource {
	return &AccessPointClientSecretResource{
		BaseResource: NewBaseResource("access_point_client_secret"),
	}
}

// AccessPointClientSecretResource defines the resource implementation.
type AccessPointClientSecretResource struct {
	BaseResource
}

// AccessPointClientSecretResourceModel describes the resource data model.
type AccessPointClientSecretResourceModel struct {
	Id                  types.String      `tfsdk:"id"`
	AccessPointClientId types.String      `tfsdk:"access_point_client_id"`
	ExpiresAt           timetypes.RFC3339 `tfsdk:"expires_at"`
	Secret              types.String      `tfsdk:"secret"`
	LastUsedAt          timetypes.RFC3339 `tfsdk:"last_used_at"`
	CreatedAt           timetypes.RFC3339 `tfsdk:"created_at"`
}

func (r *AccessPointClientSecretResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A secret an access point client signs in with. A client can have several, so a secret can be replaced without changing the client ID: add the new one, move callers to it, then remove the old one. `create_before_destroy` does this in one apply.",
		Attributes: map[string]schema.Attribute{
			"id": commonSchemas.id,
			"access_point_client_id": schema.StringAttribute{
				MarkdownDescription: "ID of the client the secret signs in as. Changing it replaces the secret.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "When the secret stops working. Left out, it doesn't expire. Changing it replaces the secret.",
				CustomType:          timetypes.RFC3339Type{},
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "The secret. It is readable only when it is made, so an imported secret has none.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"last_used_at": schema.StringAttribute{
				MarkdownDescription: "When the secret last signed in. A secret that stopped being used is safe to remove.",
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
			},
			"created_at": commonSchemas.createdAt,
		},
	}
}

func (r *AccessPointClientSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccessPointClientSecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "access_point_client_secret", "")

	createResp, err := pgql.CreateAccessPointClientSecret(ctx, r.GetClient(), data.AccessPointClientId.ValueString(), data.ExpiresAt.ValueStringPointer())
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create access point client secret")
		return
	}

	data.Secret = types.StringValue(createResp.CreateAccessPointClientSecret.Secret)
	resp.Diagnostics.Append(data.mapFromGraphQL(ctx, createResp.CreateAccessPointClientSecret.ClientSecret)...)

	logResourceOperation(ctx, "created", "access_point_client_secret", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPointClientSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccessPointClientSecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "access_point_client_secret", data.Id.ValueString())

	getResp, err := pgql.GetAccessPointClientSecret(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "access_point_client_secret", data.Id.ValueString())
		return
	}
	if getResp.AccessPointClientSecret == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.mapFromGraphQL(ctx, *getResp.AccessPointClientSecret)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPointClientSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Every attribute either replaces the secret or is computed, so there is
	// nothing to send.
	var data AccessPointClientSecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPointClientSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccessPointClientSecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "access_point_client_secret", data.Id.ValueString())

	_, err := pgql.DeleteAccessPointClientSecret(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "access_point_client_secret", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "access_point_client_secret", data.Id.ValueString())
}

func (m *AccessPointClientSecretResourceModel) mapFromGraphQL(ctx context.Context, secretDetails pgql.AccessPointClientSecretDetails) diag.Diagnostics {
	// The server may give the expiry in another time zone. The same instant
	// keeps the configuration's spelling, so it doesn't read as a change.
	configuredExpiry := m.ExpiresAt
	diags := setFields(
		ctx,
		&m.Id, secretDetails.Id,
		&m.AccessPointClientId, secretDetails.AccessPointClientID,
		&m.ExpiresAt, secretDetails.ExpiresAt,
		&m.LastUsedAt, secretDetails.LastUsedAt,
		&m.CreatedAt, secretDetails.CreatedAt,
	)
	configured, configuredErr := time.Parse(time.RFC3339, configuredExpiry.ValueString())
	stored, storedErr := time.Parse(time.RFC3339, m.ExpiresAt.ValueString())
	if configuredErr == nil && storedErr == nil && configured.Equal(stored) {
		m.ExpiresAt = configuredExpiry
	}
	return diags
}
