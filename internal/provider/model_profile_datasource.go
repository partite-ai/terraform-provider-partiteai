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
			"configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "The model and the settings it is called with.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "The name the service knows the model by",
						Computed:            true,
					},
					"max_tokens": schema.Int64Attribute{
						MarkdownDescription: "The most tokens the model may produce in one response",
						Computed:            true,
					},
					"temperature": schema.Float64Attribute{
						MarkdownDescription: "How much the model is allowed to vary its output",
						Computed:            true,
					},
					"top_p": schema.Float64Attribute{
						MarkdownDescription: "Nucleus sampling cutoff",
						Computed:            true,
					},
					"top_k": schema.Int64Attribute{
						MarkdownDescription: "How many candidate tokens the model considers",
						Computed:            true,
					},
					"thinking_budget_tokens": schema.Int64Attribute{
						MarkdownDescription: "Tokens the model may spend reasoning before it answers",
						Computed:            true,
					},
					"reasoning_effort": schema.StringAttribute{
						MarkdownDescription: "How hard the model should think",
						Computed:            true,
					},
					"thinking_type": schema.StringAttribute{
						MarkdownDescription: "How the model thinks",
						Computed:            true,
					},
					"thinking_display": schema.StringAttribute{
						MarkdownDescription: "What the model returns of its thinking",
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
