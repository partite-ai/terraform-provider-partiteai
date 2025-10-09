package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AgentResource{}
var _ resource.ResourceWithImportState = &AgentResource{}

func NewAgentResource() resource.Resource {
	return &AgentResource{
		BaseResource: NewBaseResource("agent"),
	}
}

// AgentResource defines the resource implementation.
type AgentResource struct {
	BaseResource
}

type AgentResourceModel struct {
	AgentBaseModel
}

var memorySlotType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Memory slot name",
			Required:            true,
		},
		"description": schema.StringAttribute{
			MarkdownDescription: "Memory slot description",
			Optional:            true,
		},
		"memory_type": schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: "Schema defining the structure of data stored in this memory slot",
			Required:            true,
		},
		"memory_scope": schema.StringAttribute{
			MarkdownDescription: "Scope of the memory slot (e.g., 'conversation', 'user')",
			Required:            true,
			Validators: []validator.String{
				commonValidators.memoryScope,
			},
		},
	},
}

var mcpConnectionType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "MCP connection identifier",
			Required:            true,
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "MCP connection name",
			Computed:            true,
		},
		"filter_type": schema.StringAttribute{
			MarkdownDescription: "Type of filter applied to the MCP connection",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("NONE"),
		},
		"tool_names": schema.ListAttribute{
			ElementType:         types.StringType,
			MarkdownDescription: "List of tool names associated with the MCP connection",
			Optional:            true,
		},
		"tool_name_pattern": schema.StringAttribute{
			MarkdownDescription: "Pattern to match tool names associated with the MCP connection",
			Optional:            true,
		},
	},
}

func (r *AgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Agent resource",
		Attributes: standardSchemaAttributes(map[string]schema.Attribute{
			"version_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current version identifier of the agent",
			},
			"version_ordinal": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Current version ordinal of the agent",
			},
			"version_created_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the current version was created",
			},
			"mcp_connections": schema.SetNestedAttribute{
				MarkdownDescription: "List of MCP connections associated with the agent",
				Optional:            true,
				Computed:            true,
				NestedObject:        mcpConnectionType,
				Default:             setdefault.StaticValue(types.SetValueMust(mcpConnectionType.Type(), nil)),
			},
			"memory_slots": schema.SetNestedAttribute{
				MarkdownDescription: "List of memory slots for the agent",
				Optional:            true,
				Computed:            true,
				NestedObject:        memorySlotType,
				Default:             setdefault.StaticValue(types.SetValueMust(memorySlotType.Type(), nil)),
			},
			"identity": schema.StringAttribute{
				MarkdownDescription: "A brief description of the agent's identity, such as its role or persona.",
				Optional:            true,
			},
			"task_instructions": schema.StringAttribute{
				MarkdownDescription: "Detailed instructions that guide the agent's behavior and decision-making processes.",
				Required:            true,
			},
			"model_complexity": schema.StringAttribute{
				MarkdownDescription: "Defines the complexity of the model the agent uses.",
				Required:            true,
				Validators: []validator.String{
					commonValidators.modelComplexity,
				},
			},
			"input_message_type": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Schema defining the structure of input messages the agent can process",
				Required:            true,
			},
			"preserve_conversation_history": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent should retain conversation history for context in future interactions",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"send_thinking_to_user": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent should send 'thinking' messages to the user",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"send_messages_to_user": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent should send messages to the user",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"ask_questions_to_user": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent is allowed to ask questions to the user for clarification",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		}),
	}
}

