package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AgentResource{}
var _ resource.ResourceWithImportState = &AgentResource{}

func NewAgentResource() resource.Resource {
	return &AgentResource{}
}

// AgentResource defines the resource implementation.
type AgentResource struct {
	client graphql.Client
}

// AgentResourceModel describes the resource data model.
type AgentResourceModel struct {
	Id                          types.String         `tfsdk:"id"`
	Name                        types.String         `tfsdk:"name"`
	Description                 types.String         `tfsdk:"description"`
	Tags                        types.List           `tfsdk:"tags"`
	CreatedAt                   timetypes.RFC3339    `tfsdk:"created_at"`
	UpdatedAt                   timetypes.RFC3339    `tfsdk:"updated_at"`
	VersionId                   types.String         `tfsdk:"version_id"`
	VersionOrdinal              types.Int64          `tfsdk:"version_ordinal"`
	VersionCreatedAt            timetypes.RFC3339    `tfsdk:"version_created_at"`
	McpConnections              types.Set            `tfsdk:"mcp_connections"`
	MemorySlots                 types.Set            `tfsdk:"memory_slots"`
	Identity                    types.String         `tfsdk:"identity"`
	TaskInstructions            types.String         `tfsdk:"task_instructions"`
	ModelComplexity             types.String         `tfsdk:"model_complexity"`
	InputMessageType            jsontypes.Normalized `tfsdk:"input_message_type"`
	PreserveConversationHistory types.Bool           `tfsdk:"preserve_conversation_history"`
	SendThinkingToUser          types.Bool           `tfsdk:"send_thinking_to_user"`
	SendMessagesToUser          types.Bool           `tfsdk:"send_messages_to_user"`
	AskQuestionsToUser          types.Bool           `tfsdk:"ask_questions_to_user"`
}

var memorySlotType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "Memory slot identifier",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
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
				stringvalidator.OneOf("GLOBAL", "LOCAL", "REQUEST"),
			},
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Timestamp when the memory slot was created",
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
	},
}

