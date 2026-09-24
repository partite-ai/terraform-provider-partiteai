package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ datasource.DataSource = &MemoryStoreDataSource{}

func NewMemoryStoreDataSource() datasource.DataSource {
	return &MemoryStoreDataSource{
		BaseDataSource: NewBaseDataSource("memory_store"),
	}
}

type MemoryStoreDataSource struct {
	BaseDataSource
}

type MemoryStoreDataSourceModel struct {
	MemoryStoreBaseModel
}

func (d *MemoryStoreDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A memory store: what a mesh remembers, and the scopes that divide it.",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"created_at":  commonDataSourceSchemas.createdAt,
			"updated_at":  commonDataSourceSchemas.updatedAt,
			"ingest_conversations": schema.BoolAttribute{
				MarkdownDescription: "Whether the turns of a conversation are written into this store as they are said",
				Computed:            true,
			},
			"index_conversation_messages": schema.BoolAttribute{
				MarkdownDescription: "Whether those turns are indexed and embedded as well as kept",
				Computed:            true,
			},
			"embedding_profile_id": schema.StringAttribute{
				MarkdownDescription: "ID of the embedding profile this store writes its vectors with",
				Computed:            true,
			},
			"user_name_tag_key": schema.StringAttribute{
				MarkdownDescription: "The tag key whose value names whoever is not the agent",
				Computed:            true,
			},
			"extract_interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "How often the turns of this store are planned into extraction work",
				Computed:            true,
			},
			"consolidate_interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "How often the facts of this store are queued into consolidation passes",
				Computed:            true,
			},
			"scopes": schema.ListNestedAttribute{
				MarkdownDescription: "The scopes of this store",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Scope identifier",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "What this scope is for",
							Computed:            true,
						},
						"kind": schema.StringAttribute{
							MarkdownDescription: "SLICED keeps a memory of each value of the tag it matches apart. UNSLICED makes one memory of everything it matches.",
							Computed:            true,
						},
						"key": schema.StringAttribute{
							MarkdownDescription: "The tag key that every record of this scope carries",
							Computed:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "What an UNSLICED scope writes under that key",
							Computed:            true,
						},
						"match": schema.StringAttribute{
							MarkdownDescription: "The tag key that selects the documents this scope reads",
							Computed:            true,
						},
						"extraction_enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether facts are taken out of the documents this scope reads",
							Computed:            true,
						},
						"extraction_prompt": schema.StringAttribute{
							MarkdownDescription: "Extra instructions for the extraction call",
							Computed:            true,
						},
						"extraction_model_profile_id": schema.StringAttribute{
							MarkdownDescription: "ID of the model profile extraction runs on",
							Computed:            true,
						},
						"consolidation_enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether the facts of this scope are folded into observations",
							Computed:            true,
						},
						"consolidation_prompt": schema.StringAttribute{
							MarkdownDescription: "Extra instructions for the consolidation call",
							Computed:            true,
						},
						"consolidation_model_profile_id": schema.StringAttribute{
							MarkdownDescription: "ID of the model profile consolidation runs on",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *MemoryStoreDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MemoryStoreDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "memory_store", data.Id.ValueString())

	storeResp, err := pgql.GetMemoryStore(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "memory_store", data.Id.ValueString())
		return
	}
	if storeResp.MemoryStore == nil {
		resp.Diagnostics.AddError("Not found", "memory_store "+data.Id.ValueString()+" not found")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *storeResp.MemoryStore)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
