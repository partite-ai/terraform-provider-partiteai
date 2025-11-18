package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MeshInputResource{}
var _ resource.ResourceWithImportState = &MeshInputResource{}

func NewMeshInputResource() resource.Resource {
	return &MeshInputResource{
		BaseResource: NewBaseResource("mesh_input"),
	}
}

// MeshInputResource implements the mesh input resource.
type MeshInputResource struct {
	BaseResource
}

// MeshInputResourceModel describes the resource data model.
type MeshInputResourceModel struct {
	MeshId         types.String      `tfsdk:"mesh_id"`
	AgentId        types.String      `tfsdk:"agent_id"`
	AgentVersionId types.String      `tfsdk:"agent_version_id"`
	Name           types.String      `tfsdk:"name"`
	Responses      types.Set         `tfsdk:"responses"`
	CreatedAt      timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt      timetypes.RFC3339 `tfsdk:"updated_at"`
}

type inputResponseStruct struct {
	Name        string               `tfsdk:"name"`
	Description string               `tfsdk:"description"`
	MessageType jsontypes.Normalized `tfsdk:"message_type"`
}

func (r *inputResponseStruct) FromGQL(src pgql.InputDetailsInputResponsesInputResponse) diag.Diagnostics {
	r.Name = src.Name
	r.Description = src.Description
	return setFields(
		context.Background(),
		&r.MessageType, src.MessageType,
	)
}

func (r *MeshInputResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Mesh Input resource",
		Attributes: map[string]schema.Attribute{
			"name":       commonSchemas.name,
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
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

func (r *MeshInputResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	meshID, inputName, ok := strings.Cut(req.ID, ":")
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			"The import identifier must be in the format '<mesh_id>:<input_name>'.\n\n"+
				"Example: 'cmsh_1234567890abcdef:my_input'",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mesh_id"), meshID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), inputName)...)
}

func (r *MeshInputResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MeshInputResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "mesh_input", "")

	agentID, err := r.resolveAgentID(ctx, data.AgentId, data.AgentVersionId)
	if err != nil {
		resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Unable to resolve agent_id: %s", err))
		return
	}

	var responseStructs []inputResponseStruct
	resp.Diagnostics.Append(data.Responses.ElementsAs(ctx, &responseStructs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var responses []pgql.NewInputResponse
	for _, r := range responseStructs {
		responses = append(responses, pgql.NewInputResponse{
			Name:        r.Name,
			Description: r.Description,
			MessageType: r.MessageType.ValueString(),
		})
	}

	createResp, err := pgql.CreateInput(ctx, r.GetClient(), data.MeshId.ValueString(), &pgql.NewInput{
		Name:      data.Name.ValueString(),
		AgentID:   agentID,
		Responses: responses,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create mesh input")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, data.MeshId.ValueString(), createResp.AddMeshInput)...)

	logResourceOperation(ctx, "created", "mesh_input", data.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshInputResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeshInputResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "mesh_input", data.Name.ValueString())

	meshResp, err := pgql.GetMeshInputs(ctx, r.GetClient(), data.MeshId.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh_input", data.Name.ValueString())
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

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, data.MeshId.ValueString(), *input)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshInputResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MeshInputResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "mesh_input", planData.Name.ValueString())

	var update pgql.UpdateInput
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.AgentVersionId.Equal(stateData.AgentVersionId) && !planData.AgentVersionId.IsUnknown() {
		update.AgentVersionID = pgql.Opt(planData.AgentVersionId.ValueString())
	}
	if !planData.Responses.Equal(stateData.Responses) {
		var responseStructs []inputResponseStruct
		resp.Diagnostics.Append(planData.Responses.ElementsAs(ctx, &responseStructs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		var responses []pgql.NewInputResponse
		for _, r := range responseStructs {
			responses = append(responses, pgql.NewInputResponse{
				Name:        r.Name,
				Description: r.Description,
				MessageType: r.MessageType.ValueString(),
			})
		}
		update.Responses = pgql.Opt(responses)
	}

	updateResp, err := pgql.ModifyInput(ctx, r.GetClient(),
		planData.MeshId.ValueString(),
		planData.Name.ValueString(),
		&update,
	)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "mesh_input", planData.Name.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, planData.MeshId.ValueString(), updateResp.UpdateInput)...)

	logResourceOperation(ctx, "updated", "mesh_input", planData.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *MeshInputResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeshInputResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "mesh_input", data.Name.ValueString())

	_, err := pgql.DeleteInput(ctx, r.GetClient(),
		data.MeshId.ValueString(),
		data.Name.ValueString(),
	)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "mesh_input", data.Name.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "mesh_input", data.Name.ValueString())
}

func (r *MeshInputResource) mapResponse(ctx context.Context, data *MeshInputResourceModel, meshID string, input pgql.InputDetails) diag.Diagnostics {
	return setFields(
		ctx,
		&data.MeshId, meshID,
		&data.AgentId, input.AgentVersion.Agent.Id,
		&data.AgentVersionId, input.AgentVersion.Id,
		&data.Name, input.Name,
		&data.CreatedAt, input.CreatedAt,
		&data.UpdatedAt, input.UpdatedAt,
		&data.Responses, newStructSliceMappable(input.InputResponses, inputResponseStruct{}),
	)
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
		resp, err := pgql.GetAgentIDForVersion(ctx, r.GetClient(), agentVersionID.ValueString())
		if err != nil {
			return "", err
		}
		return resp.AgentVersion.Agent.Id, nil
	}
	return "", fmt.Errorf("either agent_id or agent_version_id must be set")
}