type mcpConnectionInfo struct {
	Id   string       `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type memorySlotInfo struct {
	Id          string               `json:"id" tfsdk:"id"`
	Name        string               `json:"name" tfsdk:"name"`
	Description string               `json:"description" tfsdk:"description"`
	MemoryType  jsontypes.Normalized `json:"memoryType" tfsdk:"memory_type"`
	MemoryScope string               `json:"memoryScope" tfsdk:"memory_scope"`
	CanRead     types.String         `json:"canRead" tfsdk:"can_read"`
	CanWrite    types.String         `json:"canWrite" tfsdk:"can_write"`
	CreatedAt   timetypes.RFC3339    `json:"createdAt" tfsdk:"created_at"`
}

func (r *AgentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (r *AgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	tagType := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Tag name",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "Tag value",
				Required:            true,
			},
		},
	}
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Agent resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Agent identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Agent name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Agent description",
				Optional:            true,
			},
			"tags": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(tagType.Type(), nil)),
				NestedObject:        tagType,
				MarkdownDescription: "Tags for the Agent",
			},
			"created_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the agent was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the agent was last updated",
			},
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
					stringvalidator.OneOf("LOW", "MEDIUM", "HIGH"),
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
		},
	}
}

func (r *AgentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(graphql.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected graphql.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *AgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var tags []*pgql.Tag
	if diag := data.Tags.ElementsAs(ctx, &tags, false); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	var inputMessageType map[string]any
	if err := json.Unmarshal([]byte(data.InputMessageType.ValueString()), &inputMessageType); err != nil {
		resp.Diagnostics.AddError("Invalid JSON", fmt.Sprintf("Input message type contains invalid JSON: %s", err))
		return
	}

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

	var mcpConnections []string
	if !data.McpConnections.IsNull() {
		var mcpConnectionStructs []mcpConnectionInfo
		if diag := data.McpConnections.ElementsAs(ctx, &mcpConnectionStructs, false); diag.HasError() {
			resp.Diagnostics.Append(diag...)
			return
		}
		for _, v := range mcpConnectionStructs {
			mcpConnections = append(mcpConnections, v.Id)
		}
	}

	createResp, err := pgql.CreateAgent(ctx, r.client, &pgql.NewAgent{
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
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create agent, got error: %s", err))
		return
	}

	if diag := r.mapFields(ctx, &data, createResp.CreateAgent.Agent); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	agentResp, err := pgql.GetAgent(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent, got error: %s", err))
		return
	}

	if diag := r.mapFields(ctx, &data, *agentResp.Agent); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData AgentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

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
		var mcpConnections []string
		if !planData.McpConnections.IsNull() {
			var mcpConnectionStructs []mcpConnectionInfo
			if diag := planData.McpConnections.ElementsAs(ctx, &mcpConnectionStructs, false); diag.HasError() {
				resp.Diagnostics.Append(diag...)
				return
			}
			for _, v := range mcpConnectionStructs {
				mcpConnections = append(mcpConnections, v.Id)
			}
		}
		update.McpConnections = pgql.Opt(mcpConnections)
	}

	updateResp, err := pgql.ModifyAgent(ctx, r.client, planData.Id.ValueString(), &update)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update agent, got error: %s", err))
		return
	}

	if diag := r.mapFields(ctx, &planData, updateResp.UpdateAgent.Agent); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *AgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := pgql.DeleteAgent(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete agent, got error: %s", err))
		return
	}
}

func (r *AgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AgentResource) mapFields(ctx context.Context, data *AgentResourceModel, agentDetails pgql.AgentDetails) diag.Diagnostics {
	var diag diag.Diagnostics
	data.Id = types.StringValue(agentDetails.Id)
	data.CreatedAt, diag = timetypes.NewRFC3339Value(agentDetails.CreatedAt)
	if diag.HasError() {
		return diag
	}
	data.UpdatedAt, diag = timetypes.NewRFC3339Value(agentDetails.UpdatedAt)
	if diag.HasError() {
		return diag
	}
	data.VersionId = types.StringValue(agentDetails.LatestVersion.Id)
	data.VersionOrdinal = types.Int64Value(int64(agentDetails.LatestVersion.Ordinal))
	data.VersionCreatedAt, diag = timetypes.NewRFC3339Value(agentDetails.LatestVersion.CreatedAt)
	if diag.HasError() {
		return diag
	}

	mcpConnInfos := make([]mcpConnectionInfo, 0, len(agentDetails.LatestVersion.McpConnections))
	for _, v := range agentDetails.LatestVersion.McpConnections {
		mcpConnInfos = append(mcpConnInfos, mcpConnectionInfo{
			Id:   v.Id,
			Name: types.StringValue(v.Name),
		})
	}
	if lv, diag := types.SetValueFrom(ctx, data.McpConnections.ElementType(ctx), mcpConnInfos); diag != nil {
		return diag
	} else {
		data.McpConnections = lv
	}

	memorySlotInfos := make([]memorySlotInfo, 0, len(agentDetails.LatestVersion.MemorySlots))
	for _, v := range agentDetails.LatestVersion.MemorySlots {
		createdAt, diag := timetypes.NewRFC3339Value(v.CreatedAt)
		if diag.HasError() {
			return diag
		}
		memorySlotSerialized, err := json.Marshal(v.MemoryType)
		if err != nil {
			diag.AddError("JSON Marshalling Error", fmt.Sprintf("Unable to marshal memory slot type to JSON: %s", err))
			return diag
		}
		memorySlotInfos = append(memorySlotInfos, memorySlotInfo{
			Id:          v.Id,
			Name:        v.Name,
			Description: v.Description,
			MemoryType:  jsontypes.NewNormalizedValue(string(memorySlotSerialized)),
			MemoryScope: string(v.MemoryScope),
			CanRead:     types.StringPointerValue(v.CanRead),
			CanWrite:    types.StringPointerValue(v.CanWrite),
			CreatedAt:   createdAt,
		})
	}
	if lv, diag := types.SetValueFrom(ctx, data.MemorySlots.ElementType(ctx), memorySlotInfos); diag != nil {
		return diag
	} else {
		data.MemorySlots = lv
	}

	data.Name = types.StringValue(agentDetails.Name)
	data.Description = types.StringPointerValue(agentDetails.Description)
	data.Tags, diag = types.ListValueFrom(ctx, data.Tags.ElementType(ctx), agentDetails.Tags)
	if diag.HasError() {
		return diag
	}
	data.Identity = types.StringPointerValue(agentDetails.LatestVersion.Identity)
	data.TaskInstructions = types.StringValue(agentDetails.LatestVersion.TaskInstructions)
	data.ModelComplexity = types.StringValue(string(agentDetails.LatestVersion.ModelComplexity))
	inputMessageTypeSerialized, err := json.Marshal(agentDetails.LatestVersion.InputMessageType)
	if err != nil {
		diag.AddError("JSON Marshalling Error", fmt.Sprintf("Unable to marshal input message type to JSON: %s", err))
		return diag
	}
	data.InputMessageType = jsontypes.NewNormalizedValue(string(inputMessageTypeSerialized))
	data.PreserveConversationHistory = types.BoolValue(agentDetails.LatestVersion.PreserveConversationHistory)
	data.SendThinkingToUser = types.BoolValue(agentDetails.LatestVersion.SendThinkingToUser)
	data.SendMessagesToUser = types.BoolValue(agentDetails.LatestVersion.SendMessagesToUser)
	data.AskQuestionsToUser = types.BoolValue(agentDetails.LatestVersion.AskQuestionsToUser)
	return diag
}
