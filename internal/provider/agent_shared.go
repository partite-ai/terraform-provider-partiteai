package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// AgentBaseModel contains all the common fields for both resource and data source.
type AgentBaseModel struct {
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

// MapFromGraphQL maps GraphQL response to the base model.
func (m *AgentBaseModel) MapFromGraphQL(ctx context.Context, agentDetails pgql.AgentDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	// Use setFields for standard fields
	diags.Append(setFields(
		ctx,
		&m.Id, agentDetails.Id,
		&m.Name, agentDetails.Name,
		&m.Description, agentDetails.Description,
		&m.CreatedAt, agentDetails.CreatedAt,
		&m.UpdatedAt, agentDetails.UpdatedAt,
		&m.VersionId, agentDetails.LatestVersion.Id,
		&m.VersionOrdinal, int64(agentDetails.LatestVersion.Ordinal),
		&m.VersionCreatedAt, agentDetails.LatestVersion.CreatedAt,
		&m.Identity, agentDetails.LatestVersion.Identity,
		&m.TaskInstructions, agentDetails.LatestVersion.TaskInstructions,
		&m.ModelComplexity, string(agentDetails.LatestVersion.ModelComplexity),
		&m.InputMessageType, agentDetails.LatestVersion.InputMessageType,
		&m.PreserveConversationHistory, agentDetails.LatestVersion.PreserveConversationHistory,
		&m.SendThinkingToUser, agentDetails.LatestVersion.SendThinkingToUser,
		&m.SendMessagesToUser, agentDetails.LatestVersion.SendMessagesToUser,
		&m.AskQuestionsToUser, agentDetails.LatestVersion.AskQuestionsToUser,
		&m.Tags, agentDetails.Tags,
		&m.MemorySlots, newStructSliceMappable(agentDetails.LatestVersion.MemorySlots, memorySlotInfo{}),
		&m.McpConnections, newStructSliceMappable(agentDetails.LatestVersion.McpConnections, mcpConnectionInfo{}),
	)...)

	return diags
}

type mcpConnectionInfo struct {
	Id              string       `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	FilterType      types.String `tfsdk:"filter_type"`
	ToolNames       types.List   `tfsdk:"tool_names"`
	ToolNamePattern types.String `tfsdk:"tool_name_pattern"`
}

func (t *mcpConnectionInfo) FromGQL(src pgql.AgentDetailsLatestVersionAgentVersionMcpConnectionsMcpConnectionAgentAssociation) diag.Diagnostics {
	var diags diag.Diagnostics
	t.Id = src.McpConnection.Id
	t.Name = types.StringValue(src.McpConnection.Name)
	t.FilterType = types.StringValue(string(src.FilterType))
	switch src.FilterType {
	case pgql.McpConnectionFilterTypeNone:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringNull()
	case pgql.McpConnectionFilterTypeList:
		diags.Append(
			tfsdk.ValueFrom(context.Background(), src.ToolNames, types.ListType{ElemType: types.StringType}, &t.ToolNames)...,
		)
		t.ToolNamePattern = types.StringNull()
	case pgql.McpConnectionFilterTypePattern:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringPointerValue(src.ToolNamePattern)
	default:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringNull()
	}
	return nil
}

type memorySlotInfo struct {
	Name        string               `json:"name" tfsdk:"name"`
	Description string               `json:"description" tfsdk:"description"`
	MemoryType  jsontypes.Normalized `json:"memoryType" tfsdk:"memory_type"`
	MemoryScope string               `json:"memoryScope" tfsdk:"memory_scope"`
	CanRead     types.String         `json:"canRead" tfsdk:"can_read"`
	CanWrite    types.String         `json:"canWrite" tfsdk:"can_write"`
}

func (t *memorySlotInfo) FromGQL(src pgql.AgentDetailsLatestVersionAgentVersionMemorySlotsMemorySlot) diag.Diagnostics {
	var diags diag.Diagnostics
	t.Name = src.Name
	t.Description = src.Description

	smt, err := json.Marshal(src.MemoryType)
	if err != nil {
		diags.AddError("Internal Error", fmt.Sprintf("Failed to marshal MemoryType: %s", err))
		return diags
	}
	t.MemoryType = jsontypes.NewNormalizedValue(string(smt))

	t.MemoryScope = string(src.MemoryScope)
	t.CanRead = types.StringPointerValue(src.CanRead)
	t.CanWrite = types.StringPointerValue(src.CanWrite)

	return diags
}
