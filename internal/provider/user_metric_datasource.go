package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &UserMetricDataSource{}

func NewUserMetricDataSource() datasource.DataSource {
	return &UserMetricDataSource{
		BaseDataSource: NewBaseDataSource("user_metric"),
	}
}

// UserMetricDataSource defines the data source implementation.
type UserMetricDataSource struct {
	BaseDataSource
}

// UserMetricDataSourceModel describes the data source data model.
type UserMetricDataSourceModel struct {
	UserMetricBaseModel
}

func (d *UserMetricDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A metric of your own, by id. Requires an enterprise deployment.",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"prompt": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "What the judging model is asked.",
			},
			"complexity": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Model complexity the metric judges at.",
			},
			"examples": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Worked examples shown to the judging model.",
				NestedObject:        scoreExampleDataSourceObject,
			},
			"input_fields": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Values a rubric supplies when it uses this metric.",
				NestedObject:        judgeInputFieldDataSourceObject,
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *UserMetricDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if !d.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data UserMetricDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "user_metric", data.Id.ValueString())

	getResp, err := pgql.GetUserMetric(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "user_metric", data.Id.ValueString())
		return
	}
	if getResp.UserMetric == nil {
		resp.Diagnostics.AddError(
			"User Metric Not Found",
			fmt.Sprintf("no user metric with ID %q", data.Id.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, getResp.UserMetric)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
