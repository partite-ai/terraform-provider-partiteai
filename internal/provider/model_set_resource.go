package provider

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ModelSetResource{}
var _ resource.ResourceWithImportState = &ModelSetResource{}

func NewModelSetResource() resource.Resource {
	return &ModelSetResource{
		BaseResource: NewBaseResource("model_set"),
	}
}

type ModelSetResource struct {
	BaseResource
}

type ModelSetResourceModel struct {
	ModelSetBaseModel
}

func modelProfileRefType(name string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: name,
		Required:            true,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Model Profile ID",
				Optional:            true,
				Computed:            true,
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "Model Provider ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Model Profile Name",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

type modelProfileRefStruct struct {
	ModelProfileId   types.String `tfsdk:"id"`
	ModelProviderId  types.String `tfsdk:"provider_id"`
	ModelProfileName types.String `tfsdk:"name"`
}

func (r *ModelSetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Set resource",
		Attributes: standardSchemaAttributes(map[string]schema.Attribute{
			"default_model_profile": modelProfileRefType("Default Model Profile"),
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "Rules for selecting model profiles",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"model_profile": modelProfileRefType("Model Profile"),
						"complexity": schema.StringAttribute{
							MarkdownDescription: "Model complexity",
							Optional:            true,
						},
						"match_tags": schema.ListNestedAttribute{
							MarkdownDescription: "Tags to match for this rule",
							NestedObject:        tagObjectType,
							Optional:            true,
							Computed:            true,
							Default:             listdefault.StaticValue(types.ListValueMust(tagObjectType.Type(), nil)),
						},
					},
				},
				Required: true,
			},
		}),
	}
}

func (r *ModelSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelSetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "model_set", "")

	var createReq pgql.NewModelSet
	createReq.Name = data.Name.ValueString()
	createReq.Description = pgql.OptPtr(data.Description.ValueStringPointer())
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

	if profileID, diags := resolveModelProfileID(ctx, r.GetClient(), data.DefaultModelProfile); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	} else {
		createReq.DefaultModelProfileID = profileID
	}

	// Resolve rules
	var rules []struct {
		ModelProfile types.Object `tfsdk:"model_profile"`
		Complexity   types.String `tfsdk:"complexity"`
		MatchTags    types.List   `tfsdk:"match_tags"`
	}
	if diag := tfsdk.ValueAs(ctx, data.Rules, &rules); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	for _, rule := range rules {
		modelProfileID, diags := resolveModelProfileID(ctx, r.GetClient(), rule.ModelProfile)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		var matchTagsOpt pgql.Optional[[]*pgql.Tag]
		if !rule.MatchTags.IsNull() && !rule.MatchTags.IsUnknown() {
			var tags []*pgql.Tag
			if diags := tfsdk.ValueAs(ctx, rule.MatchTags, &tags); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			matchTagsOpt = pgql.Opt(tags)
		}
		createReq.Rules = append(createReq.Rules, pgql.ModelSetRuleInput{
			ModelProfileID: modelProfileID,
			Complexity:     pgql.OptPtr(rule.Complexity.ValueStringPointer()),
			MatchTags:      matchTagsOpt,
		})
	}

	createResp, err := pgql.CreateModelSet(ctx, r.GetClient(), &createReq)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create model set")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, createResp.CreateModelSet)...)

	logResourceOperation(ctx, "created", "model_set", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelSetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "model_set", data.Id.ValueString())

	modelSetResp, err := pgql.GetModelSet(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "model_set", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, modelSetResp.ModelSet)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData ModelSetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "model_set", planData.Id.ValueString())

	var update pgql.UpdateModelSet
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}
	if !planData.Tags.Equal(stateData.Tags) {
		var updateTags []*pgql.Tag
		_ = tfsdk.ValueAs(ctx, planData.Tags, &updateTags)
		update.Tags = pgql.Opt(updateTags)
	}

	if !planData.DefaultModelProfile.Equal(stateData.DefaultModelProfile) {
		modelProfileID, diags := resolveModelProfileID(ctx, r.GetClient(), planData.DefaultModelProfile)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		update.DefaultModelProfileID = pgql.Opt(modelProfileID)
	}

	if !planData.Rules.Equal(stateData.Rules) {
		var rules []pgql.ModelSetRuleInput
		for _, rule := range planData.Rules.Elements() {
			var ruleData struct {
				ModelProfile types.Object `tfsdk:"model_profile"`
				Complexity   types.String `tfsdk:"complexity"`
				MatchTags    types.List   `tfsdk:"match_tags"`
			}
			if diags := tfsdk.ValueAs(ctx, rule, &ruleData); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}

			modelProfileID, diags := resolveModelProfileID(ctx, r.GetClient(), ruleData.ModelProfile)
			if diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			var matchTagsOpt pgql.Optional[[]*pgql.Tag]
			if !ruleData.MatchTags.IsNull() && !ruleData.MatchTags.IsUnknown() {
				var tags []*pgql.Tag
				if diags := tfsdk.ValueAs(ctx, ruleData.MatchTags, &tags); diags.HasError() {
					resp.Diagnostics.Append(diags...)
					return
				}
				matchTagsOpt = pgql.Opt(tags)
			}
			rules = append(rules, pgql.ModelSetRuleInput{
				ModelProfileID: modelProfileID,
				Complexity:     pgql.OptPtr(ruleData.Complexity.ValueStringPointer()),
				MatchTags:      matchTagsOpt,
			})
		}

		update.Rules = pgql.Opt(rules)
	}

	updateResp, err := pgql.ModifyModelSet(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "model_set", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, updateResp.UpdateModelSet)...)

	logResourceOperation(ctx, "updated", "model_set", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ModelSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelSetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "model_set", data.Id.ValueString())

	_, err := pgql.DeleteModelSet(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "model_set", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "model_set", data.Id.ValueString())
}

