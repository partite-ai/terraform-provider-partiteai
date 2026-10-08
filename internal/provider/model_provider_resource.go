package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ModelProviderResource{}
var _ resource.ResourceWithImportState = &ModelProviderResource{}

func NewModelProviderResource() resource.Resource {
	return &ModelProviderResource{
		BaseResource: NewBaseResource("model_provider"),
	}
}

type ModelProviderResource struct {
	BaseResource
}

type ModelProviderResourceModel struct {
	ModelProviderBaseModel
	CredentialsVersion types.Int64 `tfsdk:"credentials_version"`
	Probe              types.Bool  `tfsdk:"probe"`
}

// secretAttribute is a provider secret. It is write-only: Terraform never
// stores it, and the server never returns it, so a changed value is sent only
// when credentials_version changes with it.
func secretAttribute(description string, required bool) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: description + " Write-only: never stored in state. Change `credentials_version` to send a new value.",
		Required:            required,
		Optional:            !required,
		Sensitive:           true,
		WriteOnly:           true,
	}
}

func (r *ModelProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Provider resource",
		Attributes: standardSchemaAttributes(map[string]schema.Attribute{
			"credentials_version": schema.Int64Attribute{
				MarkdownDescription: "The version of the provider's secret. The secret is write-only, so Terraform cannot " +
					"see it change; increment this to send a new one.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(1),
			},
			"probe": providerProbeAttribute,
			"anthropic": schema.SingleNestedAttribute{
				MarkdownDescription: "Anthropic provider config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"api_key_wo": secretAttribute("API key for Anthropic.", true),
				},
			},
			"openai": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI provider config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"api_key_wo": secretAttribute("API key for OpenAI.", true),
				},
			},
			"google_genai": schema.SingleNestedAttribute{
				MarkdownDescription: "Google GenAI provider config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"gemini": schema.SingleNestedAttribute{
						MarkdownDescription: "Gemini API config",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"api_key_wo": secretAttribute("API key for Gemini.", true),
						},
					},
					"vertex": schema.SingleNestedAttribute{
						MarkdownDescription: "Vertex API config",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"service_account_key_wo": secretAttribute("Service account key for Vertex.", true),
							"project_id": schema.StringAttribute{
								MarkdownDescription: "Project ID for Vertex",
								Required:            true,
							},
							"location": schema.StringAttribute{
								MarkdownDescription: "Location for Vertex",
								Required:            true,
							},
						},
					},
				},
			},
			"openai_compatible": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI-compatible provider config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "URL for OpenAI-compatible provider",
						Required:            true,
					},
					"api_token_wo": secretAttribute("API token for the OpenAI-compatible provider. Leave it out for one that takes no token.", false),
				},
			},
			"fireworks": schema.SingleNestedAttribute{
				MarkdownDescription: "Fireworks provider config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"api_key_wo": secretAttribute("API key for Fireworks.", true),
					"account_id": schema.StringAttribute{
						MarkdownDescription: "Fireworks account id. When set, the model list includes this account's models.",
						Optional:            true,
					},
				},
			},
			"cerebras": schema.SingleNestedAttribute{
				MarkdownDescription: "Cerebras provider config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"api_key_wo": secretAttribute("API key for Cerebras.", true),
				},
			},
		}),
	}
}

