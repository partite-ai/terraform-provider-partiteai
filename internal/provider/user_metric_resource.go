package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &UserMetricResource{}
var _ resource.ResourceWithImportState = &UserMetricResource{}

func NewUserMetricResource() resource.Resource {
	return &UserMetricResource{
		BaseResource: NewBaseResource("user_metric"),
	}
}

// UserMetricResource defines the resource implementation.
type UserMetricResource struct {
	BaseResource
}

// UserMetricResourceModel extends the base model with resource-specific fields.
type UserMetricResourceModel struct {
	UserMetricBaseModel
}

func (r *UserMetricResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A metric of your own for a model to judge a conversation against. " +
			"Put one in a rubric to have it scored. Requires an enterprise deployment.",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"prompt": schema.StringAttribute{
				MarkdownDescription: "What the judging model is asked.",
				Required:            true,
			},
			"complexity": schema.StringAttribute{
				MarkdownDescription: "Model complexity to judge at. One of `LOW`, `MEDIUM` or `HIGH`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("LOW", "MEDIUM", "HIGH"),
				},
			},
			"examples": schema.ListNestedAttribute{
				MarkdownDescription: "Worked examples shown to the judging model, to anchor its scoring.",
				Optional:            true,
				Computed:            true,
				NestedObject:        scoreExampleResourceObject,
			},
			"input_fields": schema.ListNestedAttribute{
				MarkdownDescription: "Values a rubric supplies when it uses this metric.",
				Optional:            true,
				Computed:            true,
				NestedObject:        judgeInputFieldResourceObject,
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *UserMetricResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data UserMetricResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	examples, inputFields, ok := r.readJudgeConfig(ctx, data, &resp.Diagnostics)
	if !ok {
		return
	}

	logResourceOperation(ctx, "creating", "user_metric", "")

	createResp, err := pgql.CreateUserMetric(ctx, r.GetClient(), pgql.NewUserMetric{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueStringPointer(),
		Prompt:      data.Prompt.ValueString(),
		Complexity:  pgql.ModelComplexity(data.Complexity.ValueString()),
		Examples:    examples,
		InputFields: inputFields,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create user_metric")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, &createResp.CreateUserMetric)...)

	logResourceOperation(ctx, "created", "user_metric", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserMetricResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data UserMetricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "user_metric", data.Id.ValueString())

	getResp, err := pgql.GetUserMetric(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "user_metric", data.Id.ValueString())
		return
	}
	if getResp.UserMetric == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, getResp.UserMetric)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserMetricResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var planData UserMetricResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "user_metric", planData.Id.ValueString())

	examples, inputFields, ok := r.readJudgeConfig(ctx, planData, &resp.Diagnostics)
	if !ok {
		return
	}

	// The whole metric goes every time, nulls and all: the API leaves alone
	// what it is not told about, so sending only what changed could never
	// clear a field the configuration dropped.
	complexity := pgql.ModelComplexity(planData.Complexity.ValueString())
	update := pgql.UpdateUserMetric{
		Name:        planData.Name.ValueStringPointer(),
		Description: planData.Description.ValueStringPointer(),
		Prompt:      planData.Prompt.ValueStringPointer(),
		Complexity:  &complexity,
		Examples:    examples,
		InputFields: inputFields,
	}

	updateResp, err := pgql.ModifyUserMetric(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "user_metric", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, &updateResp.UpdateUserMetric)...)

	logResourceOperation(ctx, "updated", "user_metric", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *UserMetricResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data UserMetricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "user_metric", data.Id.ValueString())

	if _, err := pgql.DeleteUserMetric(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "user_metric", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "user_metric", data.Id.ValueString())
}

// readJudgeConfig reads the two list attributes both create and update send.
func (r *UserMetricResource) readJudgeConfig(
	ctx context.Context,
	data UserMetricResourceModel,
	diags *diag.Diagnostics,
) ([]pgql.NewScoreExample, []pgql.NewJudgeInputField, bool) {
	examples, d := scoreExampleInputs(ctx, data.Examples)
	diags.Append(d...)
	inputFields, d := judgeInputFieldInputs(ctx, data.InputFields)
	diags.Append(d...)
	return examples, inputFields, !diags.HasError()
}
