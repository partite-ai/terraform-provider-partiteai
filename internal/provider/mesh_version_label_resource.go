package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &MeshVersionLabelResource{}
var _ resource.ResourceWithImportState = &MeshVersionLabelResource{}

func NewMeshVersionLabelResource() resource.Resource {
	return &MeshVersionLabelResource{
		BaseResource: NewBaseResource("mesh_version_label"),
	}
}

type MeshVersionLabelResource struct {
	BaseResource
}

type MeshVersionLabelResourceModel struct {
	MeshVersionLabelBaseModel
}

func (r *MeshVersionLabelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Labels a mesh version. Creating the resource freezes the mesh's current draft under `label`, and the mesh gets a new draft with the same configuration. The resource then follows that version by its ID.\n\n" +
			"* Changing `label` renames that version. It does not freeze a new version. To freeze a new version, add a resource at a new address.\n" +
			"* To freeze the draft under a new label and remove the old label, replace the resource block with a block at a new address in one apply. The old version stays frozen.\n" +
			"* Destroying the resource removes the label. The version stays frozen, with its conversations and evaluation runs. If you apply the same configuration again, the resource freezes the current draft. It does not label the old version again. To keep the label when you remove the resource, use a `removed` block that has `lifecycle { destroy = false }`.\n" +
			"* Labeling must happen after the configuration that it freezes. Add `depends_on` for the mesh's `partiteai_mesh_input` and `partiteai_mesh_link` resources, and for anything else that changes the draft.\n" +
			"* A frozen version keeps references to the agent versions it uses, so an agent cannot be deleted while a frozen version of its mesh exists. Add `depends_on` for the mesh's agents to the `partiteai_mesh` resource, so that a destroy deletes the mesh first.\n" +
			"* If one apply adds two label resources for the same mesh, add `depends_on` from one to the other. Without it, Terraform can create both at the same time, and one of them can fail with a conflict. Both freeze the same configuration.\n" +
			"* A webhook destination or a schedule that names a removed or renamed label fails on its next use.",
		Attributes: map[string]schema.Attribute{
			"mesh_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the labeled mesh version.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the mesh. Changing it replaces the resource.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"label": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The label of the mesh version. Changing it renames the version.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"ordinal": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The ordinal of the labeled mesh version.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *MeshVersionLabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MeshVersionLabelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "mesh_version_label", "")

	labelResp, err := pgql.LabelMeshDraft(ctx, r.GetClient(), data.MeshId.ValueString(), data.Label.ValueString())
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create mesh version label")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, labelResp.LabelMeshDraft)...)
	logResourceOperation(ctx, "created", "mesh_version_label", data.MeshVersionId.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshVersionLabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeshVersionLabelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "mesh_version_label", data.MeshVersionId.ValueString())

	versionResp, err := pgql.GetMeshVersion(ctx, r.GetClient(), data.MeshVersionId.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh_version_label", data.MeshVersionId.ValueString())
		return
	}
	if versionResp.MeshVersion == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *versionResp.MeshVersion)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshVersionLabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state MeshVersionLabelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "mesh_version_label", state.MeshVersionId.ValueString())

	labelResp, err := pgql.LabelMeshVersion(ctx, r.GetClient(), state.MeshVersionId.ValueString(), plan.Label.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "mesh_version_label", state.MeshVersionId.ValueString())
		return
	}

	resp.Diagnostics.Append(plan.MapFromGraphQL(ctx, labelResp.LabelMeshVersion)...)
	logResourceOperation(ctx, "updated", "mesh_version_label", plan.MeshVersionId.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MeshVersionLabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeshVersionLabelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "mesh_version_label", data.MeshVersionId.ValueString())

	if _, err := pgql.UnlabelMeshVersion(ctx, r.GetClient(), data.MeshVersionId.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "mesh_version_label", data.MeshVersionId.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "mesh_version_label", data.MeshVersionId.ValueString())
}

func (r *MeshVersionLabelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("mesh_version_id"), req, resp)
}
