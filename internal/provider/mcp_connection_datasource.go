package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &McpConnectionDataSource{}

func NewMcpConnectionDataSource() datasource.DataSource {
	return &McpConnectionDataSource{
		BaseDataSource: NewBaseDataSource("mcp_connection"),
	}
}

// McpConnectionDataSource defines the data source implementation.
type McpConnectionDataSource struct {
	BaseDataSource
}

// McpConnectionDataSourceModel describes the data source data model.
type McpConnectionDataSourceModel struct {
	McpConnectionBaseModel
}

func (d *McpConnectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "MCP connection data source",
		Attributes: map[string]schema.Attribute{
			"id":         commonDataSourceSchemas.id,
			"name":       commonDataSourceSchemas.name,
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
			"http_transport": schema.SingleNestedAttribute{
				MarkdownDescription: "HTTP transport configuration for the MCP connection.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "The URL of the HTTP endpoint.",
						Computed:            true,
					},
					"authentication_method": schema.StringAttribute{
						MarkdownDescription: "The authentication method to use. Valid values are 'NONE', 'BASIC', 'HEADER', 'OAUTH2_AUTHORIZATION_CODE', 'OAUTH2_CLIENT_CREDENTIALS'.",
						Computed:            true,
					},
				},
			},
			"tunnel_transport": schema.SingleNestedAttribute{
				MarkdownDescription: "Tunnel transport configuration. The server reaches mesh by connecting outwards, so it needs no inbound endpoint, and a relay speaks the tunnel protocol on its behalf. Which relays serve which conversations is decided by partiteai_mcp_connection_subscription resources.",
				Computed:            true,
				Attributes:          map[string]schema.Attribute{},
			},
		},
	}
}

func (d *McpConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data McpConnectionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "mcp_connection", data.Id.ValueString())

	mcpConnResp, err := pgql.GetMcpConnection(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mcp_connection", data.Id.ValueString())
		return
	}

	// Use the shared mapping from the base model
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *mcpConnResp.McpConnection)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
