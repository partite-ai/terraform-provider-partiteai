package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
