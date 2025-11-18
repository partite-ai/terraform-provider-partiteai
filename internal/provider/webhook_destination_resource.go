package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &WebhookDestinationResource{}
var _ resource.ResourceWithImportState = &WebhookDestinationResource{}

func NewWebhookDestinationResource() resource.Resource {
	return &WebhookDestinationResource{
		BaseResource: NewBaseResource("webhook_destination"),
	}
}

type WebhookDestinationResource struct {
	BaseResource
}

// WebhookDestinationResourceModel extends the base model with resource-specific fields.
type WebhookDestinationResourceModel struct {
	WebhookDestinationBaseModel
}

func (r *WebhookDestinationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Webhook Destination resource",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"external_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "External ID for the webhook destination",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"invoke_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Invoke URL for the webhook destination",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the mesh associated with this webhook destination",
			},
			"mesh_version_label": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional mesh version label",
			},
			"input_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the input to invoke",
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *WebhookDestinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WebhookDestinationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "webhook_destination", "")

	input := pgql.NewWebhookDestination{
		Name:             data.Name.ValueString(),
		Description:      pgql.OptPtr(data.Description.ValueStringPointer()),
		MeshID:           data.MeshId.ValueString(),
		MeshVersionLabel: pgql.OptPtr(data.MeshVersionLabel.ValueStringPointer()),
		InputName:        data.InputName.ValueString(),
	}

	createResp, err := pgql.CreateWebhookDestination(ctx, r.GetClient(), &input)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create webhook destination")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &createResp.CreateWebhookDestination)...)
	data.InvokeUrl = types.StringValue(r.baseURL.JoinPath("webhooks", data.ExternalId.ValueString()).String())
	logResourceOperation(ctx, "created", "webhook_destination", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookDestinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WebhookDestinationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "webhook_destination", data.Id.ValueString())

	getResp, err := pgql.GetWebhookDestination(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "webhook_destination", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &getResp.WebhookDestination)...)
	data.InvokeUrl = types.StringValue(r.baseURL.JoinPath("webhooks", data.ExternalId.ValueString()).String())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookDestinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData WebhookDestinationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "webhook_destination", planData.Id.ValueString())

	input := pgql.UpdateWebhookDestination{
		Name:             pgql.OptNil(planData.Name.ValueStringPointer()),
		Description:      pgql.OptNil(planData.Description.ValueStringPointer()),
		MeshID:           pgql.OptNil(planData.MeshId.ValueStringPointer()),
		MeshVersionLabel: pgql.OptNil(planData.MeshVersionLabel.ValueStringPointer()),
		InputName:        pgql.OptNil(planData.InputName.ValueStringPointer()),
	}

	updateResp, err := pgql.ModifyWebhookDestination(ctx, r.GetClient(), planData.Id.ValueString(), &input)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "webhook_destination", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, &updateResp.UpdateWebhookDestination)...)

	logResourceOperation(ctx, "updated", "webhook_destination", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *WebhookDestinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WebhookDestinationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "webhook_destination", data.Id.ValueString())

	_, err := pgql.DeleteWebhookDestination(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "webhook_destination", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "webhook_destination", data.Id.ValueString())
}

func (r *WebhookDestinationResource) mapResponse(ctx context.Context, data *WebhookDestinationResourceModel, webhookDestination *pgql.WebhookDestinationDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, webhookDestination)
}
