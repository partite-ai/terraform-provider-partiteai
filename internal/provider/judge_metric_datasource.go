package provider

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &JudgeMetricDataSource{}
var _ datasource.DataSourceWithConfigValidators = &JudgeMetricDataSource{}

func NewJudgeMetricDataSource() datasource.DataSource {
	return &JudgeMetricDataSource{
		BaseDataSource: NewBaseDataSource("judge_metric"),
	}
}

// JudgeMetricDataSource defines the data source implementation.
type JudgeMetricDataSource struct {
	BaseDataSource
}

// JudgeMetricDataSourceModel describes the data source data model.
type JudgeMetricDataSourceModel struct {
	JudgeMetricBaseModel
}

func (d *JudgeMetricDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A judge metric, by `id` or by `name` (exactly one must be set). Use the name to " +
			"find one of the built-in judge metrics every organization starts with, such as " +
			"`Instruction Following`. Requires an enterprise deployment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Judge metric identifier. Exactly one of `id` or `name` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Judge metric name, matched exactly. Exactly one of `id` or `name` must be set.",
			},
			"description": commonDataSourceSchemas.description,
			"category": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "What the metric measures.",
			},
			"scope": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "What the model judges: the whole request or each hop between agents.",
			},
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

func (d *JudgeMetricDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *JudgeMetricDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if !d.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data JudgeMetricDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var details *pgql.JudgeMetricDetails
	if !data.Id.IsNull() {
		logDataSourceOperation(ctx, "reading", "judge_metric", data.Id.ValueString())

		getResp, err := pgql.GetJudgeMetric(ctx, d.GetClient(), data.Id.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "judge_metric", data.Id.ValueString())
			return
		}
		details = getResp.JudgeMetric
		if details == nil {
			resp.Diagnostics.AddError("Judge Metric Not Found",
				fmt.Sprintf("no judge metric with ID %q", data.Id.ValueString()))
			return
		}
	} else {
		logDataSourceOperation(ctx, "reading", "judge_metric", data.Name.ValueString())

		found, err := judgeMetricByName(ctx, d.GetClient(), data.Name.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "judge_metric", data.Name.ValueString())
			return
		}
		details = found
		if details == nil {
			resp.Diagnostics.AddError("Judge Metric Not Found",
				fmt.Sprintf("no judge metric named %q", data.Name.ValueString()))
			return
		}
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, details)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// judgeMetricSearchPageSize is how many metrics one search request reads.
const judgeMetricSearchPageSize = 50

// judgeMetricByName finds the judge metric with exactly this name, or nil. The
// API finds metrics by text, which also matches names that only contain this
// one, so every page of the search is read and the exact match kept.
func judgeMetricByName(ctx context.Context, client graphql.Client, name string) (*pgql.JudgeMetricDetails, error) {
	var after *string
	for {
		page, err := pgql.SearchJudgeMetrics(ctx, client, name, judgeMetricSearchPageSize, after)
		if err != nil {
			return nil, err
		}
		for _, edge := range page.Metrics.Edges {
			judge, ok := edge.Node.(*pgql.SearchJudgeMetricsMetricsMetricConnectionEdgesMetricEdgeNodeJudgeMetric)
			if ok && judge.Name == name {
				return &judge.JudgeMetricDetails, nil
			}
		}
		if !page.Metrics.PageInfo.HasNextPage || page.Metrics.PageInfo.EndCursor == nil {
			return nil, nil
		}
		after = page.Metrics.PageInfo.EndCursor
	}
}
