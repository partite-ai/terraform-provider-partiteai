package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

var mcpConnectionType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "MCP connection identifier",
			Required:            true,
		},
		"filter_type": schema.StringAttribute{
			MarkdownDescription: "Type of filter applied to the MCP connection. One of 'NONE', 'LIST', or 'PATTERN'.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf("NONE", "LIST", "PATTERN"),
			},
		},
		"tool_names": schema.ListAttribute{
			ElementType:         types.StringType,
			MarkdownDescription: "List of tool names associated with the MCP connection",
			Optional:            true,
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
				listvalidator.UniqueValues(),
				listvalidator.NoNullValues(),
			},
		},
		"tool_name_pattern": schema.StringAttribute{
			MarkdownDescription: "Pattern to match tool names associated with the MCP connection",
			Optional:            true,
		},
	},
}

var taskAssociationType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "Task definition identifier",
			Required:            true,
		},
	},
}

var skillAssociationType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "Skill identifier",
			Required:            true,
		},
	},
}

var particleAssociationType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "Installed particle identifier",
			Required:            true,
		},
		"filter_type": schema.StringAttribute{
			MarkdownDescription: "Tool filter type. One of 'NONE', 'LIST', or 'PATTERN'.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf("NONE", "LIST", "PATTERN"),
			},
		},
		"tool_names": schema.ListAttribute{
			ElementType:         types.StringType,
			MarkdownDescription: "Tool names to expose. Required (and non-empty) when filter_type is LIST.",
			Optional:            true,
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
				listvalidator.UniqueValues(),
				listvalidator.NoNullValues(),
			},
		},
		"tool_name_pattern": schema.StringAttribute{
			MarkdownDescription: "Regex matched against tool names. Required when filter_type is PATTERN.",
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
			"tasks": schema.SetNestedAttribute{
				MarkdownDescription: "Set of task definitions associated with the agent, referenced by task definition ID",
				Optional:            true,
				Computed:            true,
				NestedObject:        taskAssociationType,
				Default:             setdefault.StaticValue(types.SetValueMust(taskAssociationType.Type(), nil)),
			},
			"skills": schema.SetNestedAttribute{
				MarkdownDescription: "Set of skills the agent can load, referenced by skill ID. No two of them can " +
					"declare the same name.",
				Optional:     true,
				Computed:     true,
				NestedObject: skillAssociationType,
				Default:      setdefault.StaticValue(types.SetValueMust(skillAssociationType.Type(), nil)),
			},
			"particles": schema.SetNestedAttribute{
				MarkdownDescription: "List of installed particles exposed to the agent.",
				Optional:            true,
				Computed:            true,
				NestedObject:        particleAssociationType,
				Default:             setdefault.StaticValue(types.SetValueMust(particleAssociationType.Type(), nil)),
			},
			"memory_interaction_profile_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of the memory interaction profiles this agent holds. A profile decides what memory goes in the agent's prompt and which memory tools it can call.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
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

	memoryProfiles, diags := memoryProfileAssociations(ctx, data.MemoryInteractionProfileIds)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
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
				FilterType:      pgql.ToolFilterType(v.FilterType.ValueString()),
			}
			switch caa.FilterType {
			case pgql.ToolFilterTypeList:
				var toolNames []string
				if !v.ToolNames.IsNull() {
					if diag := v.ToolNames.ElementsAs(ctx, &toolNames, false); diag.HasError() {
						resp.Diagnostics.Append(diag...)
						return
					}
				}
				caa.ToolNames = toolNames
			case pgql.ToolFilterTypePattern:
				caa.ToolNamePattern = v.ToolNamePattern.ValueStringPointer()
			}
			mcpConnections = append(mcpConnections, caa)
		}
	}

	var tasks []pgql.NewTaskAgentAssociation
	if !data.Tasks.IsNull() {
		var taskStructs []taskInfo
		if diag := data.Tasks.ElementsAs(ctx, &taskStructs, false); diag.HasError() {
			resp.Diagnostics.Append(diag...)
			return
		}
		for _, v := range taskStructs {
			tasks = append(tasks, pgql.NewTaskAgentAssociation{TaskDefinitionID: v.Id})
		}
	}

	skills, diags := skillAssociations(ctx, data.Skills)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var particles []pgql.NewParticleAgentAssociation
	if !data.Particles.IsNull() {
		var particleStructs []particleAssociationInfo
		if diag := data.Particles.ElementsAs(ctx, &particleStructs, false); diag.HasError() {
			resp.Diagnostics.Append(diag...)
			return
		}
		for _, v := range particleStructs {
			pa := pgql.NewParticleAgentAssociation{
				InstalledParticleID: v.Id,
				FilterType:          pgql.ToolFilterType(v.FilterType.ValueString()),
			}
			switch pa.FilterType {
			case pgql.ToolFilterTypeList:
				var toolNames []string
				if !v.ToolNames.IsNull() {
					if diag := v.ToolNames.ElementsAs(ctx, &toolNames, false); diag.HasError() {
						resp.Diagnostics.Append(diag...)
						return
					}
				}
				pa.ToolNames = toolNames
			case pgql.ToolFilterTypePattern:
				pa.ToolNamePattern = v.ToolNamePattern.ValueStringPointer()
			}
			particles = append(particles, pa)
		}
	}

	createResp, err := pgql.CreateAgent(ctx, r.GetClient(), &pgql.NewAgent{
		Name:                      data.Name.ValueString(),
		Description:               pgql.OptPtr(data.Description.ValueStringPointer()),
		Tags:                      tags,
		Identity:                  pgql.OptPtr(data.Identity.ValueStringPointer()),
		TaskInstructions:          data.TaskInstructions.ValueString(),
		ModelComplexity:           pgql.ModelComplexity(data.ModelComplexity.ValueString()),
		InputMessageType:          data.InputMessageType.ValueString(),
		SendThinkingToUser:        pgql.OptPtr(data.SendThinkingToUser.ValueBoolPointer()),
		SendMessagesToUser:        pgql.OptPtr(data.SendMessagesToUser.ValueBoolPointer()),
		McpConnections:            mcpConnections,
		Particles:                 particles,
		MemoryInteractionProfiles: memoryProfiles,
		Tasks:                     tasks,
		Skills:                    skills,
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
		update.InputMessageType = pgql.Opt(planData.InputMessageType.ValueString())
	}

	if !planData.SendThinkingToUser.Equal(stateData.SendThinkingToUser) {
		update.SendThinkingToUser = pgql.OptPtr(planData.SendThinkingToUser.ValueBoolPointer())
	}

	if !planData.SendMessagesToUser.Equal(stateData.SendMessagesToUser) {
		update.SendMessagesToUser = pgql.OptPtr(planData.SendMessagesToUser.ValueBoolPointer())
	}

	if !planData.MemoryInteractionProfileIds.Equal(stateData.MemoryInteractionProfileIds) {
		profiles, diags := memoryProfileAssociations(ctx, planData.MemoryInteractionProfileIds)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		update.MemoryInteractionProfiles = pgql.Opt(profiles)
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
					FilterType:      pgql.ToolFilterType(v.FilterType.ValueString()),
				}
				switch caa.FilterType {
				case pgql.ToolFilterTypeList:
					var toolNames []string
					if !v.ToolNames.IsNull() {
						if diag := v.ToolNames.ElementsAs(ctx, &toolNames, false); diag.HasError() {
							resp.Diagnostics.Append(diag...)
							return
						}
					}
					caa.ToolNames = toolNames
				case pgql.ToolFilterTypePattern:
					caa.ToolNamePattern = v.ToolNamePattern.ValueStringPointer()
				}
				mcpConnections = append(mcpConnections, caa)
			}
		}
		update.McpConnections = pgql.Opt(mcpConnections)
	}

	if !planData.Tasks.Equal(stateData.Tasks) {
		var tasks []pgql.NewTaskAgentAssociation
		if !planData.Tasks.IsNull() {
			var taskStructs []taskInfo
			if diag := planData.Tasks.ElementsAs(ctx, &taskStructs, false); diag.HasError() {
				resp.Diagnostics.Append(diag...)
				return
			}
			for _, v := range taskStructs {
				tasks = append(tasks, pgql.NewTaskAgentAssociation{TaskDefinitionID: v.Id})
			}
		}
		update.Tasks = pgql.Opt(tasks)
	}

	if !planData.Skills.Equal(stateData.Skills) {
		skills, diags := skillAssociations(ctx, planData.Skills)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		update.Skills = pgql.Opt(skills)
	}

	if !planData.Particles.Equal(stateData.Particles) {
		var particles []pgql.NewParticleAgentAssociation
		if !planData.Particles.IsNull() {
			var particleStructs []particleAssociationInfo
			if diag := planData.Particles.ElementsAs(ctx, &particleStructs, false); diag.HasError() {
				resp.Diagnostics.Append(diag...)
				return
			}
			for _, v := range particleStructs {
				pa := pgql.NewParticleAgentAssociation{
					InstalledParticleID: v.Id,
					FilterType:          pgql.ToolFilterType(v.FilterType.ValueString()),
				}
				switch pa.FilterType {
				case pgql.ToolFilterTypeList:
					var toolNames []string
					if !v.ToolNames.IsNull() {
						if diag := v.ToolNames.ElementsAs(ctx, &toolNames, false); diag.HasError() {
							resp.Diagnostics.Append(diag...)
							return
						}
					}
					pa.ToolNames = toolNames
				case pgql.ToolFilterTypePattern:
					pa.ToolNamePattern = v.ToolNamePattern.ValueStringPointer()
				}
				particles = append(particles, pa)
			}
		}
		update.Particles = pgql.Opt(particles)
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

// memoryProfileAssociations turns the profile ids a configuration names into
// the associations one agent version is written with.
func memoryProfileAssociations(
	ctx context.Context,
	ids types.Set,
) ([]pgql.NewMemoryInteractionProfileAgentAssociation, diag.Diagnostics) {
	if ids.IsNull() || ids.IsUnknown() {
		return nil, nil
	}
	var named []string
	if diags := ids.ElementsAs(ctx, &named, false); diags.HasError() {
		return nil, diags
	}
	out := make([]pgql.NewMemoryInteractionProfileAgentAssociation, 0, len(named))
	for _, id := range named {
		out = append(out, pgql.NewMemoryInteractionProfileAgentAssociation{
			MemoryInteractionProfileID: id,
		})
	}
	return out, nil
}
