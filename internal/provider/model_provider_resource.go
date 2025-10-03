package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
}

func (r *ModelProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Provider resource",
		Attributes: standardSchemaAttributes(map[string]schema.Attribute{
			"anthropic": schema.SingleNestedAttribute{
				MarkdownDescription: "Anthropic provider config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"api_key": schema.StringAttribute{
						MarkdownDescription: "API key for Anthropic",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
			"openai": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI provider config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"api_key": schema.StringAttribute{
						MarkdownDescription: "API key for OpenAI",
						Required:            true,
						Sensitive:           true,
					},
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
							"api_key": schema.StringAttribute{
								MarkdownDescription: "API key for Gemini",
								Required:            true,
								Sensitive:           true,
							},
						},
					},
					"vertex": schema.SingleNestedAttribute{
						MarkdownDescription: "Vertex API config",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"service_account_key": schema.StringAttribute{
								MarkdownDescription: "Service account key for Vertex",
								Required:            true,
								Sensitive:           true,
							},
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
					"api_token": schema.StringAttribute{
						MarkdownDescription: "API token for OpenAI-compatible provider",
						Optional:            true,
						Sensitive:           true,
					},
				},
			},
		}),
	}
}

func (r *ModelProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
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

	// Determine which provider block is set
	switch {
	case !data.Anthropic.IsNull() && !data.Anthropic.IsUnknown():
		createReq.ProviderType = pgql.ModelProviderTypeAnthropic
		var block struct {
			ApiKey string `tfsdk:"api_key"`
		}
		_ = tfsdk.ValueAs(ctx, data.Anthropic, &block)
		createReq.ProviderConfig = pgql.ModelProviderConfigInput{
			Anthropic: &pgql.ModelProviderConfigInputAnthropic{
				ApiKey: block.ApiKey,
			},
		}
	case !data.OpenAI.IsNull() && !data.OpenAI.IsUnknown():
		createReq.ProviderType = pgql.ModelProviderTypeOpenAi
		var block struct {
			ApiKey string `tfsdk:"api_key"`
		}
		_ = tfsdk.ValueAs(ctx, data.OpenAI, &block)
		createReq.ProviderConfig = pgql.ModelProviderConfigInput{
			OpenAI: &pgql.ModelProviderConfigInputOpenAI{
				ApiKey: block.ApiKey,
			},
		}
	case !data.GoogleGenAI.IsNull() && !data.GoogleGenAI.IsUnknown():
		createReq.ProviderType = pgql.ModelProviderTypeGoogleGenAi
		var block struct {
			Gemini types.Object `tfsdk:"gemini"`
			Vertex types.Object `tfsdk:"vertex"`
		}
		_ = tfsdk.ValueAs(ctx, data.GoogleGenAI, &block)
		geminiSet := !block.Gemini.IsNull() && !block.Gemini.IsUnknown()
		vertexSet := !block.Vertex.IsNull() && !block.Vertex.IsUnknown()
		if geminiSet && vertexSet {
			resp.Diagnostics.AddError("Invalid google_genai config", "Only one of gemini or vertex should be set, not both.")
			return
		}
		if !geminiSet && !vertexSet {
			resp.Diagnostics.AddError("Invalid google_genai config", "Either gemini or vertex config must be provided.")
			return
		}
		cfg := &pgql.ModelProviderConfigInputGoogleGenAI{}
		if geminiSet {
			var gemini struct {
				ApiKey string `tfsdk:"api_key"`
			}
			_ = tfsdk.ValueAs(ctx, block.Gemini, &gemini)
			cfg.Gemini = &pgql.ModelProviderConfigInputGoogleGenAIGeminiApi{
				ApiKey: gemini.ApiKey,
			}
		}
		if vertexSet {
			var vertex struct {
				ServiceAccountKey string `tfsdk:"service_account_key"`
				ProjectID         string `tfsdk:"project_id"`
				Location          string `tfsdk:"location"`
			}
			_ = tfsdk.ValueAs(ctx, block.Vertex, &vertex)
			cfg.Vertex = &pgql.ModelProviderConfigInputGoogleGenAIVertex{
				ServiceAccountKey: vertex.ServiceAccountKey,
				ProjectID:         vertex.ProjectID,
				Location:          vertex.Location,
			}
		}
		createReq.ProviderConfig = pgql.ModelProviderConfigInput{
			GoogleGenAI: cfg,
		}
	case !data.OpenAICompat.IsNull() && !data.OpenAICompat.IsUnknown():
		createReq.ProviderType = pgql.ModelProviderTypeOpenAiCompatible
		var block struct {
			Url      string       `tfsdk:"url"`
			ApiToken types.String `tfsdk:"api_token"`
		}
		_ = tfsdk.ValueAs(ctx, data.OpenAICompat, &block)
		createReq.ProviderConfig = pgql.ModelProviderConfigInput{
			OpenAICompatible: &pgql.ModelProviderConfigInputOpenAICompatible{
				Url:      block.Url,
				ApiToken: pgql.OptPtr(block.ApiToken.ValueStringPointer()),
			},
		}
	default:
		resp.Diagnostics.AddError("Missing provider config", "One provider config block must be set")
		return
	}

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
	var planData, stateData ModelProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
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

	// Determine which provider block is set
	switch {
	case !planData.Anthropic.IsNull() && !planData.Anthropic.IsUnknown():
		update.ProviderConfig = pgql.ModelProviderConfigInput{}
		var block struct {
			ApiKey string `tfsdk:"api_key"`
		}
		_ = tfsdk.ValueAs(ctx, planData.Anthropic, &block)
		update.ProviderConfig.Anthropic = &pgql.ModelProviderConfigInputAnthropic{
			ApiKey: block.ApiKey,
		}
	case !planData.OpenAI.IsNull() && !planData.OpenAI.IsUnknown():
		update.ProviderConfig = pgql.ModelProviderConfigInput{}
		var block struct {
			ApiKey string `tfsdk:"api_key"`
		}
		_ = tfsdk.ValueAs(ctx, planData.OpenAI, &block)
		update.ProviderConfig.OpenAI = &pgql.ModelProviderConfigInputOpenAI{
			ApiKey: block.ApiKey,
		}
	case !planData.GoogleGenAI.IsNull() && !planData.GoogleGenAI.IsUnknown():
		update.ProviderConfig = pgql.ModelProviderConfigInput{}
		var block struct {
			Gemini types.Object `tfsdk:"gemini"`
			Vertex types.Object `tfsdk:"vertex"`
		}
		_ = tfsdk.ValueAs(ctx, planData.GoogleGenAI, &block)
		geminiSet := !block.Gemini.IsNull() && !block.Gemini.IsUnknown()
		vertexSet := !block.Vertex.IsNull() && !block.Vertex.IsUnknown()
		if geminiSet && vertexSet {
			resp.Diagnostics.AddError("Invalid google_genai config", "Only one of gemini or vertex should be set, not both.")
			return
		}
		if !geminiSet && !vertexSet {
			resp.Diagnostics.AddError("Invalid google_genai config", "Either gemini or vertex config must be provided.")
			return
		}
		cfg := &pgql.ModelProviderConfigInputGoogleGenAI{}
		if geminiSet {
			var gemini struct {
				ApiKey string `tfsdk:"api_key"`
			}
			_ = tfsdk.ValueAs(ctx, block.Gemini, &gemini)
			cfg.Gemini = &pgql.ModelProviderConfigInputGoogleGenAIGeminiApi{
				ApiKey: gemini.ApiKey,
			}
		}
		if vertexSet {
			var vertex struct {
				ServiceAccountKey string `tfsdk:"service_account_key"`
				ProjectID         string `tfsdk:"project_id"`
				Location          string `tfsdk:"location"`
			}
			_ = tfsdk.ValueAs(ctx, block.Vertex, &vertex)
			cfg.Vertex = &pgql.ModelProviderConfigInputGoogleGenAIVertex{
				ServiceAccountKey: vertex.ServiceAccountKey,
				ProjectID:         vertex.ProjectID,
				Location:          vertex.Location,
			}
		}
		update.ProviderConfig.GoogleGenAI = cfg
	case !planData.OpenAICompat.IsNull() && !planData.OpenAICompat.IsUnknown():
		update.ProviderConfig = pgql.ModelProviderConfigInput{}
		var block struct {
			Url      string       `tfsdk:"url"`
			ApiToken types.String `tfsdk:"api_token"`
		}
		_ = tfsdk.ValueAs(ctx, planData.OpenAICompat, &block)
		update.ProviderConfig.OpenAICompatible = &pgql.ModelProviderConfigInputOpenAICompatible{
			Url:      block.Url,
			ApiToken: pgql.OptPtr(block.ApiToken.ValueStringPointer()),
		}
	default:
		resp.Diagnostics.AddError("Missing provider config", "One provider config block must be set")
		return
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

func (r *ModelProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ModelProviderResource) mapResponse(ctx context.Context, data *ModelProviderResourceModel, details pgql.ModelProviderDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, details)
}
