package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ClientTaskDefinitionResource{}
var _ resource.ResourceWithImportState = &ClientTaskDefinitionResource{}
var _ resource.ResourceWithMoveState = &ClientTaskDefinitionResource{}

func NewClientTaskDefinitionResource() resource.Resource {
	return &ClientTaskDefinitionResource{
		BaseResource: NewBaseResource("client_task_definition"),
	}
}

// ClientTaskDefinitionResource defines the resource implementation.
type ClientTaskDefinitionResource struct {
	BaseResource
}

// ClientTaskDefinitionResourceModel extends the base model with resource-specific fields.
type ClientTaskDefinitionResourceModel struct {
	ClientTaskDefinitionBaseModel
	OverwriteExisting types.Bool `tfsdk:"overwrite_existing"`
}

func (r *ClientTaskDefinitionResource) MoveState(ctx context.Context) []resource.StateMover {
	// This resource was partiteai_task_definition, with the same schema, so a
	// moved block from the old name keeps its state as it is.
	var source resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &source)
	return []resource.StateMover{{
		SourceSchema: &source.Schema,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
			if req.SourceTypeName != "partiteai_task_definition" || req.SourceState == nil {
				return
			}
			resp.TargetState.Raw = req.SourceState.Raw
		},
	}}
}

func (r *ClientTaskDefinitionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An author-defined client task definition describing a task an agent can send to the user as a tool call. " +
			"Carries an input schema (what the agent supplies) and an output schema (what the user submits when completing it). " +
			"One client task definition is reusable across agents via the agent resource's `client_tasks` attribute.",
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
			"ui_forms": schema.MapAttribute{
				MarkdownDescription: "UIs a client can show to complete the task, keyed by form type, with the form's " +
					"definition as the value. The only type is `A2UI`, whose definition is JSON: an A2UI v0.9 surface " +
					"for each state of the task, such as `jsonencode({ open = { components = [...] } })`. " +
					"The API checks each form against the input and output schemas. A change replaces all of the task's forms.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
				Validators: []validator.Map{
					mapvalidator.KeysAre(stringvalidator.OneOf(clientTaskUIFormTypes()...)),
				},
			},
			"overwrite_existing": schema.BoolAttribute{
				MarkdownDescription: "When true and a client task definition with this `name` already exists, create takes it over " +
					"and overwrites its description, schemas and UI forms instead of failing on the duplicate name. " +
					"Use it to apply a configuration to an organization that already has the definition, " +
					"such as one of the client task definitions every organization starts with. " +
					"Only read when the resource is created, and never stored in state, so changing it later does nothing. " +
					"Defaults to false.",
				Optional:  true,
				WriteOnly: true,
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *ClientTaskDefinitionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClientTaskDefinitionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Write-only, so the plan holds null and only the config has the value.
	var overwriteExisting types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("overwrite_existing"), &overwriteExisting)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "client_task_definition", "")

	uiForms, diags := clientTaskUIFormInputs(ctx, data.UIForms)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := &pgql.NewClientTaskDefinition{
		Name:              data.Name.ValueString(),
		Description:       data.Description.ValueString(),
		InputMessageType:  data.InputMessageType.ValueString(),
		OutputMessageType: data.OutputMessageType.ValueString(),
		UIForms:           uiForms,
	}
	if overwriteExisting.ValueBool() {
		input.OverwriteExisting = overwriteExisting.ValueBoolPointer()
	}
	createResp, err := pgql.CreateClientTaskDefinition(ctx, r.GetClient(), input)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create client_task_definition")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &createResp.CreateClientTaskDefinition)...)

	logResourceOperation(ctx, "created", "client_task_definition", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClientTaskDefinitionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClientTaskDefinitionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "client_task_definition", data.Id.ValueString())

	getResp, err := pgql.GetClientTaskDefinition(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "client_task_definition", data.Id.ValueString())
		return
	}

	if getResp.ClientTaskDefinition == nil {
		// Deleted out-of-band: drop from state so Terraform plans a re-create.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, getResp.ClientTaskDefinition)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClientTaskDefinitionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData ClientTaskDefinitionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "client_task_definition", planData.Id.ValueString())

	var update pgql.UpdateClientTaskDefinition
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
	if !planData.UIForms.Equal(stateData.UIForms) {
		uiForms, diags := clientTaskUIFormInputs(ctx, planData.UIForms)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		update.UIForms = pgql.Opt(uiForms)
	}

	updateResp, err := pgql.ModifyClientTaskDefinition(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "client_task_definition", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, &updateResp.UpdateClientTaskDefinition)...)

	logResourceOperation(ctx, "updated", "client_task_definition", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ClientTaskDefinitionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClientTaskDefinitionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "client_task_definition", data.Id.ValueString())

	// Deliberate decision: API errors (e.g. "still associated with an agent")
	// pass through unchanged via handleAPIErrorWithDetails. No special-casing.
	_, err := pgql.DeleteClientTaskDefinition(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "client_task_definition", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "client_task_definition", data.Id.ValueString())
}

func (r *ClientTaskDefinitionResource) mapResponse(ctx context.Context, data *ClientTaskDefinitionResourceModel, td *pgql.ClientTaskDefinitionDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, td)
}
