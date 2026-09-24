package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &WorkspaceDataSource{}
var _ datasource.DataSourceWithConfigValidators = &WorkspaceDataSource{}

func NewWorkspaceDataSource() datasource.DataSource {
	return &WorkspaceDataSource{
		BaseDataSource: NewBaseDataSource("workspace"),
	}
}

// WorkspaceDataSource defines the data source implementation.
type WorkspaceDataSource struct {
	BaseDataSource
}

// WorkspaceDataSourceModel describes the data source data model.
type WorkspaceDataSourceModel struct {
	WorkspaceBaseModel
}

func (d *WorkspaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Workspace data source. Look up a workspace by `id` or by `name` (exactly one must be set).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Workspace identifier. Exactly one of `id` or `name` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Workspace name. Names are unique. Exactly one of `id` or `name` must be set.",
			},
			"scope": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Lifetime and visibility of the workspace. One of `GLOBAL` or `CONVERSATION`.",
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *WorkspaceDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *WorkspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var details *pgql.WorkspaceDetails
	if !data.Id.IsNull() {
		logDataSourceOperation(ctx, "reading", "workspace", data.Id.ValueString())

		getResp, err := pgql.GetWorkspace(ctx, d.GetClient(), data.Id.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "workspace", data.Id.ValueString())
			return
		}
		details = getResp.Workspace
		if details == nil {
			resp.Diagnostics.AddError(
				"Workspace Not Found",
				fmt.Sprintf("no workspace with ID %q", data.Id.ValueString()),
			)
			return
		}
	} else {
		logDataSourceOperation(ctx, "reading", "workspace", data.Name.ValueString())

		getResp, err := pgql.GetWorkspaceByName(ctx, d.GetClient(), data.Name.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "workspace", data.Name.ValueString())
			return
		}
		details = getResp.WorkspaceByName
		if details == nil {
			resp.Diagnostics.AddError(
				"Workspace Not Found",
				fmt.Sprintf("no workspace named %q", data.Name.ValueString()),
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
