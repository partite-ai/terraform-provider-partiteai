package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &WorkspaceResource{}
var _ resource.ResourceWithImportState = &WorkspaceResource{}

func NewWorkspaceResource() resource.Resource {
	return &WorkspaceResource{
		BaseResource: NewBaseResource("workspace"),
	}
}

// WorkspaceResource defines the resource implementation.
type WorkspaceResource struct {
	BaseResource
}

// WorkspaceResourceModel extends the base model with resource-specific fields.
type WorkspaceResourceModel struct {
	WorkspaceBaseModel
}

func (r *WorkspaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A workspace holding key-value state and files for an installed particle. " +
			"Bind one to an install through the installed particle resource's `kv_workspace_id` or `mounts` attributes.",
		Attributes: map[string]schema.Attribute{
			"id":   commonSchemas.id,
			"name": commonSchemas.name,
			"scope": schema.StringAttribute{
				MarkdownDescription: "Lifetime and visibility of the workspace. `GLOBAL` state is shared across every " +
					"agent and conversation wired to it. `CONVERSATION` makes the workspace a template: each " +
					"conversation that uses it sees state of its own.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(workspaceScopeGlobal, workspaceScopeConversation),
				},
				// Scope is fixed once the workspace exists: the API can rename
				// a workspace but not re-scope one.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *WorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "workspace", "")

	createResp, err := pgql.CreateWorkspace(ctx, r.GetClient(), pgql.NewWorkspace{
		Name:  data.Name.ValueString(),
		Scope: pgql.WorkspaceScope(data.Scope.ValueString()),
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create workspace")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &createResp.CreateWorkspace)...)

	logResourceOperation(ctx, "created", "workspace", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "workspace", data.Id.ValueString())

	getResp, err := pgql.GetWorkspace(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "workspace", data.Id.ValueString())
		return
	}

	if getResp.Workspace == nil {
		// Deleted out-of-band: drop from state so Terraform plans a re-create.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, getResp.Workspace)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData WorkspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "workspace", planData.Id.ValueString())

	// A rename is the only change the API accepts; a new scope forces a
	// replacement instead, so there is nothing else to send here.
	if planData.Name.Equal(stateData.Name) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
		return
	}

	renameResp, err := pgql.RenameWorkspace(ctx, r.GetClient(),
		planData.Id.ValueString(), planData.Name.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "workspace", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, &renameResp.RenameWorkspace)...)

	logResourceOperation(ctx, "updated", "workspace", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *WorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "workspace", data.Id.ValueString())

	_, err := pgql.DeleteWorkspace(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "workspace", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "workspace", data.Id.ValueString())
}

func (r *WorkspaceResource) mapResponse(ctx context.Context, data *WorkspaceResourceModel, ws *pgql.WorkspaceDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, ws)
}
