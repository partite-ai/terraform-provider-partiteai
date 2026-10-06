package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RubricResource{}
var _ resource.ResourceWithImportState = &RubricResource{}

func NewRubricResource() resource.Resource {
	return &RubricResource{
		BaseResource: NewBaseResource("rubric"),
	}
}

// RubricResource defines the resource implementation.
type RubricResource struct {
	BaseResource
}

// RubricResourceModel extends the base model with resource-specific fields.
type RubricResourceModel struct {
	RubricBaseModel
}

var rubricMetricResourceObject = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"judge_metric_id": schema.StringAttribute{
			MarkdownDescription: "A metric a model judges. Give this or `computed_metric_key`, not both.",
			Optional:            true,
			Validators: []validator.String{
				stringvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("computed_metric_key"),
				),
			},
		},
		"computed_metric_key": schema.StringAttribute{
			MarkdownDescription: "A metric the product computes, such as latency, by key.",
			Optional:            true,
		},
		// The API fills trial_count in whether or not the configuration names
		// it, so it carries that value as a default. A default keeps it known
		// while planning, which is what lets the metrics be a set: an element
		// whose value is not yet known cannot be matched against the one
		// already there. Everything else comes back null when it is not given.
		"trial_count": schema.Int64Attribute{
			MarkdownDescription: "How many times a judge metric is scored, to even out disagreement. " +
				"Only a judge metric takes it; a computed metric reads back 1.",
			Optional: true,
			Computed: true,
			Default:  int64default.StaticInt64(1),
			Validators: []validator.Int64{
				int64validator.AtLeast(1),
				int64validator.ConflictsWith(path.MatchRelative().AtParent().AtName("computed_metric_key")),
			},
		},
		"input_values": schema.StringAttribute{
			CustomType: jsontypes.NormalizedType{},
			MarkdownDescription: "Values for the metric's input fields, as a JSON object. " +
				"Only a judge metric takes them.",
			Optional: true,
			Validators: []validator.String{
				stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("computed_metric_key")),
				inputValuesNotEmpty{},
			},
		},
		"complexity": schema.StringAttribute{
			MarkdownDescription: "Model complexity this metric judges at, overriding the metric's own. " +
				"Only a judge metric takes it.",
			Optional: true,
			Validators: []validator.String{
				stringvalidator.OneOf("LOW", "MEDIUM", "HIGH"),
				stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("computed_metric_key")),
			},
		},
		"score_floor": schema.Float64Attribute{
			MarkdownDescription: "Lowest score a computed metric can report. Only a computed metric takes it.",
			Optional:            true,
			Validators: []validator.Float64{
				float64validator.ConflictsWith(path.MatchRelative().AtParent().AtName("judge_metric_id")),
			},
		},
		"score_ceiling": schema.Float64Attribute{
			MarkdownDescription: "Highest score a computed metric can report. Only a computed metric takes it.",
			Optional:            true,
			Validators: []validator.Float64{
				float64validator.ConflictsWith(path.MatchRelative().AtParent().AtName("judge_metric_id")),
			},
		},
		"lower_is_better": schema.BoolAttribute{
			MarkdownDescription: "Whether a lower score is the better one. Only a computed metric takes it.",
			Optional:            true,
			Validators: []validator.Bool{
				boolvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("judge_metric_id")),
			},
		},
		"weight": schema.Float64Attribute{
			MarkdownDescription: "How much this metric counts towards the rubric's score.",
			Optional:            true,
		},
	},
}

func (r *RubricResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A set of metrics a conversation is scored against. Attach one to a mesh with " +
			"`partiteai_eval_mesh_monitor`. Requires an enterprise deployment.",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"metrics": schema.SetNestedAttribute{
				MarkdownDescription: "The metrics this rubric scores against. At least one is required. " +
					"They are a set: the API holds no order for them.",
				Required:     true,
				NestedObject: rubricMetricResourceObject,
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *RubricResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data RubricResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entries, diags := rubricMetricEntries(ctx, data.Metrics)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	metrics := make([]pgql.NewRubricMetric, 0, len(entries))
	for _, entry := range entries {
		metrics = append(metrics, newRubricMetric(entry))
	}

	logResourceOperation(ctx, "creating", "rubric", "")

	createResp, err := pgql.CreateRubric(ctx, r.GetClient(), pgql.NewRubric{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueStringPointer(),
		Metrics:     metrics,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create rubric")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, &createResp.CreateRubric)...)

	logResourceOperation(ctx, "created", "rubric", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RubricResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data RubricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "rubric", data.Id.ValueString())

	getResp, err := pgql.GetRubric(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "rubric", data.Id.ValueString())
		return
	}
	if getResp.Rubric == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, getResp.Rubric)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RubricResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var planData RubricResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planEntries, diags := rubricMetricEntries(ctx, planData.Metrics)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API takes the difference rather than the whole list and names each
	// entry by an id of its own, so the update reads what the rubric holds now
	// and matches it up by the metric each entry names.
	current, err := pgql.GetRubric(ctx, r.GetClient(), planData.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "rubric", planData.Id.ValueString())
		return
	}
	if current.Rubric == nil {
		resp.Diagnostics.AddError(
			"Rubric Not Found",
			"the rubric was deleted outside Terraform between the plan and the apply",
		)
		return
	}

	held := rubricEntryIDs(current.Rubric, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	update := pgql.UpdateRubric{
		Name:             planData.Name.ValueStringPointer(),
		Description:      planData.Description.ValueStringPointer(),
		DeletedMetricIDs: []string{},
		UpdatedMetrics:   []pgql.UpdateRubricMetric{},
		NewMetrics:       []pgql.NewRubricMetric{},
	}

	wanted := make(map[string]bool, len(planEntries))
	for _, entry := range planEntries {
		wanted[entry.key()] = true
		id, ok := held[entry.key()]
		if !ok {
			update.NewMetrics = append(update.NewMetrics, newRubricMetric(entry))
			continue
		}
		update.UpdatedMetrics = append(update.UpdatedMetrics, updateRubricMetric(id, entry))
	}

	for key, id := range held {
		if !wanted[key] {
			update.DeletedMetricIDs = append(update.DeletedMetricIDs, id)
		}
	}

	logResourceOperation(ctx, "updating", "rubric", planData.Id.ValueString())

	updateResp, err := pgql.ModifyRubric(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "rubric", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, &updateResp.UpdateRubric)...)

	logResourceOperation(ctx, "updated", "rubric", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *RubricResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data RubricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "rubric", data.Id.ValueString())

	if _, err := pgql.DeleteRubric(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "rubric", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "rubric", data.Id.ValueString())
}
