package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &MemoryInteractionProfileResource{}
var _ resource.ResourceWithImportState = &MemoryInteractionProfileResource{}

func NewMemoryInteractionProfileResource() resource.Resource {
	return &MemoryInteractionProfileResource{
		BaseResource: NewBaseResource("memory_interaction_profile"),
	}
}

type MemoryInteractionProfileResource struct {
	BaseResource
}

type MemoryInteractionProfileResourceModel struct {
	MemoryInteractionProfileBaseModel
}

func (r *MemoryInteractionProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A memory interaction profile: what an agent is given from memory, and what it can call to read or write more. An agent can hold several, and a profile does nothing until the agent runs in a mesh that has a memory store.",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"created_at":  commonSchemas.createdAt,
			"updated_at":  commonSchemas.updatedAt,
			"allow_expansion": schema.BoolAttribute{
				MarkdownDescription: "Let the agent read around a record it was given: the quotes either side of a quote, the quote a fact came from, the facts behind an observation",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"conversation_history": schema.SingleNestedAttribute{
				MarkdownDescription: "Put what was said in this conversation in the agent's prompt",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"token_budget": schema.Int64Attribute{
						MarkdownDescription: "How many tokens of history the prompt can hold",
						Required:            true,
					},
					"selection": schema.StringAttribute{
						MarkdownDescription: "RECENT takes an unbroken run of turns ending at the newest one. RELEVANT searches the conversation for what the query is about.",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("RECENT"),
						Validators: []validator.String{
							stringvalidator.OneOf("RECENT", "RELEVANT"),
						},
					},
					"query": memoryQuerySourceSchema("What a RELEVANT selection searches for. A RECENT selection does not read it.", false),
				},
			},
			"context_injections": schema.ListNestedAttribute{
				MarkdownDescription: "What goes in the agent's prompt from memory, without the agent asking for it",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Identifies this injection to a person. It is not put in the prompt.",
							Required:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "What these memories are, written at the head of the section that holds them",
							Optional:            true,
						},
						"token_budget": schema.Int64Attribute{
							MarkdownDescription: "How many tokens of memory this injection can hold",
							Required:            true,
						},
						"record_types": memoryRecordTypesSchema(),
						"selection":    memorySelectionSchema(true),
						"query":        memoryQuerySourceSchema("What this injection searches memory for", true),
					},
				},
			},
			"query_tools": schema.ListNestedAttribute{
				MarkdownDescription: "The tools this agent can call to search memory",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tool_name": schema.StringAttribute{
							MarkdownDescription: "What the agent calls. A model tells two tools apart by name, so it has to be unique across the profiles one agent holds.",
							Required:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "What the tool is for, and when to call it",
							Required:            true,
						},
						"record_types": memoryRecordTypesSchema(),
						"selection":    memorySelectionSchema(true),
						"token_budget": schema.Int64Attribute{
							MarkdownDescription: "How many tokens one call returns. Give either this or effort_levels.",
							Optional:            true,
						},
						"effort_levels": schema.ListNestedAttribute{
							MarkdownDescription: "The amounts the agent can ask for by name. Give either this or token_budget.",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"level": schema.StringAttribute{
										MarkdownDescription: "What the model names this amount, such as \"quick\" or \"deep\"",
										Required:            true,
									},
									"token_budget": schema.Int64Attribute{
										MarkdownDescription: "How many tokens a call at this level returns",
										Required:            true,
									},
								},
							},
						},
					},
				},
			},
			"write_tools": schema.ListNestedAttribute{
				MarkdownDescription: "The tools this agent can call to write a memory",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tool_name": schema.StringAttribute{
							MarkdownDescription: "What the agent calls",
							Required:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "What the tool is for, and when to call it",
							Required:            true,
						},
						"tag_mappings": schema.ListNestedAttribute{
							MarkdownDescription: "The tags to put on what this tool writes. They apply in order, and a later one replaces the value an earlier one wrote under the same key.",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"kind": schema.StringAttribute{
										MarkdownDescription: "FIXED writes the one tag below. ALL_FROM_REQUEST copies every tag of the request. KEYS_FROM_REQUEST copies the keys named below.",
										Required:            true,
										Validators: []validator.String{
											stringvalidator.OneOf("FIXED", "ALL_FROM_REQUEST", "KEYS_FROM_REQUEST"),
										},
									},
									"keys": schema.ListAttribute{
										MarkdownDescription: "The request tag keys to copy. At least one is required for KEYS_FROM_REQUEST.",
										ElementType:         types.StringType,
										Optional:            true,
									},
									"tag_key": schema.StringAttribute{
										MarkdownDescription: "The key FIXED writes. Required for FIXED.",
										Optional:            true,
									},
									"tag_value": schema.StringAttribute{
										MarkdownDescription: "The value FIXED writes. Required for FIXED.",
										Optional:            true,
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

// memoryRecordTypesSchema is the kinds of record an injection or a query tool
// reads.
func memoryRecordTypesSchema() schema.Attribute {
	return schema.ListAttribute{
		MarkdownDescription: "The kinds of record to read: CHUNK for what was said, FACT for a claim taken out of it, OBSERVATION for a summary of facts. At least one is required.",
		ElementType:         types.StringType,
		Required:            true,
	}
}

// memorySelectionSchema is what narrows the memory one injection or tool
// reads.
func memorySelectionSchema(required bool) schema.Attribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "Which memories this reads. ALL reads every record of the store. TAGS reads the records that carry the tags below.",
		Required:            required,
		Optional:            !required,
		Attributes: map[string]schema.Attribute{
			"kind": schema.StringAttribute{
				MarkdownDescription: "ALL or TAGS",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("ALL", "TAGS"),
				},
			},
			"tags": schema.ListNestedAttribute{
				MarkdownDescription: "The tags to read by. At least one is required for TAGS, and ALL takes none.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"source": schema.StringAttribute{
							MarkdownDescription: "FIXED reads by the value below. REQUEST_TAG reads by whatever the request carries under request_tag_key.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("FIXED", "REQUEST_TAG"),
							},
						},
						"tag_key": schema.StringAttribute{
							MarkdownDescription: "The key this reads the store by",
							Required:            true,
						},
						"tag_value": schema.StringAttribute{
							MarkdownDescription: "The value FIXED reads by. Required for FIXED.",
							Optional:            true,
						},
						"request_tag_key": schema.StringAttribute{
							MarkdownDescription: "The key to read off the request. Required for REQUEST_TAG.",
							Optional:            true,
						},
					},
				},
			},
		},
	}
}