func (r *AgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "agent", "")

	var tags []*pgql.Tag
	if diag := data.Tags.ElementsAs(ctx, &tags, false); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	var inputMessageType map[string]any
	resp.Diagnostics.Append(data.InputMessageType.Unmarshal(&inputMessageType)...)

	var memorySlots []pgql.NewMemorySlot
	if !data.MemorySlots.IsNull() {
		for _, v := range data.MemorySlots.Elements() {
			var memorySlot memorySlotInfo
			if diag := tfsdk.ValueAs(ctx, v, &memorySlot); diag.HasError() {
				resp.Diagnostics.Append(diag...)
				return
			}

			var memoryType map[string]any
			if err := json.Unmarshal([]byte(memorySlot.MemoryType.ValueString()), &memoryType); err != nil {
				resp.Diagnostics.AddError("Invalid JSON", fmt.Sprintf("Memory type contains invalid JSON: %s", err))
				return
			}

			memorySlots = append(memorySlots, pgql.NewMemorySlot{
				Name:        memorySlot.Name,
				Description: memorySlot.Description,
				MemoryType:  memoryType,
				Scope:       pgql.MemoryScope(memorySlot.MemoryScope),
				CanRead:     pgql.OptPtr(memorySlot.CanRead.ValueStringPointer()),
				CanWrite:    pgql.OptPtr(memorySlot.CanWrite.ValueStringPointer()),
			})
		}
	}

	var mcpConnections []pgql.NewMcpConnectionAgentAssociation
	if !data.McpConnections.IsNull() {
		var mcpConnectionStructs []mcpConnectionInfo
		if diag := data.McpConnections.ElementsAs(ctx, &mcpConnectionStructs, false); diag.HasError() {
			resp.Diagnostics.Append(diag...)
			return
		}
		for _, v := range mcpConnectionStructs {
			caa := pgql.NewMcpConnectionAgentAssociation{
				McpConnectionID: v.Id,
				FilterType:      pgql.McpConnectionFilterType(v.FilterType.ValueString()),
			}
			switch caa.FilterType {
			case pgql.McpConnectionFilterTypeList:
				var toolNames []string
				if !v.ToolNames.IsNull() {
					if diag := v.ToolNames.ElementsAs(ctx, &toolNames, false); diag.HasError() {
						resp.Diagnostics.Append(diag...)
						return
					}
				}
				caa.ToolNames = toolNames
			case pgql.McpConnectionFilterTypePattern:
				caa.ToolNamePattern = v.ToolNamePattern.ValueStringPointer()
			}
			mcpConnections = append(mcpConnections, caa)
		}
	}

	createResp, err := pgql.CreateAgent(ctx, r.GetClient(), &pgql.NewAgent{
		Name:                        data.Name.ValueString(),
		Description:                 pgql.OptPtr(data.Description.ValueStringPointer()),
		Tags:                        tags,
		Identity:                    pgql.OptPtr(data.Identity.ValueStringPointer()),
		TaskInstructions:            data.TaskInstructions.ValueString(),
		ModelComplexity:             pgql.ModelComplexity(data.ModelComplexity.ValueString()),
		InputMessageType:            inputMessageType,
		SendThinkingToUser:          pgql.OptPtr(data.SendThinkingToUser.ValueBoolPointer()),
		SendMessagesToUser:          pgql.OptPtr(data.SendMessagesToUser.ValueBoolPointer()),
		AskQuestionsToUser:          pgql.OptPtr(data.AskQuestionsToUser.ValueBoolPointer()),
		PreserveConversationHistory: pgql.OptPtr(data.PreserveConversationHistory.ValueBoolPointer()),
		MemorySlots:                 memorySlots,
		McpConnections:              mcpConnections,
	})

	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create agent")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, createResp.CreateAgent.Agent)...)

	logResourceOperation(ctx, "created", "agent", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "agent", data.Id.ValueString())

	agentResp, err := pgql.GetAgent(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "agent", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, *agentResp.Agent)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData AgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "agent", planData.Id.ValueString())

	var update pgql.UpdateAgent
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}

	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}

	if !planData.Tags.Equal(stateData.Tags) {
		var tags []*pgql.Tag
		if diag := planData.Tags.ElementsAs(ctx, &tags, false); diag.HasError() {
			resp.Diagnostics.Append(diag...)
			return
		}
		update.Tags = pgql.Opt(tags)
	}

	if !planData.Identity.Equal(stateData.Identity) {
		update.Identity = pgql.OptNil(planData.Identity.ValueStringPointer())
	}

	if !planData.TaskInstructions.Equal(stateData.TaskInstructions) {
		update.TaskInstructions = pgql.Opt(planData.TaskInstructions.ValueString())
	}

	if !planData.ModelComplexity.Equal(stateData.ModelComplexity) {
		update.ModelComplexity = pgql.Opt(pgql.ModelComplexity(planData.ModelComplexity.ValueString()))
	}

	if !planData.InputMessageType.Equal(stateData.InputMessageType) {
		var inputMessageType map[string]any
		if err := json.Unmarshal([]byte(planData.InputMessageType.ValueString()), &inputMessageType); err != nil {
			resp.Diagnostics.AddError("Invalid JSON", fmt.Sprintf("Input message type contains invalid JSON: %s", err))
			return
		}
		update.InputMessageType = pgql.Opt(inputMessageType)
	}

	if !planData.SendThinkingToUser.Equal(stateData.SendThinkingToUser) {
		update.SendThinkingToUser = pgql.OptPtr(planData.SendThinkingToUser.ValueBoolPointer())
	}

	if !planData.SendMessagesToUser.Equal(stateData.SendMessagesToUser) {
		update.SendMessagesToUser = pgql.OptPtr(planData.SendMessagesToUser.ValueBoolPointer())
	}

	if !planData.AskQuestionsToUser.Equal(stateData.AskQuestionsToUser) {
		update.AskQuestionsToUser = pgql.OptPtr(planData.AskQuestionsToUser.ValueBoolPointer())
	}

	if !planData.PreserveConversationHistory.Equal(stateData.PreserveConversationHistory) {
		update.PreserveConversationHistory = pgql.OptPtr(planData.PreserveConversationHistory.ValueBoolPointer())
	}

	if !planData.MemorySlots.Equal(stateData.MemorySlots) {
		var memorySlots []pgql.NewMemorySlot
		if !planData.MemorySlots.IsNull() {
			for _, v := range planData.MemorySlots.Elements() {
				var memorySlot memorySlotInfo
				if diag := tfsdk.ValueAs(ctx, v, &memorySlot); diag.HasError() {
					resp.Diagnostics.Append(diag...)
					return
				}

				var memoryType map[string]any
				if err := json.Unmarshal([]byte(memorySlot.MemoryType.ValueString()), &memoryType); err != nil {
					resp.Diagnostics.AddError("Invalid JSON", fmt.Sprintf("Memory type contains invalid JSON: %s", err))
					return
				}

				memorySlots = append(memorySlots, pgql.NewMemorySlot{
					Name:        memorySlot.Name,
					Description: memorySlot.Description,
					MemoryType:  memoryType,
					Scope:       pgql.MemoryScope(memorySlot.MemoryScope),
					CanRead:     pgql.OptPtr(memorySlot.CanRead.ValueStringPointer()),
					CanWrite:    pgql.OptPtr(memorySlot.CanWrite.ValueStringPointer()),
				})
			}
		}

		update.MemorySlots = pgql.Opt(memorySlots)
	}

	if !planData.McpConnections.Equal(stateData.McpConnections) {
		var mcpConnections []pgql.NewMcpConnectionAgentAssociation
		if !planData.McpConnections.IsNull() {
			var mcpConnectionStructs []mcpConnectionInfo
			if diag := planData.McpConnections.ElementsAs(ctx, &mcpConnectionStructs, false); diag.HasError() {
				resp.Diagnostics.Append(diag...)
				return
			}
			for _, v := range mcpConnectionStructs {
				caa := pgql.NewMcpConnectionAgentAssociation{
					McpConnectionID: v.Id,
					FilterType:      pgql.McpConnectionFilterType(v.FilterType.ValueString()),
				}
				switch caa.FilterType {
				case pgql.McpConnectionFilterTypeList:
					var toolNames []string
					if !v.ToolNames.IsNull() {
						if diag := v.ToolNames.ElementsAs(ctx, &toolNames, false); diag.HasError() {
							resp.Diagnostics.Append(diag...)
							return
						}
					}
					caa.ToolNames = toolNames
				case pgql.McpConnectionFilterTypePattern:
					caa.ToolNamePattern = v.ToolNamePattern.ValueStringPointer()
				}
				mcpConnections = append(mcpConnections, caa)
			}
		}
		update.McpConnections = pgql.Opt(mcpConnections)
	}

	updateResp, err := pgql.ModifyAgent(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "agent", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, updateResp.UpdateAgent.Agent)...)

	logResourceOperation(ctx, "updated", "agent", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *AgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "agent", data.Id.ValueString())

	_, err := pgql.DeleteAgent(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "agent", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "agent", data.Id.ValueString())
}

func (r *AgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AgentResource) mapResponse(ctx context.Context, data *AgentResourceModel, agentDetails pgql.AgentDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, agentDetails)
}
