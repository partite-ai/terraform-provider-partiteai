package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &EvalMeshMonitorDataSource{}

func NewEvalMeshMonitorDataSource() datasource.DataSource {
	return &EvalMeshMonitorDataSource{
		BaseDataSource: NewBaseDataSource("eval_mesh_monitor"),
	}
}

// EvalMeshMonitorDataSource defines the data source implementation.
type EvalMeshMonitorDataSource struct {
	BaseDataSource
}

// EvalMeshMonitorDataSourceModel describes the data source data model.
type EvalMeshMonitorDataSourceModel struct {
	EvalMeshMonitorBaseModel
}

func (d *EvalMeshMonitorDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One mesh monitor, by id. Requires an enterprise deployment.",
		Attributes: map[string]schema.Attribute{
			"id": commonDataSourceSchemas.id,
			"mesh_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mesh whose conversations are scored.",
			},
			"rubric_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Rubric the conversations are scored against.",
			},
			"model_set_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Model set the judging runs on.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the monitor is scoring.",
			},
			"sample_rate": schema.Float64Attribute{
				Computed:            true,
				MarkdownDescription: "Share of conversations scored, from 0 to 1.",
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *EvalMeshMonitorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if !d.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data EvalMeshMonitorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "eval_mesh_monitor", data.Id.ValueString())

	getResp, err := pgql.GetEvalMeshMonitor(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "eval_mesh_monitor", data.Id.ValueString())
		return
	}
	if getResp.EvalMeshMonitor == nil {
		resp.Diagnostics.AddError(
			"Mesh Monitor Not Found",
			fmt.Sprintf("no mesh monitor with ID %q", data.Id.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, getResp.EvalMeshMonitor)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