// providerConfigInput builds the provider type and config to send from the
// config, the only place the write-only secrets are.
func providerConfigInput(ctx context.Context, data *ModelProviderResourceModel, diags *diag.Diagnostics) (pgql.ModelProviderType, *pgql.ModelProviderConfigInput) {
	set := func(o types.Object) bool { return !o.IsNull() && !o.IsUnknown() }

	switch {
	case set(data.Anthropic):
		var block struct {
			ApiKey string `tfsdk:"api_key_wo"`
		}
		diags.Append(tfsdk.ValueAs(ctx, data.Anthropic, &block)...)
		return pgql.ModelProviderTypeAnthropic, &pgql.ModelProviderConfigInput{
			Anthropic: &pgql.ModelProviderConfigInputAnthropic{ApiKey: block.ApiKey},
		}
	case set(data.OpenAI):
		var block struct {
			ApiKey string `tfsdk:"api_key_wo"`
		}
		diags.Append(tfsdk.ValueAs(ctx, data.OpenAI, &block)...)
		return pgql.ModelProviderTypeOpenAi, &pgql.ModelProviderConfigInput{
			OpenAI: &pgql.ModelProviderConfigInputOpenAI{ApiKey: block.ApiKey},
		}
	case set(data.GoogleGenAI):
		var block struct {
			Gemini types.Object `tfsdk:"gemini"`
			Vertex types.Object `tfsdk:"vertex"`
		}
		diags.Append(tfsdk.ValueAs(ctx, data.GoogleGenAI, &block)...)
		geminiSet, vertexSet := set(block.Gemini), set(block.Vertex)
		if geminiSet && vertexSet {
			diags.AddError("Invalid google_genai config", "Only one of gemini or vertex should be set, not both.")
			return "", nil
		}
		if !geminiSet && !vertexSet {
			diags.AddError("Invalid google_genai config", "Either gemini or vertex config must be provided.")
			return "", nil
		}
		cfg := &pgql.ModelProviderConfigInputGoogleGenAI{}
		if geminiSet {
			var gemini struct {
				ApiKey string `tfsdk:"api_key_wo"`
			}
			diags.Append(tfsdk.ValueAs(ctx, block.Gemini, &gemini)...)
			cfg.Gemini = &pgql.ModelProviderConfigInputGoogleGenAIGeminiApi{ApiKey: gemini.ApiKey}
		}
		if vertexSet {
			var vertex struct {
				ServiceAccountKey string `tfsdk:"service_account_key_wo"`
				ProjectID         string `tfsdk:"project_id"`
				Location          string `tfsdk:"location"`
			}
			diags.Append(tfsdk.ValueAs(ctx, block.Vertex, &vertex)...)
			cfg.Vertex = &pgql.ModelProviderConfigInputGoogleGenAIVertex{
				ServiceAccountKey: vertex.ServiceAccountKey,
				ProjectID:         vertex.ProjectID,
				Location:          vertex.Location,
			}
		}
		return pgql.ModelProviderTypeGoogleGenAi, &pgql.ModelProviderConfigInput{GoogleGenAI: cfg}
	case set(data.OpenAICompat):
		var block struct {
			Url      string       `tfsdk:"url"`
			ApiToken types.String `tfsdk:"api_token_wo"`
		}
		diags.Append(tfsdk.ValueAs(ctx, data.OpenAICompat, &block)...)
		return pgql.ModelProviderTypeOpenAiCompatible, &pgql.ModelProviderConfigInput{
			OpenAICompatible: &pgql.ModelProviderConfigInputOpenAICompatible{
				Url:      block.Url,
				ApiToken: pgql.OptPtr(block.ApiToken.ValueStringPointer()),
			},
		}
	case set(data.Fireworks):
		var block struct {
			ApiKey    string       `tfsdk:"api_key_wo"`
			AccountID types.String `tfsdk:"account_id"`
		}
		diags.Append(tfsdk.ValueAs(ctx, data.Fireworks, &block)...)
		return pgql.ModelProviderTypeFireworks, &pgql.ModelProviderConfigInput{
			Fireworks: &pgql.ModelProviderConfigInputFireworks{
				ApiKey:    block.ApiKey,
				AccountID: pgql.OptPtr(block.AccountID.ValueStringPointer()),
			},
		}
	case set(data.Cerebras):
		var block struct {
			ApiKey string `tfsdk:"api_key_wo"`
		}
		diags.Append(tfsdk.ValueAs(ctx, data.Cerebras, &block)...)
		return pgql.ModelProviderTypeCerebras, &pgql.ModelProviderConfigInput{
			Cerebras: &pgql.ModelProviderConfigInputCerebras{ApiKey: block.ApiKey},
		}
	default:
		diags.AddError("Missing provider config", "One provider config block must be set")
		return "", nil
	}
}

