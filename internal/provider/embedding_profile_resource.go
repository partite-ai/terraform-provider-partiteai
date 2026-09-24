package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &EmbeddingProfileResource{}
var _ resource.ResourceWithImportState = &EmbeddingProfileResource{}

func NewEmbeddingProfileResource() resource.Resource {
	return &EmbeddingProfileResource{
		BaseResource: NewBaseResource("embedding_profile"),
	}
}

type EmbeddingProfileResource struct {
	BaseResource
}

type EmbeddingProfileResourceModel struct {
	EmbeddingProfileBaseModel
}

func (r *EmbeddingProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An embedding profile: the model a memory store writes its vectors with.",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"created_at":  commonSchemas.createdAt,
			"updated_at":  commonSchemas.updatedAt,
			"model_provider_id": schema.StringAttribute{
				MarkdownDescription: "ID of the model provider this profile belongs to",
				Required:            true,
			},
			"dimensions": schema.Int64Attribute{
				MarkdownDescription: "The width of the vectors this profile produces. A memory store keeps every vector of one width together, so changing it means re-embedding what the store already holds.",
				Computed:            true,
			},
			"openai": schema.SingleNestedAttribute{
				MarkdownDescription: "OpenAI embedding configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "The name the service knows the embedding model by, such as text-embedding-3-small",
						Required:            true,
					},
					"dimensions": schema.Int64Attribute{
						MarkdownDescription: "The width of the vectors this profile produces",
						Required:            true,
					},
					"send_dimensions": schema.BoolAttribute{
						MarkdownDescription: "Ask the service for that width with each request. Set it only for a model that can be asked for fewer dimensions than it produces by default.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},
			"google": schema.SingleNestedAttribute{
				MarkdownDescription: "Google embedding configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						MarkdownDescription: "The name the service knows the embedding model by, such as gemini-embedding-001",
						Required:            true,
					},
					"dimensions": schema.Int64Attribute{
						MarkdownDescription: "The width of the vectors this profile produces",
						Required:            true,
					},
					"send_dimensions": schema.BoolAttribute{
						MarkdownDescription: "Ask the service for that width with each request",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"task_type": schema.StringAttribute{
						MarkdownDescription: "What the embedding is for, such as RETRIEVAL_DOCUMENT",
						Optional:            true,
					},
				},
			},
		},
	}
}

func (r *EmbeddingProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EmbeddingProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "embedding_profile", "")

	openAI, google, ok := embeddingProfileConfig(ctx, data, &resp.Diagnostics)
	if !ok {
		return
	}

	createResp, err := pgql.CreateEmbeddingProfile(ctx, r.GetClient(),
		data.ModelProviderId.ValueString(), &pgql.NewEmbeddingProfile{
			Name:                data.Name.ValueString(),
			Description:         pgql.OptPtr(data.Description.ValueStringPointer()),
			ConfigurationOpenAI: openAI,
			ConfigurationGoogle: google,
		})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create embedding profile")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, createResp.CreateEmbeddingProfile)...)

	logResourceOperation(ctx, "created", "embedding_profile", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EmbeddingProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EmbeddingProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "embedding_profile", data.Id.ValueString())

	profileResp, err := pgql.GetEmbeddingProfile(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "embedding_profile", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, profileResp.EmbeddingProfile)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EmbeddingProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData EmbeddingProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "embedding_profile", planData.Id.ValueString())

	var update pgql.UpdateEmbeddingProfile
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}

	openAI, google, ok := embeddingProfileConfig(ctx, planData, &resp.Diagnostics)
	if !ok {
		return
	}
	if !planData.OpenAI.Equal(stateData.OpenAI) {
		update.ConfigurationOpenAI = openAI
	}
	if !planData.Google.Equal(stateData.Google) {
		update.ConfigurationGoogle = google
	}

	updateResp, err := pgql.ModifyEmbeddingProfile(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "embedding_profile", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, updateResp.UpdateEmbeddingProfile)...)

	logResourceOperation(ctx, "updated", "embedding_profile", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *EmbeddingProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EmbeddingProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "embedding_profile", data.Id.ValueString())

	if _, err := pgql.DeleteEmbeddingProfile(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "embedding_profile", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "embedding_profile", data.Id.ValueString())
}

func (r *EmbeddingProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// embeddingProfileConfig reads whichever provider block the configuration set.
// Exactly one of them is allowed, because a profile belongs to one provider.
func embeddingProfileConfig(
	ctx context.Context,
	data EmbeddingProfileResourceModel,
	diags *diag.Diagnostics,
) (*pgql.EmbeddingProfileConfigInputOpenAI, *pgql.EmbeddingProfileConfigInputGoogle, bool) {
	hasOpenAI := !data.OpenAI.IsNull() && !data.OpenAI.IsUnknown()
	hasGoogle := !data.Google.IsNull() && !data.Google.IsUnknown()
	if hasOpenAI == hasGoogle {
		diags.AddError("Invalid configuration", "Set exactly one of openai or google.")
		return nil, nil, false
	}

	if hasOpenAI {
		var block embeddingProfileOpenAIBlock
		if d := tfsdk.ValueAs(ctx, data.OpenAI, &block); d.HasError() {
			diags.Append(d...)
			return nil, nil, false
		}
		return &pgql.EmbeddingProfileConfigInputOpenAI{
			Model:          block.Model,
			Dimensions:     int(block.Dimensions),
			SendDimensions: pgql.OptPtr(block.SendDimensions.ValueBoolPointer()),
		}, nil, true
	}

	var block embeddingProfileGoogleBlock
	if d := tfsdk.ValueAs(ctx, data.Google, &block); d.HasError() {
		diags.Append(d...)
		return nil, nil, false
	}
	return nil, &pgql.EmbeddingProfileConfigInputGoogle{
		Model:          block.Model,
		Dimensions:     int(block.Dimensions),
		SendDimensions: pgql.OptPtr(block.SendDimensions.ValueBoolPointer()),
		TaskType:       pgql.OptPtr(block.TaskType.ValueStringPointer()),
	}, true
}
