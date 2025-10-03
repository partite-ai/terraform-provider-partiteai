package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &MeshInputDataSource{}

func NewMeshInputDataSource() datasource.DataSource {
	return &MeshInputDataSource{
		BaseDataSource: NewBaseDataSource("mesh_input"),
	}
}

// MeshInputDataSource defines the data source implementation.
type MeshInputDataSource struct {
	BaseDataSource
}

// MeshInputDataSourceModel describes the data source data model.
type MeshInputDataSourceModel struct {
	MeshInputResourceModel
}

func (d *MeshInputDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Mesh Input data source",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Resource name",
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Mesh ID",
			},
			"agent_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Agent ID",
			},
			"agent_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Agent Version ID",
			},
			"responses": schema.SetNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Response name",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Response description",
						},
						"message_type": schema.StringAttribute{
							CustomType:          jsontypes.NormalizedType{},
							Computed:            true,
							MarkdownDescription: "Response message type in JSON Schema format",
						},
					},
				},
			},
		},
	}
}

func (d *MeshInputDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MeshInputDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	meshInputId := data.MeshId.ValueString() + "/" + data.Name.ValueString()
	logDataSourceOperation(ctx, "reading", "mesh_input", meshInputId)

	meshResp, err := pgql.GetMeshInputs(ctx, d.GetClient(), data.MeshId.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh_input", meshInputId)
		return
	}

	var input *pgql.InputDetails
	for _, i := range meshResp.Mesh.DraftVersion.Inputs {
		if i.Name == data.Name.ValueString() {
			input = &i
			break
		}
	}
	if input == nil {
		resp.Diagnostics.AddError("Not Found", "Mesh input not found")
		return
	}

	// Use a temporary resource to access its mapResponse method without storing the instance
	tempResource := MeshInputResource{BaseResource: BaseResource{client: d.GetClient(), typeName: "mesh_input"}}
	resp.Diagnostics.Append(tempResource.mapResponse(ctx, &data.MeshInputResourceModel, data.MeshId.ValueString(), *input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
