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
	Probe types.Bool `tfsdk:"probe"`
}

func (r *ModelProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Profile resource",
		Attributes: standardSchemaAttributes(map[string]schema.Attribute{
			"model_provider_id": schema.StringAttribute{
				MarkdownDescription: "ID of the Model Provider",
				Required:            true,
			},
			"configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "The model and the settings it is called with. Which of these a model " +
					"accepts is reported by its provider, as a model's supported parameters, rather than being " +
					"fixed by the vendor, so this is one block for every provider.",
				Required: true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "The name the service knows the model by.",
						Required:            true,
					},
					"max_tokens": schema.Int64Attribute{
						MarkdownDescription: "The most tokens the model may produce in one response.",
						Optional:            true,
					},
					"temperature": schema.Float64Attribute{
						MarkdownDescription: "How much the model is allowed to vary its output.",
						Optional:            true,
					},
					"top_p": schema.Float64Attribute{
						MarkdownDescription: "Nucleus sampling cutoff.",
						Optional:            true,
					},
					"top_k": schema.Int64Attribute{
						MarkdownDescription: "How many candidate tokens the model considers.",
						Optional:            true,
					},
					"thinking_budget_tokens": schema.Int64Attribute{
						MarkdownDescription: "Tokens the model may spend reasoning before it answers.",
						Optional:            true,
					},
					"reasoning_effort": schema.StringAttribute{
						MarkdownDescription: "How hard the model should think: NONE, MINIMAL, LOW, MEDIUM, HIGH, XHIGH or MAX.",
						Optional:            true,
					},
					"thinking_type": schema.StringAttribute{
						MarkdownDescription: "How the model thinks: ADAPTIVE, ENABLED (needs thinking_budget_tokens), DISABLED or BETWEEN_TOOLS.",
						Optional:            true,
					},
					"thinking_display": schema.StringAttribute{
						MarkdownDescription: "What the model returns of its thinking: SUMMARIZED (reasoning summaries with progress updates mixed in), UPDATES (only the progress updates it writes between tool calls) or OMITTED.",
						Optional:            true,
					},
				},
			},
			"probe": probeAttribute,
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

	config, ok := modelProfileConfig(ctx, data.Configuration, &resp.Diagnostics)
	if !ok {
		return
	}
	createReq.Configuration = *config
	createReq.Probe = configProbe(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
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

	if !planData.Configuration.Equal(stateData.Configuration) {
		config, ok := modelProfileConfig(ctx, planData.Configuration, &resp.Diagnostics)
		if !ok {
			return
		}
		update.Configuration = config
	}

	update.Probe = configProbe(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
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