// memoryQuerySourceSchema is the text an injection or a history searches for.
func memoryQuerySourceSchema(description string, required bool) schema.Attribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: description,
		Required:            required,
		Optional:            !required,
		Attributes: map[string]schema.Attribute{
			"kind": schema.StringAttribute{
				MarkdownDescription: "FIXED searches for the text below. DERIVED searches for what the agent was given to work on.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("FIXED", "DERIVED"),
				},
			},
			"text": schema.StringAttribute{
				MarkdownDescription: "What FIXED searches for. Required for FIXED, and DERIVED does not read it.",
				Optional:            true,
			},
		},
	}
}

func (r *MemoryInteractionProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MemoryInteractionProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "memory_interaction_profile", "")

	create := profileInput(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := pgql.CreateMemoryInteractionProfile(ctx, r.GetClient(), &create)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create memory interaction profile")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, createResp.CreateMemoryInteractionProfile)...)

	logResourceOperation(ctx, "created", "memory_interaction_profile", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MemoryInteractionProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MemoryInteractionProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "memory_interaction_profile", data.Id.ValueString())

	profileResp, err := pgql.GetMemoryInteractionProfile(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "memory_interaction_profile", data.Id.ValueString())
		return
	}
	if profileResp.MemoryInteractionProfile == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *profileResp.MemoryInteractionProfile)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update sends each list that changed whole, because that is what the API
// takes: a list given replaces the list the profile held, and a list left out
// keeps it.
func (r *MemoryInteractionProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MemoryInteractionProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "memory_interaction_profile", planData.Id.ValueString())

	wanted := profileInput(ctx, planData, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var update pgql.UpdateMemoryInteractionProfile
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(wanted.Name)
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}
	if !planData.AllowExpansion.Equal(stateData.AllowExpansion) {
		update.AllowExpansion = pgql.Opt(wanted.AllowExpansion)
	}
	if !planData.ConversationHistory.Equal(stateData.ConversationHistory) {
		update.ConversationHistory = pgql.Opt(wanted.ConversationHistory)
	}
	if !planData.ContextInjections.Equal(stateData.ContextInjections) {
		update.ContextInjections = pgql.Opt(wanted.ContextInjections)
	}
	if !planData.QueryTools.Equal(stateData.QueryTools) {
		update.QueryTools = pgql.Opt(wanted.QueryTools)
	}
	if !planData.WriteTools.Equal(stateData.WriteTools) {
		update.WriteTools = pgql.Opt(wanted.WriteTools)
	}

	updateResp, err := pgql.ModifyMemoryInteractionProfile(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "memory_interaction_profile", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, updateResp.UpdateMemoryInteractionProfile)...)

	logResourceOperation(ctx, "updated", "memory_interaction_profile", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *MemoryInteractionProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MemoryInteractionProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "memory_interaction_profile", data.Id.ValueString())

	if _, err := pgql.DeleteMemoryInteractionProfile(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "memory_interaction_profile", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "memory_interaction_profile", data.Id.ValueString())
}

