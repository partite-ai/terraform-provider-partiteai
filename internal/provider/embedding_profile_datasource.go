package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ datasource.DataSource = &EmbeddingProfileDataSource{}

func NewEmbeddingProfileDataSource() datasource.DataSource {
	return &EmbeddingProfileDataSource{
		BaseDataSource: NewBaseDataSource("embedding_profile"),
	}
}

type EmbeddingProfileDataSource struct {
	BaseDataSource
}

type EmbeddingProfileDataSourceModel struct {
	EmbeddingProfileBaseModel
}

func (d *EmbeddingProfileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An embedding profile: the model a memory store writes its vectors with.",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"created_at":  commonDataSourceSchemas.createdAt,
			"updated_at":  commonDataSourceSchemas.updatedAt,
			"model_provider_id": schema.StringAttribute{
				MarkdownDescription: "ID of the model provider this profile belongs to",
				Computed:            true,
			},
			"dimensions": schema.Int64Attribute{
				MarkdownDescription: "The width of the vectors this profile produces",
				Computed:            true,
			},
			"openai": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI embedding configuration",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "The name the service knows the embedding model by",
						Computed:            true,
					},
					"dimensions": schema.Int64Attribute{
						MarkdownDescription: "The width of the vectors this profile produces",
						Computed:            true,
					},
					"send_dimensions": schema.BoolAttribute{
						MarkdownDescription: "Whether the width is asked for with each request",
						Computed:            true,
					},
				},
			},
			"google": schema.SingleNestedAttribute{
				MarkdownDescription: "Google embedding configuration",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "The name the service knows the embedding model by",
						Computed:            true,
					},
					"dimensions": schema.Int64Attribute{
						MarkdownDescription: "The width of the vectors this profile produces",
						Computed:            true,
					},
					"send_dimensions": schema.BoolAttribute{
						MarkdownDescription: "Whether the width is asked for with each request",
						Computed:            true,
					},
					"task_type": schema.StringAttribute{
						MarkdownDescription: "What the embedding is for, such as RETRIEVAL_DOCUMENT",
						Computed:            true,
					},
				},
			},
		},
	}
}

func (d *EmbeddingProfileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EmbeddingProfileDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "embedding_profile", data.Id.ValueString())

	profileResp, err := pgql.GetEmbeddingProfile(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "embedding_profile", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, profileResp.EmbeddingProfile)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
