package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

func (r *MeshVersionLabelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Labels a mesh version. Creating the resource freezes the mesh's current draft under `label`, and the mesh gets a new draft with the same configuration. The resource then follows that version by its ID.\n\n" +
			"* A change to `label` renames that version. It does not freeze a new version. To freeze a new version, add a resource at a new address.\n" +
			"* To move a baseline, replace the resource block with a block at a new address in one apply. The apply removes the old label and freezes the draft under the new label. The old version stays frozen.\n" +
			"* Destroying the resource removes the label. The version stays frozen, with its conversations and evaluation runs. To keep the label, remove the resource with a `removed` block that has `lifecycle { destroy = false }`. If you apply the same configuration again, the resource freezes the current draft. It does not take back the old version.\n" +
			"* Labeling must happen after the configuration that it freezes. Add `depends_on` for the mesh's `partiteai_mesh_input` and `partiteai_mesh_link` resources, and for anything else that changes the draft.\n" +
			"* A frozen version keeps references to the agent versions it uses, so an agent cannot be deleted while a frozen version of its mesh exists. Add `depends_on` for the mesh's agents to the `partiteai_mesh` resource, so that a destroy deletes the mesh first.\n" +
			"* Do not change a label resource and the mesh configuration in the same apply. The label waits for the configuration, so it freezes the changed configuration.\n" +
			"* If one apply adds two label resources for the same mesh, add `depends_on` from one to the other. Without it, Terraform can create both at the same time, and one of them can fail with a conflict. Both freeze the same configuration.\n" +
			"* A webhook destination or a schedule that names a removed or renamed label fails on its next use.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the labeled mesh version.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the mesh. A change replaces the resource.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"label": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The label. A change renames the version.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"mesh_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the labeled mesh version. This is the same value as `id`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
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
	var data MeshVersionLabelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label := data.Label.ValueString()
	logResourceOperation(ctx, "creating", "mesh_version_label", label)

	labelResp, err := pgql.LabelMeshDraft(ctx, r.GetClient(), data.MeshId.ValueString(), label)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "label mesh draft")
		return
	}

	data.MapFromGraphQL(labelResp.LabelMeshDraft)
	logResourceOperation(ctx, "created", "mesh_version_label", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshVersionLabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeshVersionLabelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "mesh_version_label", data.Id.ValueString())

	versionResp, err := pgql.GetMeshVersion(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh_version_label", data.Id.ValueString())
		return
	}
	if versionResp.MeshVersion == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.MapFromGraphQL(*versionResp.MeshVersion)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshVersionLabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state MeshVersionLabelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "mesh_version_label", state.Id.ValueString())

	labelResp, err := pgql.LabelMeshVersion(ctx, r.GetClient(), state.Id.ValueString(), plan.Label.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "mesh_version_label", state.Id.ValueString())
		return
	}

	plan.MapFromGraphQL(labelResp.LabelMeshVersion)
	logResourceOperation(ctx, "updated", "mesh_version_label", plan.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MeshVersionLabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeshVersionLabelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "mesh_version_label", data.Id.ValueString())

	versionResp, err := pgql.GetMeshVersion(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh_version_label", data.Id.ValueString())
		return
	}
	if versionResp.MeshVersion == nil {
		return
	}

	if _, err := pgql.UnlabelMeshVersion(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "mesh_version_label", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "mesh_version_label", data.Id.ValueString())
}
