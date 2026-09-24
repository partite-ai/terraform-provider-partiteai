package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ datasource.DataSource = &InstalledParticleDataSource{}

func NewInstalledParticleDataSource() datasource.DataSource {
	return &InstalledParticleDataSource{
		BaseDataSource: NewBaseDataSource("installed_particle"),
	}
}

type InstalledParticleDataSource struct {
	BaseDataSource
}

// InstalledParticleDataSourceModel is a narrower model than the base: the
// data source does not expose write-only credentials.
type InstalledParticleDataSourceModel struct {
	Id             types.String      `tfsdk:"id"`
	Name           types.String      `tfsdk:"name"`
	ArtifactId     types.String      `tfsdk:"artifact_id"`
	KvWorkspaceId  types.String      `tfsdk:"kv_workspace_id"`
	Mounts         types.Set         `tfsdk:"mounts"`
	ActiveArtifact types.Object      `tfsdk:"active_artifact"`
	CreatedAt      timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt      timetypes.RFC3339 `tfsdk:"updated_at"`
}

func (m *InstalledParticleDataSourceModel) mapFromGraphQL(ctx context.Context, src pgql.InstalledParticleDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, src.Id,
		&m.Name, src.Name,
		&m.ArtifactId, src.ActiveArtifact.Id,
		&m.CreatedAt, src.CreatedAt,
		&m.UpdatedAt, src.UpdatedAt,
	)
	if diags.HasError() {
		return diags
	}
	activeObj, objDiags := types.ObjectValueFrom(ctx, activeArtifactAttrTypes(), activeArtifactStruct{
		Id:      src.ActiveArtifact.Id,
		Name:    src.ActiveArtifact.Name,
		Version: src.ActiveArtifact.Version,
	})
	diags.Append(objDiags...)
	m.ActiveArtifact = activeObj

	m.KvWorkspaceId = types.StringNull()
	if src.KvWorkspace != nil {
		m.KvWorkspaceId = types.StringValue(src.KvWorkspace.Id)
	}

	bindings := make([]installedParticleMountInfo, 0, len(src.Mounts))
	for _, mount := range src.Mounts {
		bindings = append(bindings, installedParticleMountInfo{
			MountName:   types.StringValue(mount.MountName),
			WorkspaceId: types.StringValue(mount.Workspace.Id),
		})
	}
	mountSet, setDiags := types.SetValueFrom(ctx, mountAttrType(), bindings)
	diags.Append(setDiags...)
	m.Mounts = mountSet

	return diags
}

func (d *InstalledParticleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An installed particle.",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"created_at":  commonDataSourceSchemas.createdAt,
			"updated_at":  commonDataSourceSchemas.updatedAt,
			"artifact_id": schema.StringAttribute{Computed: true},
			"kv_workspace_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workspace backing this install's key-value state, if it has one.",
			},
			"mounts": schema.SetNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Mount bindings, each mapping a manifest mount name to a workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"mount_name":   schema.StringAttribute{Computed: true},
						"workspace_id": schema.StringAttribute{Computed: true},
					},
				},
			},
			"active_artifact": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"id":      schema.StringAttribute{Computed: true},
					"name":    schema.StringAttribute{Computed: true},
					"version": schema.StringAttribute{Computed: true},
				},
			},
		},
	}
}

func (d *InstalledParticleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data InstalledParticleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	logDataSourceOperation(ctx, "reading", "installed_particle", data.Id.ValueString())

	getResp, err := pgql.GetInstalledParticle(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "installed_particle", data.Id.ValueString())
		return
	}
	if getResp.InstalledParticle == nil {
		resp.Diagnostics.AddError("Not found", "installed_particle "+data.Id.ValueString()+" not found")
		return
	}
	resp.Diagnostics.Append(data.mapFromGraphQL(ctx, *getResp.InstalledParticle)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
