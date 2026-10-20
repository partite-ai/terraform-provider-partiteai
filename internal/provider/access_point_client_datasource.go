package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AccessPointClientDataSource{}

func NewAccessPointClientDataSource() datasource.DataSource {
	return &AccessPointClientDataSource{
		BaseDataSource: NewBaseDataSource("access_point_client"),
	}
}

// AccessPointClientDataSource defines the data source implementation.
type AccessPointClientDataSource struct {
	BaseDataSource
}

// AccessPointClientDataSourceModel describes the data source data model.
type AccessPointClientDataSourceModel struct {
	AccessPointClientBaseModel
}

func (d *AccessPointClientDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Access point client data source",
		Attributes: map[string]schema.Attribute{
			"id":   commonDataSourceSchemas.id,
			"name": commonDataSourceSchemas.name,
			"access_point_id": schema.StringAttribute{
				MarkdownDescription: "ID of the access point the client signs in to",
				Computed:            true,
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "What the client signs in as",
				Computed:            true,
			},
			"claims": schema.StringAttribute{
				MarkdownDescription: "A JSON object copied into every token the client gets",
				CustomType:          jsontypes.NormalizedType{},
				Computed:            true,
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *AccessPointClientDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AccessPointClientDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "access_point_client", data.Id.ValueString())

	getResp, err := pgql.GetAccessPointClient(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "access_point_client", data.Id.ValueString())
		return
	}
	if getResp.AccessPointClient == nil {
		resp.Diagnostics.AddError("Not found", "No access point client has ID "+data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.AccessPointClient)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
