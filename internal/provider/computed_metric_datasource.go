package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ComputedMetricDataSource{}

func NewComputedMetricDataSource() datasource.DataSource {
	return &ComputedMetricDataSource{
		BaseDataSource: NewBaseDataSource("computed_metric"),
	}
}

// ComputedMetricDataSource reads one metric the product computes from a
// conversation, such as its latency. There is no resource beside it: these are
// built in and only referenced.
type ComputedMetricDataSource struct {
	BaseDataSource
}

// ComputedMetricDataSourceModel describes the data source data model.
type ComputedMetricDataSourceModel struct {
	Key             types.String `tfsdk:"key"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Category        types.String `tfsdk:"category"`
	CollectionScope types.String `tfsdk:"collection_scope"`
	SummarySource   types.String `tfsdk:"summary_source"`
	Aggregation     types.String `tfsdk:"aggregation"`
	Unit            types.String `tfsdk:"unit"`
}

func (m *ComputedMetricDataSourceModel) mapFromGraphQL(src *pgql.ComputedMetricDetails) {
	m.Key = types.StringValue(src.Key)
	m.Name = types.StringValue(src.Name)
	m.Description = types.StringPointerValue(src.Description)
	m.Category = types.StringValue(string(src.Category))
	m.CollectionScope = types.StringValue(string(src.CollectionScope))
	m.SummarySource = types.StringValue(string(src.SummarySource))
	m.Aggregation = types.StringValue(string(src.Aggregation))
	m.Unit = types.StringValue(string(src.Unit))
}

func (d *ComputedMetricDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A metric the product computes from a conversation, such as its latency, by key. " +
			"Put one in a rubric with `computed_metric_key`. Requires an enterprise deployment.",
		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The metric's key.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The metric's name.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "What the metric measures.",
			},
			"category": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The kind of thing the metric measures.",
			},
			"collection_scope": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Which hops the metric collects a value from: `ROOT_ONLY` or `ALL_HOPS`.",
			},
			"summary_source": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Where the request's value comes from: `ROOT_VALUE` or `AGGREGATE`.",
			},
			"aggregation": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "How the values of the hops are combined.",
			},
			"unit": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unit the value is in.",
			},
		},
	}
}

func (d *ComputedMetricDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if !d.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data ComputedMetricDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "computed_metric", data.Key.ValueString())

	// The API answers an unknown key with an error, which passes through.
	getResp, err := pgql.GetComputedMetric(ctx, d.GetClient(), data.Key.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "computed_metric", data.Key.ValueString())
		return
	}

	data.mapFromGraphQL(&getResp.ComputedMetric)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
