package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TaskDefinitionResource{}
var _ resource.ResourceWithImportState = &TaskDefinitionResource{}

func NewTaskDefinitionResource() resource.Resource {
	return &TaskDefinitionResource{
		BaseResource: NewBaseResource("task_definition"),
	}
}

// TaskDefinitionResource defines the resource implementation.
type TaskDefinitionResource struct {
	BaseResource
}

// TaskDefinitionResourceModel extends the base model with resource-specific fields.
type TaskDefinitionResourceModel struct {
	TaskDefinitionBaseModel
}

func (r *TaskDefinitionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An author-defined task definition describing a task an agent can send to the user as a tool call. " +
			"Carries an input schema (what the agent supplies) and an output schema (what the user submits when completing it). " +
			"One task definition is reusable across agents via the agent resource's `tasks` attribute.",
		Attributes: map[string]schema.Attribute{
			"id":   commonSchemas.id,
			"name": commonSchemas.name,
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the task. Shown to the LLM as the tool description.",
				Required:            true,
			},
			"input_message_type": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "JSON schema the agent must satisfy when invoking the task",
				Required:            true,
			},
			"output_message_type": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "JSON schema the user must satisfy when completing the task",
				Required:            true,
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *TaskDefinitionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TaskDefinitionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "task_definition", "")

	createResp, err := pgql.CreateTaskDefinition(ctx, r.GetClient(), &pgql.NewTaskDefinition{
		Name:              data.Name.ValueString(),
		Description:       data.Description.ValueString(),
		InputMessageType:  data.InputMessageType.ValueString(),
		OutputMessageType: data.OutputMessageType.ValueString(),
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create task_definition")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &createResp.CreateTaskDefinition)...)

	logResourceOperation(ctx, "created", "task_definition", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaskDefinitionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TaskDefinitionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "task_definition", data.Id.ValueString())

	getResp, err := pgql.GetTaskDefinition(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "task_definition", data.Id.ValueString())
		return
	}

	if getResp.TaskDefinition == nil {
		// Deleted out-of-band: drop from state so Terraform plans a re-create.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, getResp.TaskDefinition)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaskDefinitionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData TaskDefinitionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "task_definition", planData.Id.ValueString())

	var update pgql.UpdateTaskDefinition
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.Opt(planData.Description.ValueString())
	}
	if !planData.InputMessageType.Equal(stateData.InputMessageType) {
		update.InputMessageType = pgql.Opt(planData.InputMessageType.ValueString())
	}
	if !planData.OutputMessageType.Equal(stateData.OutputMessageType) {
		update.OutputMessageType = pgql.Opt(planData.OutputMessageType.ValueString())
	}

	updateResp, err := pgql.ModifyTaskDefinition(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "task_definition", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, &updateResp.UpdateTaskDefinition)...)

	logResourceOperation(ctx, "updated", "task_definition", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *TaskDefinitionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TaskDefinitionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "task_definition", data.Id.ValueString())

	// Deliberate decision: API errors (e.g. "still associated with an agent")
	// pass through unchanged via handleAPIErrorWithDetails. No special-casing.
	_, err := pgql.DeleteTaskDefinition(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "task_definition", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "task_definition", data.Id.ValueString())
}

func (r *TaskDefinitionResource) mapResponse(ctx context.Context, data *TaskDefinitionResourceModel, td *pgql.TaskDefinitionDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, td)
}
