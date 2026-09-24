package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ datasource.DataSource = &MemoryInteractionProfileDataSource{}

func NewMemoryInteractionProfileDataSource() datasource.DataSource {
	return &MemoryInteractionProfileDataSource{
		BaseDataSource: NewBaseDataSource("memory_interaction_profile"),
	}
}

type MemoryInteractionProfileDataSource struct {
	BaseDataSource
}

type MemoryInteractionProfileDataSourceModel struct {
	MemoryInteractionProfileBaseModel
}

func (d *MemoryInteractionProfileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A memory interaction profile: what an agent is given from memory, and what it can call to read or write more.",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"created_at":  commonDataSourceSchemas.createdAt,
			"updated_at":  commonDataSourceSchemas.updatedAt,
			"allow_expansion": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent can read around a record it was given",
				Computed:            true,
			},
			"conversation_history": schema.SingleNestedAttribute{
				MarkdownDescription: "What of this conversation goes in the agent's prompt",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"token_budget": schema.Int64Attribute{
						MarkdownDescription: "How many tokens of history the prompt can hold",
						Computed:            true,
					},
					"selection": schema.StringAttribute{
						MarkdownDescription: "RECENT or RELEVANT",
						Computed:            true,
					},
					"query": memoryQuerySourceDataSourceSchema("What a RELEVANT selection searches for"),
				},
			},
			"context_injections": schema.ListNestedAttribute{
				MarkdownDescription: "What goes in the agent's prompt from memory, without the agent asking for it",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Identifies this injection to a person",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "What these memories are, written at the head of the section that holds them",
							Computed:            true,
						},
						"token_budget": schema.Int64Attribute{
							MarkdownDescription: "How many tokens of memory this injection can hold",
							Computed:            true,
						},
						"record_types": memoryRecordTypesDataSourceSchema(),
						"selection":    memorySelectionDataSourceSchema(),
						"query":        memoryQuerySourceDataSourceSchema("What this injection searches memory for"),
					},
				},
			},
			"query_tools": schema.ListNestedAttribute{
				MarkdownDescription: "The tools this agent can call to search memory",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tool_name": schema.StringAttribute{
							MarkdownDescription: "What the agent calls",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "What the tool is for, and when to call it",
							Computed:            true,
						},
						"record_types": memoryRecordTypesDataSourceSchema(),
						"selection":    memorySelectionDataSourceSchema(),
						"token_budget": schema.Int64Attribute{
							MarkdownDescription: "How many tokens one call returns",
							Computed:            true,
						},
						"effort_levels": schema.ListNestedAttribute{
							MarkdownDescription: "The amounts the agent can ask for by name",
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"level": schema.StringAttribute{
										MarkdownDescription: "What the model names this amount",
										Computed:            true,
									},
									"token_budget": schema.Int64Attribute{
										MarkdownDescription: "How many tokens a call at this level returns",
										Computed:            true,
									},
								},
							},
						},
					},
				},
			},
			"write_tools": schema.ListNestedAttribute{
				MarkdownDescription: "The tools this agent can call to write a memory",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tool_name": schema.StringAttribute{
							MarkdownDescription: "What the agent calls",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "What the tool is for, and when to call it",
							Computed:            true,
						},
						"tag_mappings": schema.ListNestedAttribute{
							MarkdownDescription: "The tags put on what this tool writes",
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"kind": schema.StringAttribute{
										MarkdownDescription: "FIXED, ALL_FROM_REQUEST or KEYS_FROM_REQUEST",
										Computed:            true,
									},
									"keys": schema.ListAttribute{
										MarkdownDescription: "The request tag keys copied by KEYS_FROM_REQUEST",
										ElementType:         types.StringType,
										Computed:            true,
									},
									"tag_key": schema.StringAttribute{
										MarkdownDescription: "The key FIXED writes",
										Computed:            true,
									},
									"tag_value": schema.StringAttribute{
										MarkdownDescription: "The value FIXED writes",
										Computed:            true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// memoryRecordTypesDataSourceSchema is the kinds of record an injection or a
// query tool reads.
func memoryRecordTypesDataSourceSchema() schema.Attribute {
	return schema.ListAttribute{
		MarkdownDescription: "The kinds of record read: CHUNK, FACT or OBSERVATION",
		ElementType:         types.StringType,
		Computed:            true,
	}
}

// memorySelectionDataSourceSchema is what narrows the memory one injection or
// tool reads.
func memorySelectionDataSourceSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "Which memories this reads",
		Computed:            true,
		Attributes: map[string]schema.Attribute{
			"kind": schema.StringAttribute{
				MarkdownDescription: "ALL or TAGS",
				Computed:            true,
			},
			"tags": schema.ListNestedAttribute{
				MarkdownDescription: "The tags this reads by",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"source": schema.StringAttribute{
							MarkdownDescription: "FIXED or REQUEST_TAG",
							Computed:            true,
						},
						"tag_key": schema.StringAttribute{
							MarkdownDescription: "The key this reads the store by",
							Computed:            true,
						},
						"tag_value": schema.StringAttribute{
							MarkdownDescription: "The value FIXED reads by",
							Computed:            true,
						},
						"request_tag_key": schema.StringAttribute{
							MarkdownDescription: "The key REQUEST_TAG reads off the request",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

// memoryQuerySourceDataSourceSchema is the text an injection or a history
// searches for.
func memoryQuerySourceDataSourceSchema(description string) schema.Attribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: description,
		Computed:            true,
		Attributes: map[string]schema.Attribute{
			"kind": schema.StringAttribute{
				MarkdownDescription: "FIXED or DERIVED",
				Computed:            true,
			},
			"text": schema.StringAttribute{
				MarkdownDescription: "What FIXED searches for",
				Computed:            true,
			},
		},
	}
}

func (d *MemoryInteractionProfileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MemoryInteractionProfileDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "memory_interaction_profile", data.Id.ValueString())

	profileResp, err := pgql.GetMemoryInteractionProfile(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "memory_interaction_profile", data.Id.ValueString())
		return
	}
	if profileResp.MemoryInteractionProfile == nil {
		resp.Diagnostics.AddError("Not found", "memory_interaction_profile "+data.Id.ValueString()+" not found")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *profileResp.MemoryInteractionProfile)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
