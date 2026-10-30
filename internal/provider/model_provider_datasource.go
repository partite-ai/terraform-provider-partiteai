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
					"api_key_set": schema.BoolAttribute{
						MarkdownDescription: "Whether an API key is set for Anthropic. The key itself is never returned.",
						Computed:            true,
					},
				},
			},
			"openai": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI provider config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"api_key_set": schema.BoolAttribute{
						MarkdownDescription: "Whether an API key is set for OpenAI. The key itself is never returned.",
						Computed:            true,
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
							"api_key_set": schema.BoolAttribute{
								MarkdownDescription: "Whether an API key is set for Gemini. The key itself is never returned.",
								Computed:            true,
							},
						},
					},
					"vertex": schema.SingleNestedAttribute{
						MarkdownDescription: "Vertex API config",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"service_account_key_set": schema.BoolAttribute{
								MarkdownDescription: "Whether a service account key is set for Vertex. The key itself is never returned.",
								Computed:            true,
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
					"api_token_set": schema.BoolAttribute{
						MarkdownDescription: "Whether the OpenAI-compatible provider sends an API token. The token itself is never returned.",
						Computed:            true,
					},
				},
			},
			"fireworks": schema.SingleNestedAttribute{
				MarkdownDescription: "Fireworks provider config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"api_key_set": schema.BoolAttribute{
						MarkdownDescription: "Whether an API key is set for Fireworks. The key itself is never returned.",
						Computed:            true,
					},
					"account_id": schema.StringAttribute{
						MarkdownDescription: "Fireworks account id. When set, the model list includes this account's models.",
						Computed:            true,
					},
				},
			},
			"cerebras": schema.SingleNestedAttribute{
				MarkdownDescription: "Cerebras provider config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"api_key_set": schema.BoolAttribute{
						MarkdownDescription: "Whether an API key is set for Cerebras. The key itself is never returned.",
						Computed:            true,
					},
				},
			},
			"bedrock": schema.SingleNestedAttribute{
				MarkdownDescription: "Amazon Bedrock provider config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"region": schema.StringAttribute{
						MarkdownDescription: "The AWS Region that serves the models.",
						Computed:            true,
					},
					"api_key_set": schema.BoolAttribute{
						MarkdownDescription: "Whether an Amazon Bedrock API key is set. The key itself is never returned.",
						Computed:            true,
					},
					"static_keys": schema.SingleNestedAttribute{
						MarkdownDescription: "IAM access keys.",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"access_key_id": schema.StringAttribute{
								MarkdownDescription: "IAM access key ID.",
								Computed:            true,
							},
							"secret_access_key_set": schema.BoolAttribute{
								MarkdownDescription: "Whether an IAM secret access key is set. The key itself is never returned.",
								Computed:            true,
							},
						},
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
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, modelProviderResp.ModelProvider, secretIsSet)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
