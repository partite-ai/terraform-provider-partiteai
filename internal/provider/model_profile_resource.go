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
var _ resource.Resource = &ModelProfileResource{}
var _ resource.ResourceWithImportState = &ModelProfileResource{}

func NewModelProfileResource() resource.Resource {
	return &ModelProfileResource{}
}

type ModelProfileResource struct {
	client graphql.Client
}

type ModelProfileResourceModel struct {
	Id              types.String      `tfsdk:"id"`
	ModelProviderId types.String      `tfsdk:"model_provider_id"`
	Name            types.String      `tfsdk:"name"`
	Description     types.String      `tfsdk:"description"`
	Tags            types.List        `tfsdk:"tags"`
	CreatedAt       timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt       timetypes.RFC3339 `tfsdk:"updated_at"`
	Anthropic       types.Object      `tfsdk:"anthropic"`
	OpenAI          types.Object      `tfsdk:"openai"`
	Google          types.Object      `tfsdk:"google"`
}

func (r *ModelProfileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_profile"
}

func (r *ModelProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Profile resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Model Profile identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"model_provider_id": schema.StringAttribute{
				MarkdownDescription: "ID of the Model Provider",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Model Profile name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the Model Profile",
				Optional:            true,
			},
			"tags": schema.ListNestedAttribute{
				MarkdownDescription: "Tags for the Model Profile",
				NestedObject:        tagType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(tagType.Type(), nil)),
			},
			"created_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the Model Profile was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the Model Profile was last updated",
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
		},
	}
}

func (r *ModelProfileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ModelProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
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

	createResp, err := pgql.CreateModelProfile(ctx, r.client, data.ModelProviderId.ValueString(), &createReq)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create model profile, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &data, createResp.CreateModelProfile); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	tflog.Trace(ctx, "created a model profile resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelProfileResp, err := pgql.GetModelProfile(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read model profile, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &data, modelProfileResp.ModelProfile); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData ModelProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}
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
	updateResp, err := pgql.ModifyModelProfile(ctx, r.client, planData.Id.ValueString(), &update)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update model profile, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &planData, updateResp.UpdateModelProfile); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ModelProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := pgql.DeleteModelProfile(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete model profile, got error: %s", err))
		return
	}
}

func (r *ModelProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ModelProfileResource) mapFields(ctx context.Context, data *ModelProfileResourceModel, details pgql.ModelProfileDetails) diag.Diagnostics {
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
	data.Google = types.ObjectNull(data.Google.AttributeTypes(ctx))
	// Set the correct block based on config type
	switch c := details.Configuration.(type) {
	case *pgql.ModelProfileDetailsConfigurationModelProfileConfigAnthropic:
		block := map[string]attr.Value{
			"model":                  types.StringValue(c.Model),
			"max_tokens":             types.Int64PointerValue(int64Ptr(c.MaxTokens)),
			"temperature":            types.Float64PointerValue(c.Temperature),
			"top_p":                  types.Float64PointerValue(c.TopP),
			"top_k":                  types.Int64PointerValue(int64Ptr(c.TopK)),
			"thinking_budget_tokens": types.Int64PointerValue(int64Ptr(c.ThinkingBudgetTokens)),
		}
		data.Anthropic = types.ObjectValueMust(data.Anthropic.AttributeTypes(ctx), block)
	case *pgql.ModelProfileDetailsConfigurationModelProfileConfigOpenAI:
		block := map[string]attr.Value{
			"model":       types.StringValue(c.Model),
			"max_tokens":  types.Int64PointerValue(int64Ptr(c.MaxTokens)),
			"temperature": types.Float64PointerValue(c.Temperature),
			"top_p":       types.Float64PointerValue(c.TopP),
			"reasoning_effort": func() types.String {
				if c.ReasoningEffort != nil {
					return types.StringValue(string(*c.ReasoningEffort))
				}
				return types.StringNull()
			}(),
		}
		data.OpenAI = types.ObjectValueMust(data.OpenAI.AttributeTypes(ctx), block)
	case *pgql.ModelProfileDetailsConfigurationModelProfileConfigGoogle:
		block := map[string]attr.Value{
			"model":                  types.StringValue(c.Model),
			"max_tokens":             types.Int64PointerValue(int64Ptr(c.MaxTokens)),
			"temperature":            types.Float64PointerValue(c.Temperature),
			"top_p":                  types.Float64PointerValue(c.TopP),
			"top_k":                  types.Int64PointerValue(int64Ptr(c.TopK)),
			"thinking_budget_tokens": types.Int64PointerValue(int64Ptr(c.ThinkingBudgetTokens)),
		}
		data.Google = types.ObjectValueMust(data.Google.AttributeTypes(ctx), block)
	}
	return diag
}
