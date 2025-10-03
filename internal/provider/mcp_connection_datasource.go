package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

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
			"stdio_transport": schema.SingleNestedAttribute{
				MarkdownDescription: "Standard I/O transport configuration for the MCP connection.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"command": schema.StringAttribute{
						MarkdownDescription: "The command to execute for the standard I/O transport.",
						Computed:            true,
					},
					"arguments": schema.ListAttribute{
						MarkdownDescription: "The arguments to pass to the command.",
						ElementType:         types.StringType,
						Computed:            true,
					},
					"environment": schema.ListNestedAttribute{
						MarkdownDescription: "Environment variables for the command.",
						Computed:            true,
						NestedObject:        envVarDataSourceType,
					},
				},
			},
			"server_initiated_http_transport": schema.SingleNestedAttribute{
				MarkdownDescription: "Server-initiated HTTP transport configuration for the MCP connection.",
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
