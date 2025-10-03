package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ModelProviderDataSource{}

func NewModelProviderDataSource() datasource.DataSource {
	return &ModelProviderDataSource{
		BaseDataSource: NewBaseDataSource("model_provider"),
	}
}

// ModelProviderDataSource defines the data source implementation.
type ModelProviderDataSource struct {
	BaseDataSource
}

// ModelProviderDataSourceModel describes the data source data model.
type ModelProviderDataSourceModel struct {
	ModelProviderBaseModel
}

func (d *ModelProviderDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Provider data source",
		Attributes: standardDataSourceSchemaAttributes(map[string]schema.Attribute{
			"anthropic": schema.SingleNestedAttribute{
				MarkdownDescription: "Anthropic provider config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"api_key": schema.StringAttribute{
						MarkdownDescription: "API key for Anthropic",
						Computed:            true,
					},
				},
			},
			"openai": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI provider config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"api_key": schema.StringAttribute{
						MarkdownDescription: "API key for OpenAI",
						Computed:            true,
						Sensitive:           true,
					},
				},
			},
			"google_genai": schema.SingleNestedAttribute{
				MarkdownDescription: "Google GenAI provider config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"gemini": schema.SingleNestedAttribute{
						MarkdownDescription: "Gemini API config",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"api_key": schema.StringAttribute{
								MarkdownDescription: "API key for Gemini",
								Computed:            true,
								Sensitive:           true,
							},
						},
					},
					"vertex": schema.SingleNestedAttribute{
						MarkdownDescription: "Vertex API config",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"service_account_key": schema.StringAttribute{
								MarkdownDescription: "Service account key for Vertex",
								Computed:            true,
								Sensitive:           true,
							},
							"project_id": schema.StringAttribute{
								MarkdownDescription: "Project ID for Vertex",
								Computed:            true,
							},
							"location": schema.StringAttribute{
								MarkdownDescription: "Location for Vertex",
								Computed:            true,
							},
						},
					},
				},
			},
			"openai_compatible": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI-compatible provider config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "URL for OpenAI-compatible provider",
						Computed:            true,
					},
					"api_token": schema.StringAttribute{
						MarkdownDescription: "API token for OpenAI-compatible provider",
						Computed:            true,
						Sensitive:           true,
					},
				},
			},
		}),
	}
}

func (d *ModelProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ModelProviderDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "model_provider", data.Id.ValueString())

	modelProviderResp, err := pgql.GetModelProvider(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "model_provider", data.Id.ValueString())
		return
	}

	// Use the shared mapping from the base model
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, modelProviderResp.ModelProvider)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
