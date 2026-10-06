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
var _ resource.Resource = &JudgeMetricResource{}
var _ resource.ResourceWithImportState = &JudgeMetricResource{}

func NewJudgeMetricResource() resource.Resource {
	return &JudgeMetricResource{
		BaseResource: NewBaseResource("judge_metric"),
	}
}

// JudgeMetricResource defines the resource implementation.
type JudgeMetricResource struct {
	BaseResource
}

// JudgeMetricResourceModel extends the base model with resource-specific fields.
type JudgeMetricResourceModel struct {
	JudgeMetricBaseModel
}

func (r *JudgeMetricResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A metric a model judges a conversation against. Put one in a rubric to have " +
			"it scored. Every organization starts with a set of built-in judge metrics, which can be edited " +
			"like any other; find one with the `partiteai_judge_metric` data source. Requires an enterprise " +
			"deployment.",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"category": schema.StringAttribute{
				MarkdownDescription: "What the metric measures. One of `QUALITY`, `SAFETY`, `SECURITY`, " +
					"`RELIABILITY` or `EFFICIENCY`.",
				Required:   true,
				Validators: []validator.String{stringvalidator.OneOf(metricCategories...)},
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "What the model judges: the whole request (`REQUEST`) or each hop between " +
					"agents (`HOP`).",
				Required:   true,
				Validators: []validator.String{stringvalidator.OneOf(judgeScopes...)},
			},
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

func (r *JudgeMetricResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data JudgeMetricResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	examples, inputFields, ok := r.readJudgeConfig(ctx, data, &resp.Diagnostics)
	if !ok {
		return
	}

	logResourceOperation(ctx, "creating", "judge_metric", "")

	createResp, err := pgql.CreateJudgeMetric(ctx, r.GetClient(), pgql.NewJudgeMetric{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueStringPointer(),
		Category:    pgql.MetricCategory(data.Category.ValueString()),
		Scope:       pgql.JudgeScope(data.Scope.ValueString()),
		Prompt:      data.Prompt.ValueString(),
		Complexity:  pgql.ModelComplexity(data.Complexity.ValueString()),
		Examples:    examples,
		InputFields: inputFields,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create judge_metric")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, &createResp.CreateJudgeMetric)...)

	logResourceOperation(ctx, "created", "judge_metric", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *JudgeMetricResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data JudgeMetricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "judge_metric", data.Id.ValueString())

	getResp, err := pgql.GetJudgeMetric(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "judge_metric", data.Id.ValueString())
		return
	}
	if getResp.JudgeMetric == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, getResp.JudgeMetric)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *JudgeMetricResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var planData JudgeMetricResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "judge_metric", planData.Id.ValueString())

	examples, inputFields, ok := r.readJudgeConfig(ctx, planData, &resp.Diagnostics)
	if !ok {
		return
	}

	// The whole metric goes every time, nulls and all: the API leaves alone
	// what it is not told about, so sending only what changed could never
	// clear a field the configuration dropped.
	complexity := pgql.ModelComplexity(planData.Complexity.ValueString())
	category := pgql.MetricCategory(planData.Category.ValueString())
	scope := pgql.JudgeScope(planData.Scope.ValueString())
	update := pgql.UpdateJudgeMetric{
		Name:        planData.Name.ValueStringPointer(),
		Description: planData.Description.ValueStringPointer(),
		Category:    &category,
		Scope:       &scope,
		Prompt:      planData.Prompt.ValueStringPointer(),
		Complexity:  &complexity,
		Examples:    examples,
		InputFields: inputFields,
	}

	updateResp, err := pgql.ModifyJudgeMetric(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "judge_metric", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, &updateResp.UpdateJudgeMetric)...)

	logResourceOperation(ctx, "updated", "judge_metric", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *JudgeMetricResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data JudgeMetricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "judge_metric", data.Id.ValueString())

	if _, err := pgql.DeleteJudgeMetric(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "judge_metric", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "judge_metric", data.Id.ValueString())
}

// readJudgeConfig reads the two list attributes both create and update send.
func (r *JudgeMetricResource) readJudgeConfig(
	ctx context.Context,
	data JudgeMetricResourceModel,
	diags *diag.Diagnostics,
) ([]pgql.NewScoreExample, []pgql.NewJudgeInputField, bool) {
	examples, d := scoreExampleInputs(ctx, data.Examples)
	diags.Append(d...)
	inputFields, d := judgeInputFieldInputs(ctx, data.InputFields)
	diags.Append(d...)
	return examples, inputFields, !diags.HasError()
}
