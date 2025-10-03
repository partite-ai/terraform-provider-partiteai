package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &MeshLinkDataSource{}

func NewMeshLinkDataSource() datasource.DataSource {
	return &MeshLinkDataSource{
		BaseDataSource: NewBaseDataSource("mesh_link"),
	}
}

// MeshLinkDataSource defines the data source implementation.
type MeshLinkDataSource struct {
	BaseDataSource
}

// MeshLinkDataSourceModel describes the data source data model.
type MeshLinkDataSourceModel struct {
	MeshLinkResourceModel
}

func (d *MeshLinkDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Mesh Link data source",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"created_at":  commonDataSourceSchemas.createdAt,
			"updated_at":  commonDataSourceSchemas.updatedAt,
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Mesh ID",
			},
			"from_agent_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "From Agent ID",
			},
			"from_agent_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "From Agent Version ID",
			},
			"to_agent_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "To Agent ID",
			},
			"to_agent_version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "To Agent Version ID",
			},
			"transfers_control": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this link transfers control",
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

func (d *MeshLinkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MeshLinkDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "mesh_link", data.Id.ValueString())

	meshResp, err := pgql.GetMeshLinks(ctx, d.GetClient(), data.MeshId.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh_link", data.Id.ValueString())
		return
	}
	var link *pgql.LinkDetails
	for _, l := range meshResp.Mesh.DraftVersion.Links {
		if l.StableID == data.Id.ValueString() {
			link = &l
			break
		}
	}
	if link == nil {
		resp.Diagnostics.AddError("Not Found", "Mesh link not found")
		return
	}

	// Use a temporary resource to access its mapResponse method without storing the instance
	tempResource := MeshLinkResource{BaseResource: BaseResource{client: d.GetClient(), typeName: "mesh_link"}}
	resp.Diagnostics.Append(tempResource.mapResponse(ctx, &data.MeshLinkResourceModel, data.MeshId.ValueString(), *link)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
