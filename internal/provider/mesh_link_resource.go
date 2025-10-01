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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MeshLinkResource{}
var _ resource.ResourceWithImportState = &MeshLinkResource{}

func NewMeshLinkResource() resource.Resource {
	return &MeshLinkResource{}
}

// MeshLinkResource implements the mesh link resource.
type MeshLinkResource struct {
	client graphql.Client
}

// MeshLinkResourceModel describes the resource data model.
type MeshLinkResourceModel struct {
	Id                 types.String `tfsdk:"id"`
	MeshId             types.String `tfsdk:"mesh_id"`
	FromAgentId        types.String `tfsdk:"from_agent_id"`
	FromAgentVersionId types.String `tfsdk:"from_agent_version_id"`
	ToAgentId          types.String `tfsdk:"to_agent_id"`
	ToAgentVersionId   types.String `tfsdk:"to_agent_version_id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	TransfersControl   types.Bool   `tfsdk:"transfers_control"`
	Responses          types.Set    `tfsdk:"responses"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func (r *MeshLinkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mesh_link"
}

func (r *MeshLinkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Mesh Link resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the link",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Mesh ID",
			},
			"from_agent_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "From agent ID",
			},
			"from_agent_version_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "From agent version ID",
			},
			"to_agent_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "To agent ID",
			},
			"to_agent_version_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "To agent version ID",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Link name",
			},
			"description": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Link description",
			},
			"transfers_control": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the link transfers control",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the link was created",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the link was last updated",
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

func (r *MeshLinkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MeshLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MeshLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fromAgentID, err := r.resolveAgentID(ctx, data.FromAgentId, data.FromAgentVersionId)
	if err != nil {
		resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Unable to resolve from_agent_id: %s", err))
		return
	}
	toAgentID, err := r.resolveAgentID(ctx, data.ToAgentId, data.ToAgentVersionId)
	if err != nil {
		resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Unable to resolve to_agent_id: %s", err))
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
	var responses []pgql.NewLinkResponse
	for _, r := range responseStructs {
		var messageType map[string]interface{}
		if err := json.Unmarshal([]byte(r.MessageType), &messageType); err != nil {
			resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Invalid JSON for response message_type: %s", err))
			return
		}
		responses = append(responses, pgql.NewLinkResponse{
			Name:        r.Name,
			Description: r.Description,
			MessageType: messageType,
		})
	}

	var toAgentVersionID pgql.Optional[string]
	if !data.ToAgentVersionId.IsUnknown() {
		toAgentVersionID = pgql.Opt(data.ToAgentVersionId.ValueString())
	}

	var fromAgentVersionID pgql.Optional[string]
	if !data.FromAgentVersionId.IsUnknown() {
		fromAgentVersionID = pgql.Opt(data.FromAgentVersionId.ValueString())
	}

	createResp, err := pgql.CreateLink(ctx, r.client, data.MeshId.ValueString(), &pgql.NewLink{
		Name:               data.Name.ValueString(),
		Description:        data.Description.ValueString(),
		TransfersControl:   data.TransfersControl.ValueBool(),
		FromAgentID:        fromAgentID,
		FromAgentVersionID: fromAgentVersionID,
		ToAgentID:          toAgentID,
		ToAgentVersionID:   toAgentVersionID,
		Responses:          responses,
	})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create mesh link, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(r.mapFields(ctx, &data, data.MeshId.ValueString(), createResp.AddMeshLink)...)
	tflog.Trace(ctx, "created a mesh link resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeshLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	meshResp, err := pgql.GetMeshLinks(ctx, r.client, data.MeshId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read mesh links, got error: %s", err))
		return
	}
	var link *pgql.LinkDetails
	for _, l := range meshResp.Mesh.DraftVersion.Links {
		if l.StableID == data.Id.ValueString() {
			link = &l
			break
		}
	}
	if link == nil {
		resp.Diagnostics.AddError("Not Found", "Mesh link not found")
		return
	}
	resp.Diagnostics.Append(r.mapFields(ctx, &data, data.MeshId.ValueString(), *link)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshLinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MeshLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var update pgql.UpdateLink
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.Opt(planData.Description.ValueString())
	}
	if !planData.TransfersControl.Equal(stateData.TransfersControl) {
		update.TransfersControl = pgql.Opt(planData.TransfersControl.ValueBool())
	}

	if !planData.FromAgentVersionId.Equal(stateData.FromAgentVersionId) && !planData.FromAgentVersionId.IsUnknown() {
		update.NewFromAgentVersionID = pgql.Opt(planData.FromAgentVersionId.ValueString())
	}

	if !planData.ToAgentVersionId.Equal(stateData.ToAgentVersionId) && !planData.ToAgentVersionId.IsUnknown() {
		update.NewToAgentVersionID = pgql.Opt(planData.ToAgentVersionId.ValueString())
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
		var responses []pgql.NewLinkResponse
		for _, r := range responseStructs {
			var messageType map[string]interface{}
			if err := json.Unmarshal([]byte(r.MessageType), &messageType); err != nil {
				resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Invalid JSON for response message_type: %s", err))
				return
			}
			responses = append(responses, pgql.NewLinkResponse{
				Name:        r.Name,
				Description: r.Description,
				MessageType: messageType,
			})
		}
		update.Responses = pgql.Opt(responses)
	}

	updateResp, err := pgql.ModifyLink(ctx, r.client,
		planData.Id.ValueString(),
		&update,
	)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update mesh link, got error: %s", err))
		return
	}
	resp.Diagnostics.Append(r.mapFields(ctx, &planData, planData.MeshId.ValueString(), updateResp.UpdateLinkByStableID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *MeshLinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeshLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := pgql.DeleteLink(ctx, r.client,
		data.Id.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete mesh link, got error: %s", err))
		return
	}
}

func (r *MeshLinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *MeshLinkResource) mapFields(ctx context.Context, data *MeshLinkResourceModel, meshID string, link pgql.LinkDetails) diag.Diagnostics {
	data.Id = types.StringValue(link.StableID)
	data.MeshId = types.StringValue(meshID)
	data.FromAgentId = types.StringValue(link.FromAgentVersion.Agent.Id)
	data.FromAgentVersionId = types.StringValue(link.FromAgentVersion.Id)
	data.ToAgentId = types.StringValue(link.ToAgentVersion.Agent.Id)
	data.ToAgentVersionId = types.StringValue(link.ToAgentVersion.Id)
	data.Name = types.StringValue(link.Name)
	data.Description = types.StringPointerValue(link.Description)
	data.TransfersControl = types.BoolValue(link.TransfersControl)
	data.CreatedAt = types.StringValue(link.CreatedAt)
	data.UpdatedAt = types.StringValue(link.UpdatedAt)
	type responseStruct struct {
		Name        string `tfsdk:"name"`
		Description string `tfsdk:"description"`
		MessageType string `tfsdk:"message_type"`
	}

	var responseStructs []responseStruct
	for _, r := range link.LinkResponses {
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

func (r *MeshLinkResource) resolveAgentID(ctx context.Context, agentID types.String, agentVersionID types.String) (string, error) {
	if !agentID.IsNull() && !agentID.IsUnknown() {
		return agentID.ValueString(), nil
	}
	if !agentVersionID.IsNull() && !agentVersionID.IsUnknown() {
		resp, err := pgql.GetAgentIDForVersion(ctx, r.client, agentVersionID.ValueString())
		if err != nil {
			return "", err
		}
		return resp.AgentVersion.Agent.Id, nil
	}
	return "", fmt.Errorf("either agent_id or agent_version_id must be set")
}
