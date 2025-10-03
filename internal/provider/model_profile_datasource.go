package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ModelProfileDataSource{}

func NewModelProfileDataSource() datasource.DataSource {
	return &ModelProfileDataSource{
		BaseDataSource: NewBaseDataSource("model_profile"),
	}
}

// ModelProfileDataSource defines the data source implementation.
type ModelProfileDataSource struct {
	BaseDataSource
}

// ModelProfileDataSourceModel describes the data source data model.
type ModelProfileDataSourceModel struct {
	ModelProfileBaseModel
}

func (d *ModelProfileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Profile data source",
		Attributes: standardDataSourceSchemaAttributes(map[string]schema.Attribute{
			"model_provider_id": schema.StringAttribute{
				MarkdownDescription: "ID of the Model Provider",
				Computed:            true,
			},
			"anthropic": schema.SingleNestedAttribute{
				MarkdownDescription: "Anthropic profile config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "Anthropic model name",
						Computed:            true,
					},
					"max_tokens": schema.Int64Attribute{
						MarkdownDescription: "Maximum tokens",
						Computed:            true,
					},
					"temperature": schema.Float64Attribute{
						MarkdownDescription: "Temperature",
						Computed:            true,
					},
					"top_p": schema.Float64Attribute{
						MarkdownDescription: "Top P",
						Computed:            true,
					},
					"top_k": schema.Int64Attribute{
						MarkdownDescription: "Top K",
						Computed:            true,
					},
					"thinking_budget_tokens": schema.Int64Attribute{
						MarkdownDescription: "Thinking budget tokens",
						Computed:            true,
					},
				},
			},
			"openai": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI profile config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "OpenAI model name",
						Computed:            true,
					},
					"max_tokens": schema.Int64Attribute{
						MarkdownDescription: "Maximum tokens",
						Computed:            true,
					},
					"temperature": schema.Float64Attribute{
						MarkdownDescription: "Temperature",
						Computed:            true,
					},
					"top_p": schema.Float64Attribute{
						MarkdownDescription: "Top P",
						Computed:            true,
					},
					"reasoning_effort": schema.StringAttribute{
						MarkdownDescription: "Reasoning effort (LOW, MEDIUM, HIGH)",
						Computed:            true,
					},
				},
			},
			"google": schema.SingleNestedAttribute{
				MarkdownDescription: "Google profile config",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "Google model name",
						Computed:            true,
					},
					"max_tokens": schema.Int64Attribute{
						MarkdownDescription: "Maximum tokens",
						Computed:            true,
					},
					"temperature": schema.Float64Attribute{
						MarkdownDescription: "Temperature",
						Computed:            true,
					},
					"top_p": schema.Float64Attribute{
						MarkdownDescription: "Top P",
						Computed:            true,
					},
					"top_k": schema.Int64Attribute{
						MarkdownDescription: "Top K",
						Computed:            true,
					},
					"thinking_budget_tokens": schema.Int64Attribute{
						MarkdownDescription: "Thinking budget tokens",
						Computed:            true,
					},
				},
			},
		}),
	}
}

func (d *ModelProfileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ModelProfileDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "model_profile", data.Id.ValueString())

	modelProfileResp, err := pgql.GetModelProfile(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "model_profile", data.Id.ValueString())
		return
	}

	// Use the shared mapping from the base model
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, modelProfileResp.ModelProfile)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