func (r *ModelSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ModelSetResource) mapResponse(ctx context.Context, data *ModelSetResourceModel, details pgql.ModelSetDetails) diag.Diagnostics {
	return data.MapFromGraphQL(ctx, details)
}

func resolveModelProfileID(ctx context.Context, client graphql.Client, ref types.Object) (string, diag.Diagnostics) {
	var refStruct modelProfileRefStruct
	if diag := tfsdk.ValueAs(ctx, ref, &refStruct); diag.HasError() {
		return "", diag
	}

	var diags diag.Diagnostics
	if !refStruct.ModelProfileId.IsNull() && !refStruct.ModelProfileId.IsUnknown() {
		return refStruct.ModelProfileId.ValueString(), diags
	}

	if !refStruct.ModelProviderId.IsNull() && !refStruct.ModelProfileName.IsNull() &&
		!refStruct.ModelProviderId.IsUnknown() && !refStruct.ModelProfileName.IsUnknown() {
		// List model profiles for the provider, find the one with the given name
		// This assumes you have a way to list profiles by providerID
		profile, err := pgql.GetModelProfileByProviderAndName(ctx, client, refStruct.ModelProviderId.ValueString(), refStruct.ModelProfileName.ValueString())
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to find model profile for provider %s and name %s: %s", refStruct.ModelProviderId.ValueString(), refStruct.ModelProfileName.ValueString(), err))
		}
		return profile.ModelProfileByProviderAndName.Id, diags
	}

	diags.AddError("Invalid model profile reference", "Must specify either id or both provider_id and name.")
	return "", diags
}

type modelSetRuleStruct struct {
	ModelProfile *modelProfileRefStruct `tfsdk:"model_profile"`
	Complexity   types.String           `tfsdk:"complexity"`
	MatchTags    []*pgql.Tag            `tfsdk:"match_tags"`
}

func (r *modelSetRuleStruct) FromGQL(src pgql.ModelSetDetailsRulesModelSetRule) diag.Diagnostics {
	var diags diag.Diagnostics

	var modelProfile modelProfileRefStruct
	diags.Append(setFields(
		context.Background(),
		&modelProfile.ModelProfileId, src.ModelProfileID,
		&modelProfile.ModelProviderId, src.ModelProviderID,
		&modelProfile.ModelProfileName, src.ModelProfileName,
	)...)

	r.ModelProfile = &modelProfile
	r.MatchTags = src.MatchTags
	r.Complexity = types.StringPointerValue((*string)(src.Complexity))
	return diags
}
