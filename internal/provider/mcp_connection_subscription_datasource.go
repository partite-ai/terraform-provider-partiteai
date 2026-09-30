package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &McpConnectionSubscriptionDataSource{}

func NewMcpConnectionSubscriptionDataSource() datasource.DataSource {
	return &McpConnectionSubscriptionDataSource{
		BaseDataSource: NewBaseDataSource("mcp_connection_subscription"),
	}
}

type McpConnectionSubscriptionDataSource struct {
	BaseDataSource
}

type McpConnectionSubscriptionDataSourceModel struct {
	Id              types.String      `tfsdk:"id"`
	McpConnectionId types.String      `tfsdk:"mcp_connection_id"`
	Name            types.String      `tfsdk:"name"`
	Criteria        types.Set         `tfsdk:"criteria"`
	TunnelKeys      types.List        `tfsdk:"tunnel_keys"`
	CreatedAt       timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt       timetypes.RFC3339 `tfsdk:"updated_at"`
}

func (d *McpConnectionSubscriptionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One subscription of a tunnel MCP connection, with the keys issued for it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Subscription identifier",
			},
			"mcp_connection_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The tunnel connection this subscription belongs to.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Names the subscription within its connection.",
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
			"criteria": schema.SetNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Tags a conversation must carry for this subscription to serve it.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":  schema.StringAttribute{Computed: true},
						"value": schema.StringAttribute{Computed: true},
					},
				},
			},
			"tunnel_keys": schema.ListNestedAttribute{
				Computed: true,
				MarkdownDescription: "Credentials issued for this subscription. The credential itself is " +
					"readable only when a key is created, so it is not here.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":    schema.StringAttribute{Computed: true},
						"label": schema.StringAttribute{Computed: true},
						"attached": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "False until a subscription has taken the key.",
						},
						"expires_at": schema.StringAttribute{
							CustomType: timetypes.RFC3339Type{},
							Computed:   true,
						},
						"revoked_at": schema.StringAttribute{
							CustomType: timetypes.RFC3339Type{},
							Computed:   true,
						},
						"created_at": schema.StringAttribute{
							CustomType: timetypes.RFC3339Type{},
							Computed:   true,
						},
					},
				},
			},
		},
	}
}

func (d *McpConnectionSubscriptionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data McpConnectionSubscriptionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, diags := readMcpConnectionSubscription(ctx, d.GetClient(), data.McpConnectionId.ValueString(), data.Id.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if found == nil {
		resp.Diagnostics.AddError("Not Found", "MCP connection subscription not found")
		return
	}

	resp.Diagnostics.Append(mapMcpConnectionSubscription(ctx,
		&data.Id, &data.Name, &data.Criteria, &data.CreatedAt, &data.UpdatedAt,
		found.McpConnectionSubscriptionDetails)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, keyDiags := tunnelKeyList(ctx, found.TunnelKeys)
	resp.Diagnostics.Append(keyDiags...)
	data.TunnelKeys = keys

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *McpConnectionSubscriptionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_mcp_connection_subscription", req.ProviderTypeName)
}
