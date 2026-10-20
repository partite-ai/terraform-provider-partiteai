package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &BoundaryDataSource{}

func NewBoundaryDataSource() datasource.DataSource {
	return &BoundaryDataSource{
		BaseDataSource: NewBaseDataSource("boundary"),
	}
}

// BoundaryDataSource defines the data source implementation.
type BoundaryDataSource struct {
	BaseDataSource
}

// BoundaryDataSourceModel describes the data source data model.
type BoundaryDataSourceModel struct {
	BoundaryBaseModel
}

func (d *BoundaryDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Boundary data source",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"report_only": schema.BoolAttribute{
				MarkdownDescription: "Every rule reports what it would refuse rather than refusing it, and no modification is made",
				Computed:            true,
			},
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "The rules, checked in order",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Name of the rule"},
						"action":      schema.StringAttribute{Computed: true, MarkdownDescription: "The action the rule checks"},
						"when":        schema.StringAttribute{Computed: true, MarkdownDescription: "CEL expression that decides whether the rule applies"},
						"effect":      schema.StringAttribute{Computed: true, MarkdownDescription: "`DENY` or `ASK`"},
						"message":     schema.StringAttribute{Computed: true, MarkdownDescription: "Told to whoever the rule refuses, or asks to approve"},
						"report_only": schema.BoolAttribute{Computed: true, MarkdownDescription: "The rule reports what it would refuse rather than refusing it"},
					},
				},
			},
			"modifications": schema.ListNestedAttribute{
				MarkdownDescription: "The modifications, made in order",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":          schema.StringAttribute{Computed: true, MarkdownDescription: "Name of the modification"},
						"action":        schema.StringAttribute{Computed: true, MarkdownDescription: "The action the modification sets things on"},
						"when":          schema.StringAttribute{Computed: true, MarkdownDescription: "CEL expression that decides whether the modification applies"},
						"set_tags":      schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Tags to set, each name with a CEL expression that gives its values"},
						"set_tracing":   schema.StringAttribute{Computed: true, MarkdownDescription: "A CEL expression that gives whether the conversation is traced"},
						"set_arguments": schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Tool call arguments to set, each name with a CEL expression that gives its value"},
					},
				},
			},
			"warnings": schema.ListAttribute{
				MarkdownDescription: "What is likely wrong with the boundary",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *BoundaryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BoundaryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "boundary", data.Id.ValueString())

	getResp, err := pgql.GetBoundary(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "boundary", data.Id.ValueString())
		return
	}
	if getResp.Boundary == nil {
		resp.Diagnostics.AddError("Not found", "No boundary has ID "+data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.Boundary)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
