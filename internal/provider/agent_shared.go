package provider

import (
	"context"

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
	ClientTasks                 types.Set            `tfsdk:"client_tasks"`
	Skills                      types.Set            `tfsdk:"skills"`
	Particles                   types.Set            `tfsdk:"particles"`
	MemoryInteractionProfileIds types.Set            `tfsdk:"memory_interaction_profile_ids"`
	Identity                    types.String         `tfsdk:"identity"`
	TaskInstructions            types.String         `tfsdk:"task_instructions"`
	ModelComplexity             types.String         `tfsdk:"model_complexity"`
	InputMessageType            jsontypes.Normalized `tfsdk:"input_message_type"`
	SendThinkingToUser          types.Bool           `tfsdk:"send_thinking_to_user"`
	SendMessagesToUser          types.Bool           `tfsdk:"send_messages_to_user"`
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
		&m.SendThinkingToUser, agentDetails.LatestVersion.SendThinkingToUser,
		&m.SendMessagesToUser, agentDetails.LatestVersion.SendMessagesToUser,
		&m.Tags, agentDetails.LatestVersion.Tags,
		&m.MemoryInteractionProfileIds, memoryInteractionProfileIDs(agentDetails.LatestVersion.MemoryInteractionProfiles),
		&m.McpConnections, newStructSliceMappable(agentDetails.LatestVersion.McpConnections, mcpConnectionInfo{}),
		&m.ClientTasks, newStructSliceMappable(agentDetails.LatestVersion.ClientTasks, clientTaskInfo{}),
		&m.Skills, newStructSliceMappable(agentDetails.LatestVersion.Skills, skillAssociationInfo{}),
		&m.Particles, newStructSliceMappable(agentDetails.LatestVersion.Particles, particleAssociationInfo{}),
	)...)

	return diags
}

type mcpConnectionInfo struct {
	Id              string       `tfsdk:"id"`
	FilterType      types.String `tfsdk:"filter_type"`
	ToolNames       types.List   `tfsdk:"tool_names"`
	ToolNamePattern types.String `tfsdk:"tool_name_pattern"`
}

func (t *mcpConnectionInfo) FromGQL(src pgql.AgentDetailsLatestVersionAgentVersionMcpConnectionsMcpConnectionAgentAssociation) diag.Diagnostics {
	var diags diag.Diagnostics
	t.Id = src.McpConnection.Id
	t.FilterType = types.StringValue(string(src.FilterType))
	switch src.FilterType {
	case pgql.ToolFilterTypeNone:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringNull()
	case pgql.ToolFilterTypeList:
		diags.Append(
			tfsdk.ValueFrom(context.Background(), src.ToolNames, types.ListType{ElemType: types.StringType}, &t.ToolNames)...,
		)
		t.ToolNamePattern = types.StringNull()
	case pgql.ToolFilterTypePattern:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringPointerValue(src.ToolNamePattern)
	default:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringNull()
	}
	return nil
}

type clientTaskInfo struct {
	Id string `tfsdk:"id"`
}

func (t *clientTaskInfo) FromGQL(src pgql.AgentDetailsLatestVersionAgentVersionClientTasksClientTaskAgentAssociation) diag.Diagnostics {
	t.Id = src.ClientTaskDefinition.Id
	return nil
}

type skillAssociationInfo struct {
	Id string `tfsdk:"id"`
}

func (s *skillAssociationInfo) FromGQL(src pgql.AgentDetailsLatestVersionAgentVersionSkillsSkillAgentAssociation) diag.Diagnostics {
	s.Id = src.Skill.Id
	return nil
}

// skillAssociations are the skills a configuration gives an agent.
func skillAssociations(ctx context.Context, skills types.Set) ([]pgql.NewSkillAgentAssociation, diag.Diagnostics) {
	out := []pgql.NewSkillAgentAssociation{}
	if skills.IsNull() || skills.IsUnknown() {
		return out, nil
	}
	var infos []skillAssociationInfo
	diags := skills.ElementsAs(ctx, &infos, false)
	for _, info := range infos {
		out = append(out, pgql.NewSkillAgentAssociation{SkillID: info.Id})
	}
	return out, diags
}

// memoryInteractionProfileIDs are the profiles one agent version holds, as the
// ids a configuration names them by.
func memoryInteractionProfileIDs(
	profiles []pgql.AgentDetailsLatestVersionAgentVersionMemoryInteractionProfilesMemoryInteractionProfile,
) []string {
	out := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profile.Id)
	}
	return out
}

type particleAssociationInfo struct {
	Id              string       `tfsdk:"id"`
	FilterType      types.String `tfsdk:"filter_type"`
	ToolNames       types.List   `tfsdk:"tool_names"`
	ToolNamePattern types.String `tfsdk:"tool_name_pattern"`
}

func (t *particleAssociationInfo) FromGQL(src pgql.AgentDetailsLatestVersionAgentVersionParticlesParticleAgentAssociation) diag.Diagnostics {
	var diags diag.Diagnostics
	t.Id = src.InstalledParticle.Id
	t.FilterType = types.StringValue(string(src.FilterType))
	switch src.FilterType {
	case pgql.ToolFilterTypeNone:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringNull()
	case pgql.ToolFilterTypeList:
		diags.Append(
			tfsdk.ValueFrom(context.Background(), src.ToolNames, types.ListType{ElemType: types.StringType}, &t.ToolNames)...,
		)
		t.ToolNamePattern = types.StringNull()
	case pgql.ToolFilterTypePattern:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringPointerValue(src.ToolNamePattern)
	default:
		t.ToolNames = types.ListNull(types.StringType)
		t.ToolNamePattern = types.StringNull()
	}
	return diags
}
