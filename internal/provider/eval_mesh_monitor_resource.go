package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EvalMeshMonitorResource{}
var _ resource.ResourceWithImportState = &EvalMeshMonitorResource{}

func NewEvalMeshMonitorResource() resource.Resource {
	return &EvalMeshMonitorResource{
		BaseResource: NewBaseResource("eval_mesh_monitor"),
	}
}

// EvalMeshMonitorResource defines the resource implementation.
type EvalMeshMonitorResource struct {
	BaseResource
}

// EvalMeshMonitorResourceModel extends the base model with resource-specific fields.
type EvalMeshMonitorResourceModel struct {
	EvalMeshMonitorBaseModel
}

func (r *EvalMeshMonitorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Scores a share of a mesh's conversations against a rubric as they happen. " +
			"Requires an enterprise deployment.",
		Attributes: map[string]schema.Attribute{
			"id": commonSchemas.id,
			"mesh_id": schema.StringAttribute{
				MarkdownDescription: "Mesh whose conversations are scored.",
				Required:            true,
				// The API can move a monitor's model set but not the mesh or
				// rubric it was made for.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"rubric_id": schema.StringAttribute{
				MarkdownDescription: "Rubric the conversations are scored against.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"model_set_id": schema.StringAttribute{
				MarkdownDescription: "Model set the judging runs on.",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the monitor is scoring. A disabled monitor keeps what it has already written.",
				Required:            true,
			},
			"sample_rate": schema.Float64Attribute{
				MarkdownDescription: "Share of conversations to score, from 0 to 1.",
				Required:            true,
				Validators: []validator.Float64{
					float64validator.AtLeast(0),
					float64validator.AtMost(1),
				},
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *EvalMeshMonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data EvalMeshMonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "eval_mesh_monitor", "")

	createResp, err := pgql.CreateEvalMeshMonitor(ctx, r.GetClient(), pgql.NewEvalMeshMonitor{
		MeshID:     data.MeshId.ValueString(),
		RubricID:   data.RubricId.ValueString(),
		ModelSetID: data.ModelSetId.ValueString(),
		Enabled:    data.Enabled.ValueBool(),
		SampleRate: data.SampleRate.ValueFloat64(),
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create eval_mesh_monitor")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, &createResp.CreateEvalMeshMonitor)...)

	logResourceOperation(ctx, "created", "eval_mesh_monitor", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EvalMeshMonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data EvalMeshMonitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "eval_mesh_monitor", data.Id.ValueString())

	getResp, err := pgql.GetEvalMeshMonitor(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "eval_mesh_monitor", data.Id.ValueString())
		return
	}
	if getResp.EvalMeshMonitor == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, getResp.EvalMeshMonitor)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EvalMeshMonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var planData EvalMeshMonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "eval_mesh_monitor", planData.Id.ValueString())

	// Every field goes on every update: the configuration is what the monitor
	// should be. All three are required, so none of them is ever null.
	update := pgql.UpdateEvalMeshMonitor{
		ModelSetID: planData.ModelSetId.ValueStringPointer(),
		Enabled:    planData.Enabled.ValueBoolPointer(),
		SampleRate: planData.SampleRate.ValueFloat64Pointer(),
	}

	updateResp, err := pgql.ModifyEvalMeshMonitor(ctx, r.GetClient(), planData.Id.ValueString(), update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "eval_mesh_monitor", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, &updateResp.UpdateEvalMeshMonitor)...)

	logResourceOperation(ctx, "updated", "eval_mesh_monitor", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *EvalMeshMonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.RequireEnterprise(&resp.Diagnostics) {
		return
	}

	var data EvalMeshMonitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "eval_mesh_monitor", data.Id.ValueString())

	if _, err := pgql.DeleteEvalMeshMonitor(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "eval_mesh_monitor", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "eval_mesh_monitor", data.Id.ValueString())
}
