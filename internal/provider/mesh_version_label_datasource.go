package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ datasource.DataSource = &MeshVersionLabelDataSource{}

func NewMeshVersionLabelDataSource() datasource.DataSource {
	return &MeshVersionLabelDataSource{
		BaseDataSource: NewBaseDataSource("mesh_version_label"),
	}
}

type MeshVersionLabelDataSource struct {
	BaseDataSource
}

func (d *MeshVersionLabelDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds a mesh version by its mesh and its exact label.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the mesh version.",
			},
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the mesh.",
			},
			"label": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The label of the mesh version.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"mesh_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the mesh version. This is the same value as `id`.",
			},
			"ordinal": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The ordinal of the mesh version.",
			},
		},
	}
}

func (d *MeshVersionLabelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MeshVersionLabelModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	meshID := data.MeshId.ValueString()
	label := data.Label.ValueString()
	logDataSourceOperation(ctx, "reading", "mesh_version_label", label)

	lookup, err := pgql.GetMeshVersionByLabel(ctx, d.GetClient(), meshID, label)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "look up mesh version label")
		return
	}
	if lookup.Mesh == nil {
		resp.Diagnostics.AddError("Mesh not found", fmt.Sprintf("No mesh has the ID %s.", meshID))
		return
	}
	version := lookup.Mesh.VersionByLabel
	if version == nil {
		resp.Diagnostics.AddError(
			"Mesh version label not found",
			fmt.Sprintf("Mesh %s has no version with the label \"%s\".", meshID, label),
		)
		return
	}

	data.MapFromGraphQL(*version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
