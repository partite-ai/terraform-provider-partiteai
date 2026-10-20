package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AccessPointDataSource{}

func NewAccessPointDataSource() datasource.DataSource {
	return &AccessPointDataSource{
		BaseDataSource: NewBaseDataSource("access_point"),
	}
}

// AccessPointDataSource defines the data source implementation.
type AccessPointDataSource struct {
	BaseDataSource
}

// AccessPointDataSourceModel describes the data source data model.
type AccessPointDataSourceModel struct {
	AccessPointBaseModel
}

func (d *AccessPointDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Access point data source",
		Attributes: standardDataSourceSchemaAttributes(map[string]schema.Attribute{
			"url": schema.StringAttribute{
				MarkdownDescription: "Where callers reach the access point",
				Computed:            true,
			},
			"authorization_server_url": schema.StringAttribute{
				MarkdownDescription: "Advertised to MCP clients as where to log in",
				Computed:            true,
			},
			"client_token_lifetime_seconds": schema.Int64Attribute{
				MarkdownDescription: "How long a token issued to a client lasts, in seconds",
				Computed:            true,
			},
			"issuers": schema.ListNestedAttribute{
				MarkdownDescription: "The issuers whose tokens the access point accepts",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"issuer":         schema.StringAttribute{Computed: true, MarkdownDescription: "The `iss` a token from this issuer carries"},
						"audience":       schema.StringAttribute{Computed: true, MarkdownDescription: "Must be in a token's `aud`"},
						"check_audience": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a token's `aud` is checked"},
						"jwks_url":       schema.StringAttribute{Computed: true, MarkdownDescription: "Where the issuer publishes its key set"},
						"jwks":           schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "A key set uploaded for an issuer that publishes none"},
					},
				},
			},
			"boundary_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of the boundaries conversations created through the access point are checked against",
				ElementType:         types.StringType,
				Computed:            true,
			},
		}),
	}
}

func (d *AccessPointDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AccessPointDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "access_point", data.Id.ValueString())

	getResp, err := pgql.GetAccessPoint(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "access_point", data.Id.ValueString())
		return
	}
	if getResp.AccessPoint == nil {
		resp.Diagnostics.AddError("Not found", "No access point has ID "+data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.AccessPoint)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
