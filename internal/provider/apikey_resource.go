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
var _ resource.Resource = &ApiKeyResource{}
var _ resource.ResourceWithImportState = &ApiKeyResource{}

func NewApiKeyResource() resource.Resource {
	return &ApiKeyResource{
		BaseResource: NewBaseResource("api_key"),
	}
}

type ApiKeyResource struct {
	BaseResource
}

// ApiKeyResourceModel extends the base model with resource-specific fields.
type ApiKeyResourceModel struct {
	ApiKeyBaseModel
	Secret types.String `tfsdk:"secret"` // Only available in resource after creation
	Token  types.String `tfsdk:"token"`  // Only available in resource after creation
}

func (r *ApiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "API Key resource",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,

			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
			"scopes": schema.SetAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "List of scopes for the API key",
			},
			"kid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Key ID",
			},
			"secret": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "API key secret",
				DeprecationMessage:  "Use token instead. Sending the key ID and secret as basic auth is deprecated.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The token to send as `Authorization: Bearer <token>`. Use it as the provider's `api_token`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "api_key", "")

	var scopes []string
	if diag := data.Scopes.ElementsAs(ctx, &scopes, false); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	createResp, err := pgql.CreateApiKey(ctx, r.GetClient(), data.Name.ValueString(), data.Description.ValueStringPointer(), scopes)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create API key")
		return
	}

	apiKey := createResp.CreateApiKey.ApiKey
	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &apiKey)...)
	resp.Diagnostics.Append(setFields(
		ctx,
		&data.Secret, createResp.CreateApiKey.Secret,
		&data.Token, createResp.CreateApiKey.Token,
	)...)

	logResourceOperation(ctx, "created", "api_key", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "api_key", data.Id.ValueString())

	getResp, err := pgql.GetApiKey(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "api_key", data.Id.ValueString())
		return
	}

	apiKey := getResp.ApiKey

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &apiKey)...)
	data.fillTokenFromSecret()
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData ApiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "api_key", planData.Id.ValueString())

	var scopes []string
	if diag := planData.Scopes.ElementsAs(ctx, &scopes, false); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	updateResp, err := pgql.ModifyApiKey(ctx, r.GetClient(), planData.Id.ValueString(), planData.Name.ValueStringPointer(), planData.Description.ValueStringPointer(), scopes)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "api_key", planData.Id.ValueString())
		return
	}

	apiKey := updateResp.UpdateApiKey
	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, &apiKey)...)
	planData.Secret = stateData.Secret
	planData.Token = stateData.Token
	planData.fillTokenFromSecret()

	logResourceOperation(ctx, "updated", "api_key", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "api_key", data.Id.ValueString())

	_, err := pgql.DeleteApiKey(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "api_key", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "api_key", data.Id.ValueString())
}

func (r *ApiKeyResource) mapResponse(ctx context.Context, data *ApiKeyResourceModel, apiKey *pgql.ApiKeyDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, apiKey)
}

// fillTokenFromSecret gives state written before the token attribute existed a
// token built from the key ID and secret it already holds.
func (m *ApiKeyResourceModel) fillTokenFromSecret() {
	if !m.Token.IsNull() && !m.Token.IsUnknown() {
		return
	}
	if m.Secret.IsNull() || m.Secret.IsUnknown() || m.Secret.ValueString() == "" {
		m.Token = types.StringNull()
		return
	}
	m.Token = types.StringValue(apiKeyToken(m.Kid.ValueString(), m.Secret.ValueString()))
}
