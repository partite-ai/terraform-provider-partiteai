package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &MemoryStoreResource{}
var _ resource.ResourceWithImportState = &MemoryStoreResource{}

func NewMemoryStoreResource() resource.Resource {
	return &MemoryStoreResource{
		BaseResource: NewBaseResource("memory_store"),
	}
}

type MemoryStoreResource struct {
	BaseResource
}

type MemoryStoreResourceModel struct {
	MemoryStoreBaseModel
}

func (r *MemoryStoreResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A memory store: what a mesh remembers, and the scopes that divide it.",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"created_at":  commonSchemas.createdAt,
			"updated_at":  commonSchemas.updatedAt,
			"ingest_conversations": schema.BoolAttribute{
				MarkdownDescription: "Write the turns of a conversation into this store as they are said",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"index_conversation_messages": schema.BoolAttribute{
				MarkdownDescription: "Index and embed those turns as well as keeping them, so a recall can find one by its words or its meaning",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"embedding_profile_id": schema.StringAttribute{
				MarkdownDescription: "ID of the embedding profile this store writes its vectors with. A store without one is not embedded, and a recall over it searches words only. A store with any scope that extracts or consolidates needs one.",
				Optional:            true,
			},
			"user_name_tag_key": schema.StringAttribute{
				MarkdownDescription: "The tag key whose value names whoever is not the agent. A store without one leaves those speakers unnamed.",
				Optional:            true,
			},
			"extract_interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "How often the turns of this store are planned into extraction work",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(300),
			},
			"consolidate_interval_seconds": schema.Int64Attribute{
				MarkdownDescription: "How often the facts of this store are queued into consolidation passes",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(28800),
			},
			"scopes": schema.ListNestedAttribute{
				MarkdownDescription: "The scopes of this store. A scope decides which documents are read and what tag the records it produces carry.",
				Optional:            true,
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Scope identifier",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "What this scope is for",
							Optional:            true,
						},
						"kind": schema.StringAttribute{
							MarkdownDescription: "SLICED keeps a memory of each value of the tag it matches apart. UNSLICED makes one memory of everything it matches.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("SLICED", "UNSLICED"),
							},
						},
						"key": schema.StringAttribute{
							MarkdownDescription: "The tag key that every record of this scope carries. It names the scope, and it never changes.",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "What an UNSLICED scope writes under that key. A SLICED scope takes each value from the document instead.",
							Optional:            true,
						},
						"match": schema.StringAttribute{
							MarkdownDescription: "The tag key that selects the documents this scope reads. A SLICED scope needs one. An UNSLICED scope without one reads every document.",
							Optional:            true,
						},
						"extraction_enabled": schema.BoolAttribute{
							MarkdownDescription: "Take facts out of the documents this scope reads",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
						"extraction_prompt": schema.StringAttribute{
							MarkdownDescription: "Extra instructions for the extraction call",
							Optional:            true,
						},
						"extraction_model_profile_id": schema.StringAttribute{
							MarkdownDescription: "ID of the model profile extraction runs on. Required where extraction_enabled is set.",
							Optional:            true,
						},
						"consolidation_enabled": schema.BoolAttribute{
							MarkdownDescription: "Fold the facts of this scope into observations",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
						"consolidation_prompt": schema.StringAttribute{
							MarkdownDescription: "Extra instructions for the consolidation call",
							Optional:            true,
						},
						"consolidation_model_profile_id": schema.StringAttribute{
							MarkdownDescription: "ID of the model profile consolidation runs on. Required where consolidation_enabled is set.",
							Optional:            true,
						},
					},
				},
			},
		},
	}
}

func (r *MemoryStoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MemoryStoreResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "memory_store", "")

	scopes, diags := planScopes(ctx, data.Scopes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	create := pgql.NewMemoryStore{
		Name:                       data.Name.ValueString(),
		Description:                pgql.OptPtr(data.Description.ValueStringPointer()),
		IngestConversations:        data.IngestConversations.ValueBool(),
		IndexConversationMessages:  data.IndexConversationMessages.ValueBool(),
		EmbeddingProfileID:         pgql.OptPtr(data.EmbeddingProfileId.ValueStringPointer()),
		UserNameTagKey:             pgql.OptPtr(data.UserNameTagKey.ValueStringPointer()),
		ExtractIntervalSeconds:     int(data.ExtractIntervalSeconds.ValueInt64()),
		ConsolidateIntervalSeconds: int(data.ConsolidateIntervalSeconds.ValueInt64()),
	}
	for _, scope := range scopes {
		create.Scopes = append(create.Scopes, newScope(scope))
	}

	createResp, err := pgql.CreateMemoryStore(ctx, r.GetClient(), &create)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create memory store")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, createResp.CreateMemoryStore)...)

	logResourceOperation(ctx, "created", "memory_store", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MemoryStoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MemoryStoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "memory_store", data.Id.ValueString())

	storeResp, err := pgql.GetMemoryStore(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "memory_store", data.Id.ValueString())
		return
	}
	if storeResp.MemoryStore == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *storeResp.MemoryStore)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update writes the store's own settings, and the scopes as three lists: the
