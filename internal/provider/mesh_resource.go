package provider

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MeshResource{}
var _ resource.ResourceWithImportState = &MeshResource{}

func NewMeshResource() resource.Resource {
	return &MeshResource{}
}

// MeshResource defines the resource implementation.
type MeshResource struct {
	client graphql.Client
}

// MeshResourceModel describes the resource data model.
type MeshResourceModel struct {
	Id             types.String      `tfsdk:"id"`
	Name           types.String      `tfsdk:"name"`
	Description    types.String      `tfsdk:"description"`
	Tags           types.List        `tfsdk:"tags"`
	CreatedAt      timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt      timetypes.RFC3339 `tfsdk:"updated_at"`
	ModelSetId     types.String      `tfsdk:"model_set_id"`
	DraftVersionId types.String      `tfsdk:"draft_version_id"`
}

func (r *MeshResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mesh"
}

func (r *MeshResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	tagType := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Tag name",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "Tag value",
				Required:            true,
			},
		},
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Mesh resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mesh identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Mesh name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Mesh description",
				Optional:            true,
			},
			"tags": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(tagType.Type(), nil)),
				NestedObject:        tagType,
				MarkdownDescription: "Tags for the Mesh",
			},
			"created_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the mesh was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the mesh was last updated",
			},
			"model_set_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Model set ID associated with the mesh",
			},
			"draft_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Draft version ID of the mesh",
			},
		},
	}
}

func (r *MeshResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(graphql.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected graphql.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *MeshResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MeshResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var tags []*pgql.Tag
	if diag := data.Tags.ElementsAs(ctx, &tags, false); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	createResp, err := pgql.CreateMesh(ctx, r.client, &pgql.NewMesh{
		Name:        data.Name.ValueString(),
		Description: pgql.OptPtr(data.Description.ValueStringPointer()),
		Tags:        tags,
		ModelSetID:  data.ModelSetId.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create mesh, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &data, createResp.CreateMesh.Mesh); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	tflog.Trace(ctx, "created a mesh resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeshResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	meshResp, err := pgql.GetMesh(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read mesh, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &data, *meshResp.Mesh); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MeshResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}
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
	if !planData.Tags.Equal(stateData.Tags) {
		var tags []*pgql.Tag
		if diag := planData.Tags.ElementsAs(ctx, &tags, false); diag.HasError() {
			resp.Diagnostics.Append(diag...)
			return
		}
		update.Tags = pgql.Opt(tags)
	}
	updateResp, err := pgql.ModifyMesh(ctx, r.client, planData.Id.ValueString(), &update)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update mesh, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &planData, updateResp.UpdateMesh.Mesh); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *MeshResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeshResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := pgql.DeleteMesh(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete mesh, got error: %s", err))
		return
	}
}

func (r *MeshResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *MeshResource) mapFields(ctx context.Context, data *MeshResourceModel, meshDetails pgql.MeshDetails) diag.Diagnostics {
	var diag diag.Diagnostics
	data.Id = types.StringValue(meshDetails.Id)
	data.Name = types.StringValue(meshDetails.Name)
	data.Description = types.StringPointerValue(meshDetails.Description)
	data.Tags, diag = types.ListValueFrom(ctx, data.Tags.ElementType(ctx), meshDetails.Tags)
	if diag.HasError() {
		return diag
	}
	data.CreatedAt, diag = timetypes.NewRFC3339Value(meshDetails.CreatedAt)
	if diag.HasError() {
		return diag
	}
	data.UpdatedAt, diag = timetypes.NewRFC3339Value(meshDetails.UpdatedAt)
	if diag.HasError() {
		return diag
	}
	data.DraftVersionId = types.StringValue(meshDetails.DraftVersion.Id)
	data.ModelSetId = types.StringValue(meshDetails.DraftVersion.ModelSet.Id)
	return diag
}