func (r *MemoryInteractionProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// profileInput reads a whole profile as a configuration wrote it, and an
// update takes the parts of it that changed. Each list is built non-nil, so
// that a configuration which empties one empties it on the server too.
func profileInput(
	ctx context.Context,
	data MemoryInteractionProfileResourceModel,
	diags *diag.Diagnostics,
) pgql.NewMemoryInteractionProfile {
	profile := pgql.NewMemoryInteractionProfile{
		Name:              data.Name.ValueString(),
		Description:       pgql.OptPtr(data.Description.ValueStringPointer()),
		AllowExpansion:    data.AllowExpansion.ValueBool(),
		ContextInjections: []*pgql.MemoryContextInjectionInput{},
		QueryTools:        []*pgql.MemoryQueryToolInput{},
		WriteTools:        []*pgql.MemoryWriteToolInput{},
	}

	if history, ok := objectAs[memoryConversationHistoryModel](ctx, data.ConversationHistory, diags); ok {
		profile.ConversationHistory = &pgql.MemoryConversationHistoryInput{
			TokenBudget: int(history.TokenBudget.ValueInt64()),
			Selection:   history.Selection.ValueString(),
			Query:       querySourceInput(ctx, history.Query, diags),
		}
	}

	for _, injection := range listAs[memoryContextInjectionModel](ctx, data.ContextInjections, diags) {
		profile.ContextInjections = append(profile.ContextInjections, &pgql.MemoryContextInjectionInput{
			Name:        injection.Name.ValueString(),
			Description: pgql.OptPtr(injection.Description.ValueStringPointer()),
			TokenBudget: int(injection.TokenBudget.ValueInt64()),
			RecordTypes: listAs[string](ctx, injection.RecordTypes, diags),
			Selection:   selectionInput(ctx, injection.Selection, diags),
			Query:       querySourceInput(ctx, injection.Query, diags),
		})
	}

	for _, queryTool := range listAs[memoryQueryToolModel](ctx, data.QueryTools, diags) {
		tool := &pgql.MemoryQueryToolInput{
			ToolName:    queryTool.ToolName.ValueString(),
			Description: queryTool.Description.ValueString(),
			RecordTypes: listAs[string](ctx, queryTool.RecordTypes, diags),
			Selection:   selectionInput(ctx, queryTool.Selection, diags),
		}
		if !queryTool.TokenBudget.IsNull() {
			tool.TokenBudget = pgql.Opt(int(queryTool.TokenBudget.ValueInt64()))
		}
		for _, level := range listAs[memoryEffortLevelModel](ctx, queryTool.EffortLevels, diags) {
			tool.EffortLevels = append(tool.EffortLevels, &pgql.MemoryEffortLevelInput{
				Level:       level.Level.ValueString(),
				TokenBudget: int(level.TokenBudget.ValueInt64()),
			})
		}
		profile.QueryTools = append(profile.QueryTools, tool)
	}

	for _, writeTool := range listAs[memoryWriteToolModel](ctx, data.WriteTools, diags) {
		tool := &pgql.MemoryWriteToolInput{
			ToolName:    writeTool.ToolName.ValueString(),
			Description: writeTool.Description.ValueString(),
		}
		for _, mapping := range listAs[memoryTagMappingModel](ctx, writeTool.TagMappings, diags) {
			tool.TagMappings = append(tool.TagMappings, &pgql.MemoryTagMappingInput{
				Kind:     mapping.Kind.ValueString(),
				Keys:     listAs[string](ctx, mapping.Keys, diags),
				TagKey:   pgql.OptPtr(mapping.TagKey.ValueStringPointer()),
				TagValue: pgql.OptPtr(mapping.TagValue.ValueStringPointer()),
			})
		}
		profile.WriteTools = append(profile.WriteTools, tool)
	}

	return profile
}

// selectionInput is what narrows the memory one injection or tool reads.
func selectionInput(ctx context.Context, object types.Object, diags *diag.Diagnostics) *pgql.MemorySelectionInput {
	selection, ok := objectAs[memorySelectionModel](ctx, object, diags)
	if !ok {
		return nil
	}
	input := &pgql.MemorySelectionInput{Kind: selection.Kind.ValueString()}
	for _, tag := range listAs[memorySelectionTagModel](ctx, selection.Tags, diags) {
		input.Tags = append(input.Tags, &pgql.MemorySelectionTagInput{
			Source:        tag.Source.ValueString(),
			TagKey:        tag.TagKey.ValueString(),
			TagValue:      pgql.OptPtr(tag.TagValue.ValueStringPointer()),
			RequestTagKey: pgql.OptPtr(tag.RequestTagKey.ValueStringPointer()),
		})
	}
	return input
}

// querySourceInput is the text an injection or a history searches for.
func querySourceInput(ctx context.Context, object types.Object, diags *diag.Diagnostics) *pgql.MemoryQuerySourceInput {
	source, ok := objectAs[memoryQuerySourceModel](ctx, object, diags)
	if !ok {
		return nil
	}
	return &pgql.MemoryQuerySourceInput{
		Kind: source.Kind.ValueString(),
		Text: pgql.OptPtr(source.Text.ValueStringPointer()),
	}
}

// objectAs reads one nested block a configuration wrote. It returns false
// where the configuration left the block out.
func objectAs[T any](ctx context.Context, object types.Object, diags *diag.Diagnostics) (T, bool) {
	var out T
	if object.IsNull() || object.IsUnknown() {
		return out, false
	}
	diags.Append(object.As(ctx, &out, basetypes.ObjectAsOptions{})...)
	return out, true
}

// listAs reads a list a configuration wrote.
func listAs[T any](ctx context.Context, list types.List, diags *diag.Diagnostics) []T {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var out []T
	diags.Append(list.ElementsAs(ctx, &out, false)...)
	return out
}
