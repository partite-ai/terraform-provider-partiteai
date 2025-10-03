package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AgentDataSource{}

func NewAgentDataSource() datasource.DataSource {
	return &AgentDataSource{
		BaseDataSource: NewBaseDataSource("agent"),
	}
}

// AgentDataSource defines the data source implementation.
type AgentDataSource struct {
	BaseDataSource
}

// AgentDataSourceModel extends the base model with data source-specific fields.
type AgentDataSourceModel struct {
	AgentBaseModel
}

func (d *AgentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Agent data source",
		Attributes: standardDataSourceSchemaAttributes(map[string]schema.Attribute{
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
				Computed:            true,
				NestedObject:        mcpConnectionDataSourceType,
			},
			"memory_slots": schema.SetNestedAttribute{
				MarkdownDescription: "List of memory slots for the agent",
				Computed:            true,
				NestedObject:        memorySlotDataSourceType,
			},
			"identity": schema.StringAttribute{
				MarkdownDescription: "A brief description of the agent's identity, such as its role or persona.",
				Computed:            true,
			},
			"task_instructions": schema.StringAttribute{
				MarkdownDescription: "Detailed instructions that guide the agent's behavior and decision-making processes.",
				Computed:            true,
			},
			"model_complexity": schema.StringAttribute{
				MarkdownDescription: "Defines the complexity of the model the agent uses.",
				Computed:            true,
			},
			"input_message_type": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Schema defining the structure of input messages the agent can process",
				Computed:            true,
			},
			"preserve_conversation_history": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent should retain conversation history for context in future interactions",
				Computed:            true,
			},
			"send_thinking_to_user": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent should send 'thinking' messages to the user",
				Computed:            true,
			},
			"send_messages_to_user": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent should send messages to the user",
				Computed:            true,
			},
			"ask_questions_to_user": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent should ask questions to the user",
				Computed:            true,
			},
		}),
	}
}

func (d *AgentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "agent", data.Id.ValueString())

	agentResp, err := pgql.GetAgent(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "agent", data.Id.ValueString())
		return
	}

	// Use the shared mapping function directly
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *agentResp.Agent)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