// providerConfigChanged reports whether an update has to send the provider
// config: a new secret, or a change to what the provider blocks hold. The
// secrets are null in both plan and state, so the blocks compare equal unless
// a field Terraform can see changed.
func providerConfigChanged(plan, state *ModelProviderResourceModel) bool {
	return !plan.CredentialsVersion.Equal(state.CredentialsVersion) ||
		!plan.Anthropic.Equal(state.Anthropic) ||
		!plan.OpenAI.Equal(state.OpenAI) ||
		!plan.GoogleGenAI.Equal(state.GoogleGenAI) ||
		!plan.OpenAICompat.Equal(state.OpenAICompat) ||
		!plan.Fireworks.Equal(state.Fireworks) ||
		!plan.Cerebras.Equal(state.Cerebras)
}

func (r *ModelProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data, config ModelProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "model_provider", "")

	var createReq pgql.NewModelProvider
	createReq.Name = data.Name.ValueString()
	createReq.Description = pgql.Optional[string]{}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		createReq.Description = pgql.Opt(data.Description.ValueString())
	}
	createReq.Tags = nil
	if !data.Tags.IsNull() && !data.Tags.IsUnknown() {
		var tags []struct {
			Name  string `tfsdk:"name"`
			Value string `tfsdk:"value"`
		}
		_ = tfsdk.ValueAs(ctx, data.Tags, &tags)
		for _, tag := range tags {
			createReq.Tags = append(createReq.Tags, &pgql.Tag{Name: tag.Name, Value: tag.Value})
		}
	}

	providerType, providerConfig := providerConfigInput(ctx, &config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	createReq.ProviderType = providerType
	createReq.ProviderConfig = *providerConfig
	createReq.Probe = config.Probe.ValueBoolPointer()

	createResp, err := pgql.CreateModelProvider(ctx, r.GetClient(), &createReq)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create model provider")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, createResp.CreateModelProvider)...)

	logResourceOperation(ctx, "created", "model_provider", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "model_provider", data.Id.ValueString())

	modelProviderResp, err := pgql.GetModelProvider(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "model_provider", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, modelProviderResp.ModelProvider)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData, config ModelProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "model_provider", planData.Id.ValueString())

	var update pgql.UpdateModelProvider
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}

	// Left out, the server keeps the config and secret it has.
	if providerConfigChanged(&planData, &stateData) {
		_, providerConfig := providerConfigInput(ctx, &config, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		update.ProviderConfig = providerConfig
		update.Probe = config.Probe.ValueBoolPointer()
	}

	if !planData.Tags.Equal(stateData.Tags) {
		var tags []struct {
			Name  string `tfsdk:"name"`
			Value string `tfsdk:"value"`
		}
		_ = tfsdk.ValueAs(ctx, planData.Tags, &tags)
		var gqlTags []*pgql.Tag
		for _, tag := range tags {
			gqlTags = append(gqlTags, &pgql.Tag{Name: tag.Name, Value: tag.Value})
		}
		update.Tags = pgql.Opt(gqlTags)
	}

	updateResp, err := pgql.ModifyModelProvider(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "model_provider", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, updateResp.UpdateModelProvider)...)

	logResourceOperation(ctx, "updated", "model_provider", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ModelProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "model_provider", data.Id.ValueString())

	_, err := pgql.DeleteModelProvider(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "model_provider", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "model_provider", data.Id.ValueString())
}

// ImportState starts an imported provider at credentials_version 1, the
// default, so the first plan after an import does not send the secret again.
func (r *ModelProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("credentials_version"), 1)...)
}

func (r *ModelProviderResource) mapResponse(ctx context.Context, data *ModelProviderResourceModel, details pgql.ModelProviderDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, details, writeOnlySecret)
}
