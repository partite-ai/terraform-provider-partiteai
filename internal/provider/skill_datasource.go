package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SkillDataSource{}
var _ datasource.DataSourceWithConfigValidators = &SkillDataSource{}

func NewSkillDataSource() datasource.DataSource {
	return &SkillDataSource{
		BaseDataSource: NewBaseDataSource("skill"),
	}
}

// SkillDataSource defines the data source implementation.
type SkillDataSource struct {
	BaseDataSource
}

// SkillDataSourceModel describes the data source data model.
type SkillDataSourceModel struct {
	SkillBaseModel
}

func (d *SkillDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Skill data source. Look up a skill by `id` or by `name` (exactly one must be set).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Skill identifier. Exactly one of `id` or `name` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The skill's own name. Exactly one of `id` or `name` must be set.",
			},
			"declared_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name `SKILL.md` declares, which the agent knows the skill by",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The description `SKILL.md` declares",
			},
			"instructions": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The body of `SKILL.md`, after its front matter",
			},
			"files": schema.MapAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "The files of the skill folder, by path, `SKILL.md` among them",
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *SkillDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *SkillDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SkillDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var details *pgql.SkillDetails
	if !data.Id.IsNull() {
		logDataSourceOperation(ctx, "reading", "skill", data.Id.ValueString())

		getResp, err := pgql.GetSkill(ctx, d.GetClient(), data.Id.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "skill", data.Id.ValueString())
			return
		}
		details = getResp.Skill
		if details == nil {
			resp.Diagnostics.AddError("Skill Not Found", fmt.Sprintf("no skill with ID %q", data.Id.ValueString()))
			return
		}
	} else {
		logDataSourceOperation(ctx, "reading", "skill", data.Name.ValueString())

		getResp, err := pgql.GetSkillByName(ctx, d.GetClient(), data.Name.ValueString())
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "skill", data.Name.ValueString())
			return
		}
		details = getResp.SkillByName
		if details == nil {
			resp.Diagnostics.AddError("Skill Not Found", fmt.Sprintf("no skill named %q", data.Name.ValueString()))
			return
		}
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, details)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
