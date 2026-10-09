package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MeshResource{}
var _ resource.ResourceWithImportState = &MeshResource{}

func NewMeshResource() resource.Resource {
	return &MeshResource{
		BaseResource: NewBaseResource("mesh"),
	}
}

// MeshResource defines the resource implementation.
type MeshResource struct {
	BaseResource
}

// MeshResourceModel describes the resource data model.
type MeshResourceModel struct {
	MeshBaseModel
}

func (r *MeshResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mesh"
}

func (r *MeshResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Mesh resource",
		Attributes: standardSchemaAttributes(
			map[string]schema.Attribute{
				"model_set_id": schema.StringAttribute{
					MarkdownDescription: "Model set ID associated with the mesh",
					Required:            true,
				},
				"draft_version_id": schema.StringAttribute{
					MarkdownDescription: "Draft version ID of the mesh",
					Computed:            true,
				},
				"memory_store_id": schema.StringAttribute{
					MarkdownDescription: "ID of the memory store the mesh reads and writes. Every version of the mesh uses the same store. Removing it detaches the store and keeps its records.",
					Optional:            true,
				},
			},
		),
	}
}

func (r *MeshResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MeshResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "mesh", "")

	var tags []*pgql.Tag
	if diag := data.Tags.ElementsAs(ctx, &tags, false); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	createResp, err := pgql.CreateMesh(ctx, r.GetClient(), &pgql.NewMesh{
		Name:          data.Name.ValueString(),
		Description:   pgql.OptPtr(data.Description.ValueStringPointer()),
		Tags:          tags,
		ModelSetID:    data.ModelSetId.ValueString(),
		MemoryStoreID: pgql.OptPtr(data.MemoryStoreId.ValueStringPointer()),
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create mesh")
		return
	}

	resp.Diagnostics.Append(r.mapFields(ctx, &data, createResp.CreateMesh.Mesh)...)

	logResourceOperation(ctx, "created", "mesh", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeshResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "mesh", data.Id.ValueString())

	meshResp, err := pgql.GetMesh(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapFields(ctx, &data, *meshResp.Mesh)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MeshResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "mesh", planData.Id.ValueString())

	var update pgql.UpdateMesh
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}
	if !planData.ModelSetId.Equal(stateData.ModelSetId) {
		update.ModelSetID = pgql.Opt(planData.ModelSetId.ValueString())
	}
	if !planData.MemoryStoreId.Equal(stateData.MemoryStoreId) {
		update.MemoryStoreID = pgql.OptNil(planData.MemoryStoreId.ValueStringPointer())
	}
	if !planData.Tags.Equal(stateData.Tags) {
		var tags []*pgql.Tag
		if diag := planData.Tags.ElementsAs(ctx, &tags, false); diag.HasError() {
			resp.Diagnostics.Append(diag...)
			return
		}
		update.Tags = pgql.Opt(tags)
	}

	updateResp, err := pgql.ModifyMesh(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "mesh", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapFields(ctx, &planData, updateResp.UpdateMesh.Mesh)...)

	logResourceOperation(ctx, "updated", "mesh", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *MeshResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeshResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "mesh", data.Id.ValueString())

	_, err := pgql.DeleteMesh(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "mesh", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "mesh", data.Id.ValueString())
}

func (r *MeshResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *MeshResource) mapFields(ctx context.Context, data *MeshResourceModel, meshDetails pgql.MeshDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, meshDetails)
}
