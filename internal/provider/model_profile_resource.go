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
var _ resource.Resource = &ModelProfileResource{}
var _ resource.ResourceWithImportState = &ModelProfileResource{}

func NewModelProfileResource() resource.Resource {
	return &ModelProfileResource{
		BaseResource: NewBaseResource("model_profile"),
	}
}

type ModelProfileResource struct {
	BaseResource
}

type ModelProfileResourceModel struct {
	ModelProfileBaseModel
}

func (r *ModelProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Profile resource",
		Attributes: standardSchemaAttributes(map[string]schema.Attribute{
			"model_provider_id": schema.StringAttribute{
				MarkdownDescription: "ID of the Model Provider",
				Required:            true,
			},
			"anthropic": schema.SingleNestedAttribute{
				MarkdownDescription: "Anthropic profile config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "Anthropic model name",
						Required:            true,
					},
					"max_tokens": schema.Int64Attribute{
						MarkdownDescription: "Maximum tokens",
						Required:            true,
					},
					"temperature": schema.Float64Attribute{
						MarkdownDescription: "Temperature",
						Optional:            true,
					},
					"top_p": schema.Float64Attribute{
						MarkdownDescription: "Top P",
						Optional:            true,
					},
					"top_k": schema.Int64Attribute{
						MarkdownDescription: "Top K",
						Optional:            true,
					},
					"thinking_budget_tokens": schema.Int64Attribute{
						MarkdownDescription: "Thinking budget tokens",
						Optional:            true,
					},
				},
			},
			"openai": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI profile config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "OpenAI model name",
						Required:            true,
					},
					"max_tokens": schema.Int64Attribute{
						MarkdownDescription: "Maximum tokens",
						Optional:            true,
					},
					"temperature": schema.Float64Attribute{
						MarkdownDescription: "Temperature",
						Optional:            true,
					},
					"top_p": schema.Float64Attribute{
						MarkdownDescription: "Top P",
						Optional:            true,
					},
					"reasoning_effort": schema.StringAttribute{
						MarkdownDescription: "Reasoning effort (LOW, MEDIUM, HIGH)",
						Optional:            true,
					},
				},
			},
			"google": schema.SingleNestedAttribute{
				MarkdownDescription: "Google profile config",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "Google model name",
						Required:            true,
					},
					"max_tokens": schema.Int64Attribute{
						MarkdownDescription: "Maximum tokens",
						Optional:            true,
					},
					"temperature": schema.Float64Attribute{
						MarkdownDescription: "Temperature",
						Optional:            true,
					},
					"top_p": schema.Float64Attribute{
						MarkdownDescription: "Top P",
						Optional:            true,
					},
					"top_k": schema.Int64Attribute{
						MarkdownDescription: "Top K",
						Optional:            true,
					},
					"thinking_budget_tokens": schema.Int64Attribute{
						MarkdownDescription: "Thinking budget tokens",
						Optional:            true,
					},
				},
			},
		}),
	}
}

