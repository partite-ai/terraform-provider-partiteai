package provider

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
	return &ApiKeyResource{}
}

type ApiKeyResource struct {
	client graphql.Client
}

type ApiKeyResourceModel struct {
	Id          types.String      `tfsdk:"id"`
	Name        types.String      `tfsdk:"name"`
	Description types.String      `tfsdk:"description"`
	Scopes      types.List        `tfsdk:"scopes"`
	Kid         types.String      `tfsdk:"kid"`
	Secret      types.String      `tfsdk:"secret"`
	CreatedAt   timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt   timetypes.RFC3339 `tfsdk:"updated_at"`
}

func (r *ApiKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *ApiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "API Key resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "API key identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "API key name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "API key description",
				Optional:            true,
			},
			"scopes": schema.ListAttribute{
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
			},
			"created_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the API key was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the API key was last updated",
			},
		},
	}
}

func (r *ApiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopes []string
	if diag := data.Scopes.ElementsAs(ctx, &scopes, false); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	createResp, err := pgql.CreateApiKey(ctx, r.client, data.Name.ValueString(), data.Description.ValueStringPointer(), scopes)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create API key, got error: %s", err))
		return
	}

	apiKey := createResp.CreateApiKey.ApiKey
	data.Id = types.StringValue(apiKey.Id)
	data.Name = types.StringValue(apiKey.Name)
	data.Description = types.StringPointerValue(apiKey.Description)
	data.Kid = types.StringValue(apiKey.Kid)
	data.Scopes, _ = types.ListValueFrom(ctx, types.StringType, apiKey.Scopes)
	data.CreatedAt, _ = timetypes.NewRFC3339Value(apiKey.CreatedAt)
	data.UpdatedAt, _ = timetypes.NewRFC3339Value(apiKey.UpdatedAt)
	data.Secret = types.StringValue(createResp.CreateApiKey.Secret)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := pgql.GetApiKey(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read API key, got error: %s", err))
		return
	}

	apiKey := getResp.ApiKey
	data.Id = types.StringValue(apiKey.Id)
	data.Name = types.StringValue(apiKey.Name)
	data.Description = types.StringPointerValue(apiKey.Description)
	data.Kid = types.StringValue(apiKey.Kid)
	data.Scopes, _ = types.ListValueFrom(ctx, types.StringType, apiKey.Scopes)
	data.CreatedAt, _ = timetypes.NewRFC3339Value(apiKey.CreatedAt)
	data.UpdatedAt, _ = timetypes.NewRFC3339Value(apiKey.UpdatedAt)
	// Do not update secret on read (only available on create)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData ApiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopes []string
	if diag := planData.Scopes.ElementsAs(ctx, &scopes, false); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	updateResp, err := pgql.ModifyApiKey(ctx, r.client, planData.Id.ValueString(), planData.Name.ValueStringPointer(), planData.Description.ValueStringPointer(), scopes)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update API key, got error: %s", err))
		return
	}

	apiKey := updateResp.UpdateApiKey
	planData.Id = types.StringValue(apiKey.Id)
	planData.Name = types.StringValue(apiKey.Name)
	planData.Description = types.StringPointerValue(apiKey.Description)
	planData.Kid = types.StringValue(apiKey.Kid)
	planData.Scopes, _ = types.ListValueFrom(ctx, types.StringType, apiKey.Scopes)
	planData.CreatedAt, _ = timetypes.NewRFC3339Value(apiKey.CreatedAt)
	planData.UpdatedAt, _ = timetypes.NewRFC3339Value(apiKey.UpdatedAt)
	// Do not update secret on update

	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := pgql.DeleteApiKey(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete API key, got error: %s", err))
		return
	}
}

func (r *ApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
