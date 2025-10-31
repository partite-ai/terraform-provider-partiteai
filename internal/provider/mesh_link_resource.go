package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MeshLinkResource{}
var _ resource.ResourceWithImportState = &MeshLinkResource{}

func NewMeshLinkResource() resource.Resource {
	return &MeshLinkResource{
		BaseResource: NewBaseResource("mesh_link"),
	}
}

// MeshLinkResource implements the mesh link resource.
type MeshLinkResource struct {
	BaseResource
}

// MeshLinkResourceModel describes the resource data model.
type MeshLinkResourceModel struct {
	Id                 types.String      `tfsdk:"id"`
	MeshId             types.String      `tfsdk:"mesh_id"`
	FromAgentId        types.String      `tfsdk:"from_agent_id"`
	FromAgentVersionId types.String      `tfsdk:"from_agent_version_id"`
	ToAgentId          types.String      `tfsdk:"to_agent_id"`
	ToAgentVersionId   types.String      `tfsdk:"to_agent_version_id"`
	Name               types.String      `tfsdk:"name"`
	Description        types.String      `tfsdk:"description"`
	TransfersControl   types.Bool        `tfsdk:"transfers_control"`
	Responses          types.Set         `tfsdk:"responses"`
	CreatedAt          timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt          timetypes.RFC3339 `tfsdk:"updated_at"`
}

type linkResponseStruct struct {
	Name        string               `tfsdk:"name"`
	Description string               `tfsdk:"description"`
	MessageType jsontypes.Normalized `tfsdk:"message_type"`
}

func (r *linkResponseStruct) FromGQL(src pgql.LinkDetailsLinkResponsesLinkResponse) diag.Diagnostics {
	r.Name = src.Name
	r.Description = src.Description
	return setFields(
		context.Background(),
		&r.MessageType, src.MessageType,
	)
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
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the link was created",
			},
			"updated_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
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

func (r *MeshLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MeshLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "mesh_link", "")

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
		responses = append(responses, pgql.NewLinkResponse{
			Name:        r.Name,
			Description: r.Description,
			MessageType: r.MessageType,
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

	createResp, err := pgql.CreateLink(ctx, r.GetClient(), data.MeshId.ValueString(), &pgql.NewLink{
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
		handleAPIError(&resp.Diagnostics, err, "create mesh link")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, data.MeshId.ValueString(), createResp.AddMeshLink)...)

	logResourceOperation(ctx, "created", "mesh_link", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeshLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "mesh_link", data.Id.ValueString())

	meshResp, err := pgql.GetMeshLinks(ctx, r.GetClient(), data.MeshId.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mesh_link", data.Id.ValueString())
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
		handleAPIErrorWithDetails(&resp.Diagnostics, fmt.Errorf("mesh link not found"), "read", "mesh_link", data.Id.ValueString())
		return
	}
	resp.Diagnostics.Append(r.mapResponse(ctx, &data, data.MeshId.ValueString(), *link)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeshLinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MeshLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "mesh_link", planData.Id.ValueString())

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
		update.FromAgentVersionID = pgql.Opt(planData.FromAgentVersionId.ValueString())
	}

	if !planData.ToAgentVersionId.Equal(stateData.ToAgentVersionId) && !planData.ToAgentVersionId.IsUnknown() {
		update.ToAgentVersionID = pgql.Opt(planData.ToAgentVersionId.ValueString())
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
			responses = append(responses, pgql.NewLinkResponse{
				Name:        r.Name,
				Description: r.Description,
				MessageType: r.MessageType,
			})
		}
		update.Responses = pgql.Opt(responses)
	}

	updateResp, err := pgql.ModifyLink(ctx, r.GetClient(),
		planData.Id.ValueString(),
		&update,
	)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "mesh_link", planData.Id.ValueString())
		return
	}
	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, planData.MeshId.ValueString(), updateResp.UpdateLinkByStableID)...)

	logResourceOperation(ctx, "updated", "mesh_link", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *MeshLinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeshLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "mesh_link", data.Id.ValueString())

	_, err := pgql.DeleteLink(ctx, r.GetClient(),
		data.Id.ValueString(),
	)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "mesh_link", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "mesh_link", data.Id.ValueString())
}

func (r *MeshLinkResource) mapResponse(ctx context.Context, data *MeshLinkResourceModel, meshID string, link pgql.LinkDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	// Use setFields for standard fields
	diags.Append(setFields(
		ctx,
		&data.Id, link.StableID,
		&data.MeshId, meshID,
		&data.FromAgentId, link.FromAgentVersion.Agent.Id,
		&data.FromAgentVersionId, link.FromAgentVersion.Id,
		&data.ToAgentId, link.ToAgentVersion.Agent.Id,
		&data.ToAgentVersionId, link.ToAgentVersion.Id,
		&data.Name, link.Name,
		&data.Description, link.Description,
		&data.TransfersControl, link.TransfersControl,
		&data.CreatedAt, link.CreatedAt,
		&data.UpdatedAt, link.UpdatedAt,
		&data.Responses, newStructSliceMappable(link.LinkResponses, linkResponseStruct{}),
	)...)

	return diags
}

func (r *MeshLinkResource) resolveAgentID(ctx context.Context, agentID types.String, agentVersionID types.String) (string, error) {
	if !agentID.IsNull() && !agentID.IsUnknown() {
		return agentID.ValueString(), nil
	}
	if !agentVersionID.IsNull() && !agentVersionID.IsUnknown() {
		resp, err := pgql.GetAgentIDForVersion(ctx, r.GetClient(), agentVersionID.ValueString())
		if err != nil {
			return "", err
		}
		return resp.AgentVersion.Agent.Id, nil
	}
	return "", fmt.Errorf("either agent_id or agent_version_id must be set")
}
