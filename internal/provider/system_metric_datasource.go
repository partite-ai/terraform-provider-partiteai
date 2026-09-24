package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SystemMetricDataSource{}

func NewSystemMetricDataSource() datasource.DataSource {
	return &SystemMetricDataSource{
		BaseDataSource: NewBaseDataSource("system_metric"),
	}
}

// SystemMetricDataSource reads one metric that came with the product. There is
// no resource beside it: these are not authored, only referenced.
type SystemMetricDataSource struct {
	BaseDataSource
}

// SystemMetricDataSourceModel describes the data source data model.
type SystemMetricDataSourceModel struct {
	Key         types.String `tfsdk:"key"`
	Type        types.String `tfsdk:"type"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Complexity  types.String `tfsdk:"complexity"`
	Prompt      types.String `tfsdk:"prompt"`
	Examples    types.List   `tfsdk:"examples"`
	InputFields types.List   `tfsdk:"input_fields"`
}

func (m *SystemMetricDataSourceModel) mapFromGraphQL(ctx context.Context, src *pgql.SystemMetricDetails) diag.Diagnostics {
	var diags diag.Diagnostics
	m.Key = types.StringValue(src.Key)
	m.Type = types.StringValue(string(src.Type))
	m.Name = types.StringValue(src.Name)
	m.Description = types.StringPointerValue(src.Description)
	m.Prompt = types.StringPointerValue(src.Prompt)
	m.Complexity = types.StringNull()
	if src.Complexity != nil {
		m.Complexity = types.StringValue(string(*src.Complexity))
	}
	m.Examples = scoreExampleList(ctx, src.Examples, &diags)
	m.InputFields = judgeInputFieldList(ctx, src.InputFields, &diags)
	return diags
}

func systemMetricAttributes(keyRequired bool) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key": schema.StringAttribute{
			MarkdownDescription: "Stable identifier of the metric, which is what a rubric refers to it by.",
			Required:            keyRequired,
			Computed:            !keyRequired,
		},
		"type": schema.StringAttribute{
			MarkdownDescription: "`JUDGED` for a metric a model scores, `COMPUTED` for one worked out from the trace.",
			Computed:            true,
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Name shown to a person.",
			Computed:            true,
		},
		"description": schema.StringAttribute{
			MarkdownDescription: "What the metric measures.",
			Computed:            true,
		},
		"complexity": schema.StringAttribute{
			MarkdownDescription: "Model complexity the metric judges at, when it judges.",
			Computed:            true,
		},
		"prompt": schema.StringAttribute{
			MarkdownDescription: "Prompt the judging model is given.",
			Computed:            true,
		},
		"examples": schema.ListNestedAttribute{
			MarkdownDescription: "Worked examples shown to the judging model.",
			Computed:            true,
			NestedObject:        scoreExampleDataSourceObject,
		},
		"input_fields": schema.ListNestedAttribute{
			MarkdownDescription: "Values a rubric supplies when it uses this metric.",
			Computed:            true,
			NestedObject:        judgeInputFieldDataSourceObject,
		},
	}
}

func (d *SystemMetricDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One evaluation metric that came with the product, looked up by `key`. " +
			"Requires an enterprise deployment.",
		Attributes: systemMetricAttributes(true),
	}
}

func (d *SystemMetricDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if !d.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data SystemMetricDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "system_metric", data.Key.ValueString())

	getResp, err := pgql.GetSystemMetric(ctx, d.GetClient(), data.Key.ValueString())
	if err != nil {
		// The API answers a key it does not hold with an error of its own, so
		// there is no null to check for here.
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "system_metric", data.Key.ValueString())
		return
	}

	resp.Diagnostics.Append(data.mapFromGraphQL(ctx, &getResp.SystemMetric)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
