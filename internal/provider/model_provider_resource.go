package provider

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ModelProviderResource{}
var _ resource.ResourceWithImportState = &ModelProviderResource{}

func NewModelProviderResource() resource.Resource {
	return &ModelProviderResource{}
}

// ModelProviderResource defines the resource implementation.
type ModelProviderResource struct {
	client graphql.Client
}

// ModelProviderResourceModel describes the resource data model.
type ModelProviderResourceModel struct {
	Id           types.String      `tfsdk:"id"`
	Name         types.String      `tfsdk:"name"`
	Description  types.String      `tfsdk:"description"`
	Tags         types.List        `tfsdk:"tags"`
	CreatedAt    timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt    timetypes.RFC3339 `tfsdk:"updated_at"`
	Anthropic    types.Object      `tfsdk:"anthropic"`
	OpenAI       types.Object      `tfsdk:"openai"`
	GoogleGenAI  types.Object      `tfsdk:"google_genai"`
	OpenAICompat types.Object      `tfsdk:"openai_compatible"`
}

var tagType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Name of the tag",
			Required:            true,
		},
		"value": schema.StringAttribute{
			MarkdownDescription: "Value of the tag",
			Required:            true,
		},
	},
}

func (r *ModelProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_provider"
}

func (r *ModelProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Provider resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Model Provider identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Model Provider name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the Model Provider",
				Optional:            true,
			},
			"tags": schema.ListNestedAttribute{
				MarkdownDescription: "Tags for the Model Provider",
				NestedObject:        tagType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(tagType.Type(), nil)),
			},
			"created_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the Model Provider was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the Model Provider was last updated",
			},
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
		},
	}
}

func (r *ModelProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ModelProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
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

	createResp, err := pgql.CreateModelProvider(ctx, r.client, &createReq)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create model provider, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &data, createResp.CreateModelProvider); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	tflog.Trace(ctx, "created a model provider resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelProviderResp, err := pgql.GetModelProvider(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read model provider, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &data, modelProviderResp.ModelProvider); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData ModelProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}
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
	updateResp, err := pgql.ModifyModelProvider(ctx, r.client, planData.Id.ValueString(), &update)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update model provider, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &planData, updateResp.UpdateModelProvider); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ModelProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := pgql.DeleteModelProvider(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete model provider, got error: %s", err))
		return
	}
}

func (r *ModelProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ModelProviderResource) mapFields(ctx context.Context, data *ModelProviderResourceModel, details pgql.ModelProviderDetails) diag.Diagnostics {
	var diag diag.Diagnostics
	data.Id = types.StringValue(details.Id)
	data.Name = types.StringValue(details.Name)
	if details.Description != nil {
		data.Description = types.StringValue(*details.Description)
	} else {
		data.Description = types.StringNull()
	}
	var tags []attr.Value
	for _, tag := range details.Tags {
		tags = append(tags, types.ObjectValueMust(map[string]attr.Type{
			"name":  types.StringType,
			"value": types.StringType,
		}, map[string]attr.Value{
			"name":  types.StringValue(tag.Name),
			"value": types.StringValue(tag.Value),
		}))
	}
	data.Tags = types.ListValueMust(tagType.Type(), tags)
	data.CreatedAt, diag = timetypes.NewRFC3339Value(details.CreatedAt)
	if diag.HasError() {
		return diag
	}
	data.UpdatedAt, diag = timetypes.NewRFC3339Value(details.UpdatedAt)
	if diag.HasError() {
		return diag
	}
	// Clear all blocks
	data.Anthropic = types.ObjectNull(data.Anthropic.AttributeTypes(ctx))
	data.OpenAI = types.ObjectNull(data.OpenAI.AttributeTypes(ctx))
	data.GoogleGenAI = types.ObjectNull(data.GoogleGenAI.AttributeTypes(ctx))
	data.OpenAICompat = types.ObjectNull(data.OpenAICompat.AttributeTypes(ctx))
	// Set the correct block based on provider type
	switch details.ProviderType {
	case pgql.ModelProviderTypeAnthropic:
		c := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigAnthropic)
		var block = map[string]attr.Value{
			"api_key": types.StringValue(c.ApiKey),
		}
		data.Anthropic = types.ObjectValueMust(data.Anthropic.AttributeTypes(ctx), block)
	case pgql.ModelProviderTypeOpenAi:
		c := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAI)
		var block = map[string]attr.Value{
			"api_key": types.StringValue(c.ApiKey),
		}
		data.OpenAI = types.ObjectValueMust(data.OpenAI.AttributeTypes(ctx), block)
	case pgql.ModelProviderTypeGoogleGenAi:
		c := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigGoogleGenAI)
		block := map[string]attr.Value{}
		// Set only one of gemini or vertex
		if c.Gemini != nil {
			geminiBlock := map[string]attr.Value{
				"api_key": types.StringValue(c.Gemini.ApiKey),
			}
			block["gemini"] = types.ObjectValueMust(data.GoogleGenAI.AttributeTypes(ctx)["gemini"].(types.ObjectType).AttributeTypes(), geminiBlock)
			block["vertex"] = types.ObjectNull(data.GoogleGenAI.AttributeTypes(ctx)["vertex"].(types.ObjectType).AttributeTypes())
		} else if c.Vertex != nil {
			vertexBlock := map[string]attr.Value{
				"service_account_key": types.StringValue(c.Vertex.ServiceAccountKey),
				"project_id":          types.StringValue(c.Vertex.ProjectID),
				"location":            types.StringValue(c.Vertex.Location),
			}
			block["vertex"] = types.ObjectValueMust(data.GoogleGenAI.AttributeTypes(ctx)["vertex"].(types.ObjectType).AttributeTypes(), vertexBlock)
			block["gemini"] = types.ObjectNull(data.GoogleGenAI.AttributeTypes(ctx)["gemini"].(types.ObjectType).AttributeTypes())
		} else {
			block["gemini"] = types.ObjectNull(data.GoogleGenAI.AttributeTypes(ctx)["gemini"].(types.ObjectType).AttributeTypes())
			block["vertex"] = types.ObjectNull(data.GoogleGenAI.AttributeTypes(ctx)["vertex"].(types.ObjectType).AttributeTypes())
		}
		data.GoogleGenAI = types.ObjectValueMust(data.GoogleGenAI.AttributeTypes(ctx), block)
	case pgql.ModelProviderTypeOpenAiCompatible:
		c := details.ProviderConfig.(*pgql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAICompatible)
		block := map[string]attr.Value{
			"url": types.StringValue(c.Url),
		}
		if c.ApiToken != nil {
			block["api_token"] = types.StringValue(*c.ApiToken)
		} else {
			block["api_token"] = types.StringNull()
		}
		data.OpenAICompat = types.ObjectValueMust(data.OpenAICompat.AttributeTypes(ctx), block)
	}
	return diag
}
