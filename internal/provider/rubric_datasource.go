package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &RubricDataSource{}

func NewRubricDataSource() datasource.DataSource {
	return &RubricDataSource{
		BaseDataSource: NewBaseDataSource("rubric"),
	}
}

// RubricDataSource defines the data source implementation.
type RubricDataSource struct {
	BaseDataSource
}

// RubricDataSourceModel describes the data source data model.
type RubricDataSourceModel struct {
	RubricBaseModel
}

func (d *RubricDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One rubric, by id. Requires an enterprise deployment.",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"metrics": schema.SetNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The metrics this rubric scores against.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"user_metric_id":    schema.StringAttribute{Computed: true},
						"system_metric_key": schema.StringAttribute{Computed: true},
						"trial_count":       schema.Int64Attribute{Computed: true},
						"complexity":        schema.StringAttribute{Computed: true},
						"input_values": schema.StringAttribute{
							CustomType: jsontypes.NormalizedType{},
							Computed:   true,
						},
						"score_floor":     schema.Float64Attribute{Computed: true},
						"score_ceiling":   schema.Float64Attribute{Computed: true},
						"lower_is_better": schema.BoolAttribute{Computed: true},
						"weight":          schema.Float64Attribute{Computed: true},
					},
				},
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *RubricDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if !d.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data RubricDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "rubric", data.Id.ValueString())

	getResp, err := pgql.GetRubric(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "rubric", data.Id.ValueString())
		return
	}
	if getResp.Rubric == nil {
		resp.Diagnostics.AddError(
			"Rubric Not Found",
			fmt.Sprintf("no rubric with ID %q", data.Id.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, getResp.Rubric)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
