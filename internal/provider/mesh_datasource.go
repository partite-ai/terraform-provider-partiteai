package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &MeshDataSource{}

func NewMeshDataSource() datasource.DataSource {
	return &MeshDataSource{
		BaseDataSource: NewBaseDataSource("mesh"),
	}
}

// MeshDataSource defines the data source implementation.
type MeshDataSource struct {
	BaseDataSource
}

// MeshDataSourceModel describes the data source data model.
type MeshDataSourceModel struct {
	MeshBaseModel
}

func (d *MeshDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Mesh data source",
		Attributes: standardDataSourceSchemaAttributes(map[string]schema.Attribute{
			"model_set_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the model set",
				Computed:            true,
			},
			"draft_version_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the draft version",
				Computed:            true,
			},
			"memory_store_id": schema.StringAttribute{
				MarkdownDescription: "ID of the memory store the mesh reads and writes, if it has one",
				Computed:            true,
			},
			"boundary_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of the boundaries every conversation on the mesh is checked against",
				ElementType:         types.StringType,
				Computed:            true,
			},
		}),
	}
}

func (d *MeshDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MeshDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "mesh", data.Id.ValueString())

	meshResp, err := pgql.GetMesh(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh", data.Id.ValueString())
		return
	}

	// Use the shared mapping from the base model
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *meshResp.Mesh)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
