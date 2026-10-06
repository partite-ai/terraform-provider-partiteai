package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SkillResource{}
var _ resource.ResourceWithImportState = &SkillResource{}

func NewSkillResource() resource.Resource {
	return &SkillResource{
		BaseResource: NewBaseResource("skill"),
	}
}

// SkillResource defines the resource implementation.
type SkillResource struct {
	BaseResource
}

// SkillResourceModel extends the base model with resource-specific fields.
type SkillResourceModel struct {
	SkillBaseModel
}

func (r *SkillResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A skill: instructions an agent loads when a task calls for them, with the files those " +
			"instructions refer to. The skill is its folder of files, `SKILL.md` among them; the front matter of " +
			"`SKILL.md` gives the name the agent knows the skill by and its description. Agents use a skill through " +
			"the agent resource's `skills` attribute. Skills with a `scripts` folder or with files that are not text " +
			"are refused.",
		Attributes: map[string]schema.Attribute{
			"id": commonSchemas.id,
			"name": schema.StringAttribute{
				MarkdownDescription: "The skill's own name, unique across skills. Two skills can come from folders " +
					"that declare the same name, such as two copies of one skill.",
				Required: true,
			},
			"files": schema.MapAttribute{
				MarkdownDescription: "The files of the skill folder, by path, as text. Paths are relative to the " +
					"folder, such as `SKILL.md` or `reference/forms.md`. Must include `SKILL.md`. A change replaces " +
					"all of the skill's files.",
				ElementType: types.StringType,
				Required:    true,
				Validators:  []validator.Map{skillFilePathsAreClean},
			},
			"declared_name": schema.StringAttribute{
				MarkdownDescription: "The name `SKILL.md` declares. The agent knows the skill by it, so an agent " +
					"can't use two skills with the same declared name.",
				Computed: true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description `SKILL.md` declares. It tells the agent when to use the skill.",
				Computed:            true,
			},
			"instructions": schema.StringAttribute{
				MarkdownDescription: "The body of `SKILL.md`, after its front matter.",
				Computed:            true,
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *SkillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SkillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "skill", "")

	files, diags := skillSourceFiles(ctx, data.Files)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := pgql.CreateSkill(ctx, r.GetClient(), &pgql.NewSkill{
		Name:   data.Name.ValueString(),
		Source: pgql.SkillSource{Files: files},
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create skill")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &createResp.CreateSkill)...)

	logResourceOperation(ctx, "created", "skill", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SkillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "skill", data.Id.ValueString())

	getResp, err := pgql.GetSkill(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "skill", data.Id.ValueString())
		return
	}

	if getResp.Skill == nil {
		// Deleted out-of-band: drop from state so Terraform plans a re-create.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, getResp.Skill)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SkillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData SkillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "skill", planData.Id.ValueString())

	var update pgql.UpdateSkill
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Files.Equal(stateData.Files) {
		files, diags := skillSourceFiles(ctx, planData.Files)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		update.Source = pgql.Opt(pgql.SkillSource{Files: files})
	}

	updateResp, err := pgql.ModifySkill(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "skill", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, &updateResp.UpdateSkill)...)

	logResourceOperation(ctx, "updated", "skill", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *SkillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "skill", data.Id.ValueString())

	// A skill an agent still uses can't be deleted; the API error says so.
	_, err := pgql.DeleteSkill(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "skill", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "skill", data.Id.ValueString())
}

func (r *SkillResource) mapResponse(ctx context.Context, data *SkillResourceModel, s *pgql.SkillDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, s)
}