func (r *ModelProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "model_profile", "")

	var createReq pgql.NewModelProfile
	createReq.Name = data.Name.ValueString()
	createReq.Description = pgql.OptPtr(data.Description.ValueStringPointer())
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

	// Only one config block should be set
	numSet := 0
	if !data.Anthropic.IsNull() && !data.Anthropic.IsUnknown() {
		numSet++
	}
	if !data.OpenAI.IsNull() && !data.OpenAI.IsUnknown() {
		numSet++
	}
	if !data.Google.IsNull() && !data.Google.IsUnknown() {
		numSet++
	}
	if numSet != 1 {
		resp.Diagnostics.AddError("Invalid config", "Exactly one of anthropic, openai, or google must be set.")
		return
	}

	createReq.ConfigurationAnthropic = nil
	createReq.ConfigurationOpenAI = nil
	createReq.ConfigurationGoogle = nil

	if !data.Anthropic.IsNull() && !data.Anthropic.IsUnknown() {
		var block struct {
			Model                string        `tfsdk:"model"`
			MaxTokens            int64         `tfsdk:"max_tokens"`
			Temperature          types.Float64 `tfsdk:"temperature"`
			TopP                 types.Float64 `tfsdk:"top_p"`
			TopK                 types.Int64   `tfsdk:"top_k"`
			ThinkingBudgetTokens types.Int64   `tfsdk:"thinking_budget_tokens"`
		}
		_ = tfsdk.ValueAs(ctx, data.Anthropic, &block)
		createReq.ConfigurationAnthropic = &pgql.ModelProfileConfigInputAnthropic{
			Model:                block.Model,
			MaxTokens:            int(block.MaxTokens),
			Temperature:          pgql.OptPtr(block.Temperature.ValueFloat64Pointer()),
			TopP:                 pgql.OptPtr(block.TopP.ValueFloat64Pointer()),
			TopK:                 pgql.OptPtr(block.TopK.ValueInt64Pointer()),
			ThinkingBudgetTokens: pgql.OptPtr(block.ThinkingBudgetTokens.ValueInt64Pointer()),
		}
	}
	if !data.OpenAI.IsNull() && !data.OpenAI.IsUnknown() {
		var block struct {
			Model           string        `tfsdk:"model"`
			MaxTokens       types.Int64   `tfsdk:"max_tokens"`
			Temperature     types.Float64 `tfsdk:"temperature"`
			TopP            types.Float64 `tfsdk:"top_p"`
			ReasoningEffort types.String  `tfsdk:"reasoning_effort"`
		}
		_ = tfsdk.ValueAs(ctx, data.OpenAI, &block)
		createReq.ConfigurationOpenAI = &pgql.ModelProfileConfigInputOpenAI{
			Model:           block.Model,
			MaxTokens:       pgql.OptPtr(block.MaxTokens.ValueInt64Pointer()),
			Temperature:     pgql.OptPtr(block.Temperature.ValueFloat64Pointer()),
			TopP:            pgql.OptPtr(block.TopP.ValueFloat64Pointer()),
			ReasoningEffort: pgql.OptPtr(block.ReasoningEffort.ValueStringPointer()),
		}
	}
	if !data.Google.IsNull() && !data.Google.IsUnknown() {
		var block struct {
			Model                string        `tfsdk:"model"`
			MaxTokens            types.Int64   `tfsdk:"max_tokens"`
			Temperature          types.Float64 `tfsdk:"temperature"`
			TopP                 types.Float64 `tfsdk:"top_p"`
			TopK                 types.Int64   `tfsdk:"top_k"`
			ThinkingBudgetTokens types.Int64   `tfsdk:"thinking_budget_tokens"`
		}
		_ = tfsdk.ValueAs(ctx, data.Google, &block)
		createReq.ConfigurationGoogle = &pgql.ModelProfileConfigInputGoogle{
			Model:                block.Model,
			MaxTokens:            pgql.OptPtr(block.MaxTokens.ValueInt64Pointer()),
			Temperature:          pgql.OptPtr(block.Temperature.ValueFloat64Pointer()),
			TopP:                 pgql.OptPtr(block.TopP.ValueFloat64Pointer()),
			TopK:                 pgql.OptPtr(block.TopK.ValueInt64Pointer()),
			ThinkingBudgetTokens: pgql.OptPtr(block.ThinkingBudgetTokens.ValueInt64Pointer()),
		}
	}

	createResp, err := pgql.CreateModelProfile(ctx, r.GetClient(), data.ModelProviderId.ValueString(), &createReq)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create model profile")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, createResp.CreateModelProfile)...)

	logResourceOperation(ctx, "created", "model_profile", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "model_profile", data.Id.ValueString())

	modelProfileResp, err := pgql.GetModelProfile(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "model_profile", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, modelProfileResp.ModelProfile)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData ModelProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "model_profile", planData.Id.ValueString())

	var update pgql.UpdateModelProfile
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}

	// Only one config block should be set
	numSet := 0
	if !planData.Anthropic.IsNull() && !planData.Anthropic.IsUnknown() {
		numSet++
	}
	if !planData.OpenAI.IsNull() && !planData.OpenAI.IsUnknown() {
		numSet++
	}
	if !planData.Google.IsNull() && !planData.Google.IsUnknown() {
		numSet++
	}
	if numSet != 1 {
		resp.Diagnostics.AddError("Invalid config", "Exactly one of anthropic, openai, or google must be set.")
		return
	}

	update.ConfigurationAnthropic = nil
	update.ConfigurationOpenAI = nil
	update.ConfigurationGoogle = nil

	if !planData.Anthropic.Equal(stateData.Anthropic) && !planData.Anthropic.IsNull() {
		var block struct {
			Model                string        `tfsdk:"model"`
			MaxTokens            int64         `tfsdk:"max_tokens"`
			Temperature          types.Float64 `tfsdk:"temperature"`
			TopP                 types.Float64 `tfsdk:"top_p"`
			TopK                 types.Int64   `tfsdk:"top_k"`
			ThinkingBudgetTokens types.Int64   `tfsdk:"thinking_budget_tokens"`
		}
		_ = tfsdk.ValueAs(ctx, planData.Anthropic, &block)
		update.ConfigurationAnthropic = &pgql.ModelProfileConfigInputAnthropic{
			Model:                block.Model,
			MaxTokens:            int(block.MaxTokens),
			Temperature:          pgql.OptPtr(block.Temperature.ValueFloat64Pointer()),
			TopP:                 pgql.OptPtr(block.TopP.ValueFloat64Pointer()),
			TopK:                 pgql.OptPtr(block.TopK.ValueInt64Pointer()),
			ThinkingBudgetTokens: pgql.OptPtr(block.ThinkingBudgetTokens.ValueInt64Pointer()),
		}
	}
	if !planData.OpenAI.Equal(stateData.OpenAI) && !planData.OpenAI.IsNull() {
		var block struct {
			Model           string        `tfsdk:"model"`
			MaxTokens       types.Int64   `tfsdk:"max_tokens"`
			Temperature     types.Float64 `tfsdk:"temperature"`
			TopP            types.Float64 `tfsdk:"top_p"`
			ReasoningEffort types.String  `tfsdk:"reasoning_effort"`
		}
		_ = tfsdk.ValueAs(ctx, planData.OpenAI, &block)
		update.ConfigurationOpenAI = &pgql.ModelProfileConfigInputOpenAI{
			Model:           block.Model,
			MaxTokens:       pgql.OptPtr(block.MaxTokens.ValueInt64Pointer()),
			Temperature:     pgql.OptPtr(block.Temperature.ValueFloat64Pointer()),
			TopP:            pgql.OptPtr(block.TopP.ValueFloat64Pointer()),
			ReasoningEffort: pgql.OptPtr(block.ReasoningEffort.ValueStringPointer()),
		}
	}
	if !planData.Google.Equal(stateData.Google) && !planData.Google.IsNull() {
		var block struct {
			Model                string        `tfsdk:"model"`
			MaxTokens            types.Int64   `tfsdk:"max_tokens"`
			Temperature          types.Float64 `tfsdk:"temperature"`
			TopP                 types.Float64 `tfsdk:"top_p"`
			TopK                 types.Int64   `tfsdk:"top_k"`
			ThinkingBudgetTokens types.Int64   `tfsdk:"thinking_budget_tokens"`
		}
		_ = tfsdk.ValueAs(ctx, planData.Google, &block)
		update.ConfigurationGoogle = &pgql.ModelProfileConfigInputGoogle{
			Model:                block.Model,
			MaxTokens:            pgql.OptPtr(block.MaxTokens.ValueInt64Pointer()),
			Temperature:          pgql.OptPtr(block.Temperature.ValueFloat64Pointer()),
			TopP:                 pgql.OptPtr(block.TopP.ValueFloat64Pointer()),
			TopK:                 pgql.OptPtr(block.TopK.ValueInt64Pointer()),
			ThinkingBudgetTokens: pgql.OptPtr(block.ThinkingBudgetTokens.ValueInt64Pointer()),
		}
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

	updateResp, err := pgql.ModifyModelProfile(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "model_profile", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, updateResp.UpdateModelProfile)...)

	logResourceOperation(ctx, "updated", "model_profile", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ModelProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "model_profile", data.Id.ValueString())

	_, err := pgql.DeleteModelProfile(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "model_profile", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "model_profile", data.Id.ValueString())
}

func (r *ModelProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ModelProfileResource) mapResponse(ctx context.Context, data *ModelProfileResourceModel, details pgql.ModelProfileDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, details)
}
