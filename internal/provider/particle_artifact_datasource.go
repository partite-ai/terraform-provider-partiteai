package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ datasource.DataSource = &ParticleArtifactDataSource{}

func NewParticleArtifactDataSource() datasource.DataSource {
	return &ParticleArtifactDataSource{
		BaseDataSource: NewBaseDataSource("particle_artifact"),
	}
}

type ParticleArtifactDataSource struct {
	BaseDataSource
}

type ParticleArtifactDataSourceModel struct {
	ParticleArtifactBaseModel
}

func (d *ParticleArtifactDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A particle artifact uploaded to the Partite AI mesh.",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"source_path": schema.StringAttribute{Computed: true},
			"source_hash": schema.StringAttribute{Computed: true},
			"name":        schema.StringAttribute{Computed: true},
			"version":     schema.StringAttribute{Computed: true},
			"description": schema.StringAttribute{Computed: true},
			"created_at":  commonDataSourceSchemas.createdAt,
			"capabilities": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"http": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"allowed_hosts": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
							},
						},
					},
				},
			},
			"credentials": schema.ListNestedAttribute{
				Computed:     true,
				NestedObject: particleArtifactCredentialDataSourceSchema(),
			},
			"tools": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
						"input_schema": schema.StringAttribute{
							CustomType: jsontypes.NormalizedType{},
							Computed:   true,
						},
					},
				},
			},
		},
	}
}

func particleArtifactCredentialDataSourceSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Computed: true},
			"hosts":    schema.ListAttribute{ElementType: types.StringType, Computed: true},
			"required": schema.BoolAttribute{Computed: true},
			"methods": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":        schema.StringAttribute{Computed: true},
						"type":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
						"apikey_location": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"kind":   schema.StringAttribute{Computed: true},
								"name":   schema.StringAttribute{Computed: true},
								"scheme": schema.StringAttribute{Computed: true},
							},
						},
						"oauth2": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"authorization_url": schema.StringAttribute{Computed: true},
								"token_url":         schema.StringAttribute{Computed: true},
								"device_auth_url":   schema.StringAttribute{Computed: true},
								"scopes":            schema.ListAttribute{ElementType: types.StringType, Computed: true},
								"flows":             schema.ListAttribute{ElementType: types.StringType, Computed: true},
							},
						},
						"signing_key": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"algorithm": schema.StringAttribute{Computed: true},
							},
						},
					},
				},
			},
		},
	}
}

func (d *ParticleArtifactDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ParticleArtifactDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "particle_artifact", data.Id.ValueString())

	getResp, err := pgql.GetParticleArtifact(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "particle_artifact", data.Id.ValueString())
		return
	}
	if getResp.ParticleArtifact == nil {
		resp.Diagnostics.AddError("Not found", "particle_artifact "+data.Id.ValueString()+" not found")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.ParticleArtifact)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
