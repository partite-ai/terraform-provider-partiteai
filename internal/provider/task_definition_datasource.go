package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &TaskDefinitionDataSource{}
var _ datasource.DataSourceWithConfigValidators = &TaskDefinitionDataSource{}

func NewTaskDefinitionDataSource() datasource.DataSource {
	return &TaskDefinitionDataSource{
		BaseDataSource: NewBaseDataSource("task_definition"),
	}
}

// TaskDefinitionDataSource defines the data source implementation.
type TaskDefinitionDataSource struct {
	BaseDataSource
}

// TaskDefinitionDataSourceModel describes the data source data model.
type TaskDefinitionDataSourceModel struct {
	TaskDefinitionBaseModel
}

func (d *TaskDefinitionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Task Definition data source. Look up an author-defined task definition by `id` or by `name` (exactly one must be set).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Task definition identifier. Exactly one of `id` or `name` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Task definition name (the tool name exposed to the LLM). Exactly one of `id` or `name` must be set.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the task",
			},
			"input_message_type": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				Computed:            true,
				MarkdownDescription: "JSON schema the agent must satisfy when invoking the task",
			},
			"output_message_type": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				Computed:            true,
				MarkdownDescription: "JSON schema the user must satisfy when completing the task",
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *TaskDefinitionDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *TaskDefinitionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TaskDefinitionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var details *pgql.TaskDefinitionDetails
	if !data.Id.IsNull() {
		logDataSourceOperation(ctx, "reading", "task_definition", data.Id.ValueString())

		getResp, err := pgql.GetTaskDefinition(ctx, d.GetClient(), data.Id.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "task_definition", data.Id.ValueString())
			return
		}
		details = getResp.TaskDefinition
		if details == nil {
			resp.Diagnostics.AddError(
				"Task Definition Not Found",
				fmt.Sprintf("no task definition with ID %q", data.Id.ValueString()),
			)
			return
		}
	} else {
		logDataSourceOperation(ctx, "reading", "task_definition", data.Name.ValueString())

		getResp, err := pgql.GetTaskDefinitionByName(ctx, d.GetClient(), data.Name.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "task_definition", data.Name.ValueString())
			return
		}
		details = getResp.TaskDefinitionByName
		if details == nil {
			resp.Diagnostics.AddError(
				"Task Definition Not Found",
				fmt.Sprintf("no task definition named %q", data.Name.ValueString()),
			)
			return
		}
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, details)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
