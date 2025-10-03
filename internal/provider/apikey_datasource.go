package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ApiKeyDataSource{}

func NewApiKeyDataSource() datasource.DataSource {
	return &ApiKeyDataSource{
		BaseDataSource: NewBaseDataSource("api_key"),
	}
}

// ApiKeyDataSource defines the data source implementation.
type ApiKeyDataSource struct {
	BaseDataSource
}

// ApiKeyDataSourceModel extends the base model with data source-specific fields
type ApiKeyDataSourceModel struct {
	ApiKeyBaseModel
	Secret types.String `tfsdk:"secret"` // Read-only, may be empty for existing keys
}

func (d *ApiKeyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "API Key data source",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"created_at":  commonDataSourceSchemas.createdAt,
			"updated_at":  commonDataSourceSchemas.updatedAt,
			"scopes": schema.SetAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of scopes for the API key",
			},
			"kid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Key ID",
			},
			"secret": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "API key secret (only available after creation)",
			},
		},
	}
}

func (d *ApiKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ApiKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "api_key", data.Id.ValueString())

	apiKeyResp, err := pgql.GetApiKey(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "api_key", data.Id.ValueString())
		return
	}

	// Use the shared mapping function directly
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, &apiKeyResp.ApiKey)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
