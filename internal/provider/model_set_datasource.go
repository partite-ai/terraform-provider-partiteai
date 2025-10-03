package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ModelSetDataSource{}

func NewModelSetDataSource() datasource.DataSource {
	return &ModelSetDataSource{
		BaseDataSource: NewBaseDataSource("model_set"),
	}
}

// ModelSetDataSource defines the data source implementation.
type ModelSetDataSource struct {
	BaseDataSource
}

// ModelSetDataSourceModel describes the data source data model.
type ModelSetDataSourceModel struct {
	ModelSetBaseModel
}

func dataSourceModelProfileRefType(name string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: name,
		Computed:            true,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Model Profile ID",
				Computed:            true,
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "Model Provider ID",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Model Profile Name",
				Computed:            true,
			},
		},
	}
}

func (d *ModelSetDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Set data source",
		Attributes: standardDataSourceSchemaAttributes(map[string]schema.Attribute{
			"default_model_profile": dataSourceModelProfileRefType("Default Model Profile"),
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "Rules for selecting model profiles",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"model_profile": dataSourceModelProfileRefType("Model Profile"),
						"complexity": schema.StringAttribute{
							MarkdownDescription: "Model complexity",
							Computed:            true,
						},
						"match_tags": schema.ListNestedAttribute{
							MarkdownDescription: "Tags to match for this rule",
							NestedObject:        commonDataSourceSchemas.tagsField.NestedObject,
							Computed:            true,
						},
					},
				},
				Computed: true,
			},
		}),
	}
}

func (d *ModelSetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ModelSetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "model_set", data.Id.ValueString())

	modelSetResp, err := pgql.GetModelSet(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "model_set", data.Id.ValueString())
		return
	}

	// Use the shared mapping from the base model
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, modelSetResp.ModelSet)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
