package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &McpConnectionTunnelKeyResource{}
var _ resource.ResourceWithImportState = &McpConnectionTunnelKeyResource{}

func NewMcpConnectionTunnelKeyResource() resource.Resource {
	return &McpConnectionTunnelKeyResource{
		BaseResource: NewBaseResource("mcp_connection_tunnel_key"),
	}
}

// McpConnectionTunnelKeyResource manages a relay credential.
type McpConnectionTunnelKeyResource struct {
	BaseResource
}

type McpConnectionTunnelKeyResourceModel struct {
	Id                          types.String      `tfsdk:"id"`
	McpConnectionId             types.String      `tfsdk:"mcp_connection_id"`
	McpConnectionSubscriptionId types.String      `tfsdk:"mcp_connection_subscription_id"`
	Label                       types.String      `tfsdk:"label"`
	ExpiresAt                   timetypes.RFC3339 `tfsdk:"expires_at"`
	Credential                  types.String      `tfsdk:"credential"`
	RevokedAt                   timetypes.RFC3339 `tfsdk:"revoked_at"`
	CreatedAt                   timetypes.RFC3339 `tfsdk:"created_at"`
}

func (r *McpConnectionTunnelKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A relay credential for one subscription of a tunnel MCP connection.\n\n" +
			"A key names exactly one subscription and a relay names nothing, so a relay can only reach the " +
			"subscription its key was issued for.\n\n" +
			"The credential is returned when the key is created and is never readable again, so it is held in " +
			"Terraform state. Treat the state as a secret.",
		Attributes: map[string]schema.Attribute{
			"id":         commonSchemas.id,
			"created_at": commonSchemas.createdAt,
			"mcp_connection_id": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The connection the subscription belongs to. A key is reached through its " +
					"connection, so this is named rather than derived.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mcp_connection_subscription_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The subscription this key serves.",
				PlanModifiers: []planmodifier.String{
					// A key belongs to one subscription for its whole life: the API
					// will not move an attached key, so a change replaces it.
					stringplanmodifier.RequiresReplace(),
				},
			},
			"label": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Names the key for an operator, so `laptop-bob` can be told from `ci` " +
					"without decoding anything.",
				PlanModifiers: []planmodifier.String{
					// The API mints a label with the key and does not change it.
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				CustomType: timetypes.RFC3339Type{},
				Optional:   true,
				MarkdownDescription: "When the key stops working. Left out, it does not expire and is " +
					"withdrawn by destroying it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				MarkdownDescription: "The credential to configure on a relay, as `PARTITE_RELAY_TOKEN`. It " +
					"exists in this form once, so it is kept in state and cannot be recovered from the API.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"revoked_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "When the key was withdrawn, if it has been.",
			},
		},
	}
}

func (r *McpConnectionTunnelKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data McpConnectionTunnelKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "mcp_connection_tunnel_key", data.Label.ValueString())

	subscriptionID := data.McpConnectionSubscriptionId.ValueString()
	input := pgql.NewMcpConnectionTunnelKey{
		Label:                       data.Label.ValueString(),
		McpConnectionSubscriptionID: &subscriptionID,
	}
	if !data.ExpiresAt.IsNull() && !data.ExpiresAt.IsUnknown() {
		expiresAt := data.ExpiresAt.ValueString()
		input.ExpiresAt = &expiresAt
	}

	created, err := pgql.CreateMcpConnectionTunnelKey(ctx, r.GetClient(), input)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create mcp connection tunnel key")
		return
	}

	// The only time the credential is readable.
	data.Credential = types.StringValue(created.CreateMcpConnectionTunnelKey.Credential)
	resp.Diagnostics.Append(r.mapResponse(ctx, &data, created.CreateMcpConnectionTunnelKey.TunnelKey)...)

	logResourceOperation(ctx, "created", "mcp_connection_tunnel_key", data.Label.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionTunnelKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpConnectionTunnelKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "mcp_connection_tunnel_key", data.Label.ValueString())

	found, subscriptionID, diags := readMcpConnectionTunnelKey(ctx, r.GetClient(),
		data.McpConnectionId.ValueString(),
		data.McpConnectionSubscriptionId.ValueString(),
		data.Id.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.McpConnectionSubscriptionId = types.StringValue(subscriptionID)

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, *found)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update never runs: every configurable attribute requires replacement, because the
// API mints a key once and only ever withdraws it.
func (r *McpConnectionTunnelKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data McpConnectionTunnelKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionTunnelKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpConnectionTunnelKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "mcp_connection_tunnel_key", data.Label.ValueString())

	// Revoking is how a key is withdrawn. It takes effect on the next message
	// dispatched to a relay holding it, rather than by closing its connection.
	if _, err := pgql.RevokeMcpConnectionTunnelKey(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIError(&resp.Diagnostics, err, "revoke mcp connection tunnel key")
		return
	}
}

func (r *McpConnectionTunnelKeyResource) mapResponse(ctx context.Context, data *McpConnectionTunnelKeyResourceModel, details pgql.McpConnectionTunnelKeyDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&data.Id, details.Id,
		&data.Label, details.Label,
		&data.CreatedAt, details.CreatedAt,
	)
	if diags.HasError() {
		return diags
	}

	data.ExpiresAt = optionalRFC3339(details.ExpiresAt)
	data.RevokedAt = optionalRFC3339(details.RevokedAt)
	return diags
}

func (r *McpConnectionTunnelKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_mcp_connection_tunnel_key", req.ProviderTypeName)
}

func (r *McpConnectionTunnelKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// A key is reached through its connection, so importing needs both. Read
	// finds the subscription it belongs to.
	connectionID, keyID, ok := cutImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			"The import identifier must be in the format '<mcp_connection_id>:<tunnel_key_id>'.\n\n"+
				"Example: 'cmco_1234567890abcdef:ptk_1234567890abcdef'",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mcp_connection_id"), connectionID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), keyID)...)
}