// ones to remove, the ones to change and the ones to add. A scope is matched by
// its key, which never changes, so a scope that keeps its key keeps the records
// it already produced.
func (r *MemoryStoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MemoryStoreResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "memory_store", planData.Id.ValueString())

	var update pgql.UpdateMemoryStore
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}
	if !planData.IngestConversations.Equal(stateData.IngestConversations) {
		update.IngestConversations = pgql.Opt(planData.IngestConversations.ValueBool())
	}
	if !planData.IndexConversationMessages.Equal(stateData.IndexConversationMessages) {
		update.IndexConversationMessages = pgql.Opt(planData.IndexConversationMessages.ValueBool())
	}
	if !planData.EmbeddingProfileId.Equal(stateData.EmbeddingProfileId) {
		update.EmbeddingProfileID = pgql.OptNil(planData.EmbeddingProfileId.ValueStringPointer())
	}
	if !planData.UserNameTagKey.Equal(stateData.UserNameTagKey) {
		update.UserNameTagKey = pgql.OptNil(planData.UserNameTagKey.ValueStringPointer())
	}
	if !planData.ExtractIntervalSeconds.Equal(stateData.ExtractIntervalSeconds) {
		update.ExtractIntervalSeconds = pgql.Opt(int(planData.ExtractIntervalSeconds.ValueInt64()))
	}
	if !planData.ConsolidateIntervalSeconds.Equal(stateData.ConsolidateIntervalSeconds) {
		update.ConsolidateIntervalSeconds = pgql.Opt(int(planData.ConsolidateIntervalSeconds.ValueInt64()))
	}

	if !planData.Scopes.Equal(stateData.Scopes) {
		wanted, diags := planScopes(ctx, planData.Scopes)
		resp.Diagnostics.Append(diags...)
		current, diags := planScopes(ctx, stateData.Scopes)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		byKey := make(map[string]memoryStoreScopeModel, len(current))
		for _, scope := range current {
			byKey[scope.Key.ValueString()] = scope
		}

		for _, scope := range wanted {
			existing, known := byKey[scope.Key.ValueString()]
			if !known {
				update.CreateScopes = append(update.CreateScopes, newScope(scope))
				continue
			}
			delete(byKey, scope.Key.ValueString())
			update.UpdateScopes = append(update.UpdateScopes, &pgql.MemoryStoreScopeEdit{
				ID: existing.Id.ValueString(),
				Changes: &pgql.UpdateMemoryStoreScope{
					Description:                 pgql.OptNil(scope.Description.ValueStringPointer()),
					Match:                       pgql.OptNil(scope.Match.ValueStringPointer()),
					ExtractionEnabled:           pgql.Opt(scope.ExtractionEnabled.ValueBool()),
					ExtractionPrompt:            pgql.OptNil(scope.ExtractionPrompt.ValueStringPointer()),
					ExtractionModelProfileID:    pgql.OptNil(scope.ExtractionModelProfileId.ValueStringPointer()),
					ConsolidationEnabled:        pgql.Opt(scope.ConsolidationEnabled.ValueBool()),
					ConsolidationPrompt:         pgql.OptNil(scope.ConsolidationPrompt.ValueStringPointer()),
					ConsolidationModelProfileID: pgql.OptNil(scope.ConsolidationModelProfileId.ValueStringPointer()),
				},
			})
		}
		for _, scope := range byKey {
			update.DeleteScopeIDs = append(update.DeleteScopeIDs, scope.Id.ValueString())
		}
	}

	updateResp, err := pgql.ModifyMemoryStore(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "memory_store", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, updateResp.UpdateMemoryStore)...)

	logResourceOperation(ctx, "updated", "memory_store", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *MemoryStoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MemoryStoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "memory_store", data.Id.ValueString())

	if _, err := pgql.DeleteMemoryStore(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "memory_store", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "memory_store", data.Id.ValueString())
}

func (r *MemoryStoreResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// planScopes reads the scopes a configuration wrote.
func planScopes(ctx context.Context, scopes types.List) ([]memoryStoreScopeModel, diag.Diagnostics) {
	if scopes.IsNull() || scopes.IsUnknown() {
		return nil, nil
	}
	var out []memoryStoreScopeModel
	diags := scopes.ElementsAs(ctx, &out, false)
	return out, diags
}

// newScope is one scope as it is written, either with its store or into one.
func newScope(scope memoryStoreScopeModel) *pgql.NewMemoryStoreScope {
	return &pgql.NewMemoryStoreScope{
		Description:                 pgql.OptPtr(scope.Description.ValueStringPointer()),
		Kind:                        scope.Kind.ValueString(),
		Key:                         scope.Key.ValueString(),
		Value:                       pgql.OptPtr(scope.Value.ValueStringPointer()),
		Match:                       pgql.OptPtr(scope.Match.ValueStringPointer()),
		ExtractionEnabled:           scope.ExtractionEnabled.ValueBool(),
		ExtractionPrompt:            pgql.OptPtr(scope.ExtractionPrompt.ValueStringPointer()),
		ExtractionModelProfileID:    pgql.OptPtr(scope.ExtractionModelProfileId.ValueStringPointer()),
		ConsolidationEnabled:        scope.ConsolidationEnabled.ValueBool(),
		ConsolidationPrompt:         pgql.OptPtr(scope.ConsolidationPrompt.ValueStringPointer()),
		ConsolidationModelProfileID: pgql.OptPtr(scope.ConsolidationModelProfileId.ValueStringPointer()),
	}
}
