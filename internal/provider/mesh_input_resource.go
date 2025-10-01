package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MeshInputResource{}
var _ resource.ResourceWithImportState = &MeshInputResource{}

func NewMeshInputResource() resource.Resource {
	return &MeshInputResource{}
}

// MeshInputResource implements the mesh input resource.
type MeshInputResource struct {
	client graphql.Client
}

// MeshInputResourceModel describes the resource data model.
type MeshInputResourceModel struct {
	MeshId         types.String `tfsdk:"mesh_id"`
	AgentId        types.String `tfsdk:"agent_id"`
	AgentVersionId types.String `tfsdk:"agent_version_id"`
	Name           types.String `tfsdk:"name"`
	Responses      types.Set    `tfsdk:"responses"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *MeshInputResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mesh_input"
}

func (r *MeshInputResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Mesh Input resource",
		Attributes: map[string]schema.Attribute{
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Mesh ID",
			},
			"agent_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Agent ID. Either agent_id or agent_version_id must be set.",
			},
			"agent_version_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Agent Version ID. Either agent_id or agent_version_id must be set.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Input name (unique within mesh)",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the input was created",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the input was last updated",
			},
			"responses": schema.SetNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Response name",
						},
						"description": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Response description",
						},
						"message_type": schema.StringAttribute{
							CustomType:          jsontypes.NormalizedType{},
							Required:            true,
							MarkdownDescription: "Response message type in JSON Schema format",
						},
					},
				},
			},
		},
	}
}

func (r *MeshInputResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MeshInputResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MeshInputResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentID, err := r.resolveAgentID(ctx, data.AgentId, data.AgentVersionId)
	if err != nil {
		resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Unable to resolve agent_id: %s", err))
		return
	}

	var responseStructs []struct {
		Name        string `tfsdk:"name"`
		Description string `tfsdk:"description"`
		MessageType string `tfsdk:"message_type"`
	}
	resp.Diagnostics.Append(data.Responses.ElementsAs(ctx, &responseStructs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var responses []pgql.NewInputResponse
	for _, r := range responseStructs {
		var messageType map[string]interface{}
		if err := json.Unmarshal([]byte(r.MessageType), &messageType); err != nil {
			resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Invalid JSON for response message_type: %s", err))
			return
		}
		responses = append(responses, pgql.NewInputResponse{
			Name:        r.Name,
			Description: r.Description,
			MessageType: messageType,
		})
	}

	createResp, err := pgql.CreateInput(ctx, r.client, data.MeshId.ValueString(), &pgql.NewInput{
		Name:      data.Name.ValueString(),
		AgentID:   agentID,
		Responses: responses,
	})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create mesh input, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(r.mapFields(ctx, &data, data.MeshId.ValueString(), createResp.AddMeshInput)...)
	tflog.Trace(ctx, "created a mesh input resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshInputResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeshInputResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	meshResp, err := pgql.GetMeshInputs(ctx, r.client, data.MeshId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read mesh inputs, got error: %s", err))
		return
	}
	var input *pgql.InputDetails
	for _, i := range meshResp.Mesh.DraftVersion.Inputs {
		if i.Name == data.Name.ValueString() {
			input = &i
			break
		}
	}
	if input == nil {
		resp.Diagnostics.AddError("Not Found", "Mesh input not found")
		return
	}
	resp.Diagnostics.Append(r.mapFields(ctx, &data, data.MeshId.ValueString(), *input)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshInputResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MeshInputResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var update pgql.UpdateInput
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.AgentVersionId.Equal(stateData.AgentVersionId) && !planData.AgentVersionId.IsUnknown() {
		update.AgentVersionID = pgql.Opt(planData.AgentVersionId.ValueString())
	}
	if !planData.Responses.Equal(stateData.Responses) {
		var responseStructs []struct {
			Name        string `tfsdk:"name"`
			Description string `tfsdk:"description"`
			MessageType string `tfsdk:"message_type"`
		}
		resp.Diagnostics.Append(planData.Responses.ElementsAs(ctx, &responseStructs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		var responses []pgql.NewInputResponse
		for _, r := range responseStructs {
			var messageType map[string]interface{}
			if err := json.Unmarshal([]byte(r.MessageType), &messageType); err != nil {
				resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Invalid JSON for response message_type: %s", err))
				return
			}
			responses = append(responses, pgql.NewInputResponse{
				Name:        r.Name,
				Description: r.Description,
				MessageType: messageType,
			})
		}
		update.Responses = pgql.Opt(responses)
	}

	updateResp, err := pgql.ModifyInput(ctx, r.client,
		planData.MeshId.ValueString(),
		planData.Name.ValueString(),
		&update,
	)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update mesh input, got error: %s", err))
		return
	}
	resp.Diagnostics.Append(r.mapFields(ctx, &planData, planData.MeshId.ValueString(), updateResp.UpdateInput)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *MeshInputResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeshInputResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := pgql.DeleteInput(ctx, r.client,
		data.MeshId.ValueString(),
		data.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete mesh input, got error: %s", err))
		return
	}
}

func (r *MeshInputResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *MeshInputResource) mapFields(ctx context.Context, data *MeshInputResourceModel, meshID string, input pgql.InputDetails) diag.Diagnostics {
	data.MeshId = types.StringValue(meshID)
	data.AgentId = types.StringValue(input.AgentVersion.Agent.Id)
	data.AgentVersionId = types.StringValue(input.AgentVersion.Id)
	data.Name = types.StringValue(input.Name)
	data.CreatedAt = types.StringValue(input.CreatedAt)
	data.UpdatedAt = types.StringValue(input.UpdatedAt)
	type responseStruct struct {
		Name        string `tfsdk:"name"`
		Description string `tfsdk:"description"`
		MessageType string `tfsdk:"message_type"`
	}

	var responseStructs []responseStruct
	for _, r := range input.InputResponses {
		messageTypeBytes, _ := json.Marshal(r.MessageType)
		responseStructs = append(responseStructs, responseStruct{
			Name:        r.Name,
			Description: r.Description,
			MessageType: string(messageTypeBytes),
		})
	}

	var responses types.Set
	diag := tfsdk.ValueFrom(ctx, responseStructs, types.SetType{
		ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"name":         types.StringType,
			"description":  types.StringType,
			"message_type": jsontypes.NormalizedType{},
		}},
	}, &responses)
	data.Responses = responses
	return diag
}

// resolveAgentID returns agentID and agentVersionID, ensuring only one is set in config, and computes the other if needed.
func (r *MeshInputResource) resolveAgentID(ctx context.Context, agentID types.String, agentVersionID types.String) (string, error) {
	agentIDSet := !agentID.IsNull() && !agentID.IsUnknown() && agentID.ValueString() != ""
	agentVersionIDSet := !agentVersionID.IsNull() && !agentVersionID.IsUnknown() && agentVersionID.ValueString() != ""
	if agentIDSet && agentVersionIDSet {
		return "", fmt.Errorf("only one of agent_id or agent_version_id should be set")
	}
	if agentIDSet {
		return agentID.ValueString(), nil
	}
	if agentVersionIDSet {
		resp, err := pgql.GetAgentIDForVersion(ctx, r.client, agentVersionID.ValueString())
		if err != nil {
			return "", err
		}
		return resp.AgentVersion.Agent.Id, nil
	}
	return "", fmt.Errorf("either agent_id or agent_version_id must be set")
}
