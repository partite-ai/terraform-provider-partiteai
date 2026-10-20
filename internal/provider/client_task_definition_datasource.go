package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ClientTaskDefinitionDataSource{}
var _ datasource.DataSourceWithConfigValidators = &ClientTaskDefinitionDataSource{}

func NewClientTaskDefinitionDataSource() datasource.DataSource {
	return &ClientTaskDefinitionDataSource{
		BaseDataSource: NewBaseDataSource("client_task_definition"),
	}
}

// ClientTaskDefinitionDataSource defines the data source implementation.
type ClientTaskDefinitionDataSource struct {
	BaseDataSource
}

// ClientTaskDefinitionDataSourceModel describes the data source data model.
type ClientTaskDefinitionDataSourceModel struct {
	ClientTaskDefinitionBaseModel
}

func (d *ClientTaskDefinitionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Client Task Definition data source. Look up an author-defined client task definition by `id` or by `name` (exactly one must be set).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Client task definition identifier. Exactly one of `id` or `name` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Client task definition name (the tool name exposed to the LLM). Exactly one of `id` or `name` must be set.",
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
			"ui_forms": schema.MapAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "UIs a client can show to complete the task, keyed by form type, with the form's definition as the value",
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *ClientTaskDefinitionDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *ClientTaskDefinitionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ClientTaskDefinitionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var details *pgql.ClientTaskDefinitionDetails
	if !data.Id.IsNull() {
		logDataSourceOperation(ctx, "reading", "client_task_definition", data.Id.ValueString())

		getResp, err := pgql.GetClientTaskDefinition(ctx, d.GetClient(), data.Id.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "client_task_definition", data.Id.ValueString())
			return
		}
		details = getResp.ClientTaskDefinition
		if details == nil {
			resp.Diagnostics.AddError(
				"Client Task Definition Not Found",
				fmt.Sprintf("no client task definition with ID %q", data.Id.ValueString()),
			)
			return
		}
	} else {
		logDataSourceOperation(ctx, "reading", "client_task_definition", data.Name.ValueString())

		getResp, err := pgql.GetClientTaskDefinitionByName(ctx, d.GetClient(), data.Name.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "client_task_definition", data.Name.ValueString())
			return
		}
		details = getResp.ClientTaskDefinitionByName
		if details == nil {
			resp.Diagnostics.AddError(
				"Client Task Definition Not Found",
				fmt.Sprintf("no client task definition named %q", data.Name.ValueString()),
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
