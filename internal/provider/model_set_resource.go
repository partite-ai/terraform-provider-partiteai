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
var _ resource.Resource = &ModelSetResource{}
var _ resource.ResourceWithImportState = &ModelSetResource{}

func NewModelSetResource() resource.Resource {
	return &ModelSetResource{}
}

type ModelSetResource struct {
	client graphql.Client
}

type ModelSetResourceModel struct {
	Id                  types.String      `tfsdk:"id"`
	Name                types.String      `tfsdk:"name"`
	Description         types.String      `tfsdk:"description"`
	Tags                types.List        `tfsdk:"tags"`
	DefaultModelProfile types.Object      `tfsdk:"default_model_profile"`
	Rules               types.List        `tfsdk:"rules"`
	CreatedAt           timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt           timetypes.RFC3339 `tfsdk:"updated_at"`
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

func (r *ModelSetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_set"
}

func (r *ModelSetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Model Set resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Model Set identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Model Set name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the Model Set",
				Optional:            true,
			},
			"tags": schema.ListNestedAttribute{
				MarkdownDescription: "Tags for the Model Set",
				NestedObject:        tagType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(tagType.Type(), nil)),
			},
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
							NestedObject:        tagType,
							Optional:            true,
							Computed:            true,
							Default:             listdefault.StaticValue(types.ListValueMust(tagType.Type(), nil)),
						},
					},
				},
				Required: true,
			},
			"created_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the Model Set was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the Model Set was last updated",
			},
		},
	}
}

func (r *ModelSetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ModelSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelSetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	if profileID, diags := resolveModelProfileID(ctx, r.client, data.DefaultModelProfile); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	} else {
		createReq.DefaultModelProfileID = profileID
	}

	// Resolve rules
	var rules []struct {
		ModelProfile types.Object `tfsdk:"model_profile"`
		Complexity   types.String `tfsdk:"complexity"`
		MatchTags    types.Set    `tfsdk:"match_tags"`
	}
	if diag := tfsdk.ValueAs(ctx, data.Rules, &rules); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	for _, rule := range rules {
		modelProfileID, diags := resolveModelProfileID(ctx, r.client, rule.ModelProfile)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		var matchTagsOpt pgql.Optional[[]*pgql.Tag]
		if !rule.MatchTags.IsNull() && !rule.MatchTags.IsUnknown() {
			var tags []struct {
				Name  string `tfsdk:"name"`
				Value string `tfsdk:"value"`
			}
			if diags := tfsdk.ValueAs(ctx, rule.MatchTags, &tags); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			var matchTags []*pgql.Tag
			for _, tag := range tags {
				matchTags = append(matchTags, &pgql.Tag{Name: tag.Name, Value: tag.Value})
			}
			matchTagsOpt = pgql.Opt(matchTags)
		}
		createReq.Rules = append(createReq.Rules, pgql.ModelSetRuleInput{
			ModelProfileID: modelProfileID,
			Complexity:     pgql.OptPtr(rule.Complexity.ValueStringPointer()),
			MatchTags:      matchTagsOpt,
		})
	}

	createResp, err := pgql.CreateModelSet(ctx, r.client, &createReq)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create model set, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &data, createResp.CreateModelSet); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	tflog.Trace(ctx, "created a model set resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelSetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelSetResp, err := pgql.GetModelSet(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read model set, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &data, modelSetResp.ModelSet); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData ModelSetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var update pgql.UpdateModelSet
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}
	if !planData.Tags.Equal(stateData.Tags) {
		var tags []struct {
			Name  string `tfsdk:"name"`
			Value string `tfsdk:"value"`
		}
		var updateTags []*pgql.Tag
		_ = tfsdk.ValueAs(ctx, planData.Tags, &tags)
		for _, tag := range tags {
			updateTags = append(updateTags, &pgql.Tag{Name: tag.Name, Value: tag.Value})
		}
		update.Tags = pgql.Opt(updateTags)
	}

	if !planData.DefaultModelProfile.Equal(stateData.DefaultModelProfile) {
		modelProfileID, diags := resolveModelProfileID(ctx, r.client, planData.DefaultModelProfile)
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
				MatchTags    types.Set    `tfsdk:"match_tags"`
			}
			if diags := tfsdk.ValueAs(ctx, rule, &ruleData); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}

			modelProfileID, diags := resolveModelProfileID(ctx, r.client, ruleData.ModelProfile)
			if diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			var matchTagsOpt pgql.Optional[[]*pgql.Tag]
			if !ruleData.MatchTags.IsNull() && !ruleData.MatchTags.IsUnknown() {
				var tags []struct {
					Name  string `tfsdk:"name"`
					Value string `tfsdk:"value"`
				}
				if diags := tfsdk.ValueAs(ctx, ruleData.MatchTags, &tags); diags.HasError() {
					resp.Diagnostics.Append(diags...)
					return
				}
				var matchTags []*pgql.Tag
				for _, tag := range tags {
					matchTags = append(matchTags, &pgql.Tag{Name: tag.Name, Value: tag.Value})
				}
				matchTagsOpt = pgql.Opt(matchTags)
			}
			rules = append(rules, pgql.ModelSetRuleInput{
				ModelProfileID: modelProfileID,
				Complexity:     pgql.OptPtr(ruleData.Complexity.ValueStringPointer()),
				MatchTags:      matchTagsOpt,
			})
		}

		update.Rules = pgql.Opt(rules)
	}

	updateResp, err := pgql.ModifyModelSet(ctx, r.client, planData.Id.ValueString(), &update)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update model set, got error: %s", err))
		return
	}
	if diag := r.mapFields(ctx, &planData, updateResp.UpdateModelSet); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *ModelSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelSetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := pgql.DeleteModelSet(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete model set, got error: %s", err))
		return
	}
}

func (r *ModelSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ModelSetResource) mapFields(ctx context.Context, data *ModelSetResourceModel, details pgql.ModelSetDetails) diag.Diagnostics {
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
	// Default model profile block
	data.DefaultModelProfile = types.ObjectValueMust(map[string]attr.Type{
		"id":          types.StringType,
		"provider_id": types.StringType,
		"name":        types.StringType,
	}, map[string]attr.Value{
		"id":          types.StringValue(details.DefaultModelProfile.Id),
		"provider_id": types.StringValue(details.DefaultModelProfile.Provider.Id),
		"name":        types.StringValue(details.DefaultModelProfile.Name),
	})

	// Rules
	var rules []attr.Value
	for _, rule := range details.Rules {

		var matchTags []attr.Value
		for _, tag := range rule.MatchTags {
			matchTags = append(matchTags, types.ObjectValueMust(map[string]attr.Type{
				"name":  types.StringType,
				"value": types.StringType,
			}, map[string]attr.Value{
				"name":  types.StringValue(tag.Name),
				"value": types.StringValue(tag.Value),
			}))
		}
		ruleObj := map[string]attr.Value{
			"model_profile": types.ObjectValueMust(map[string]attr.Type{
				"id":          types.StringType,
				"provider_id": types.StringType,
				"name":        types.StringType,
			}, map[string]attr.Value{
				"id":          types.StringValue(rule.ModelProfileID),
				"provider_id": types.StringValue(rule.ModelProviderID),
				"name":        types.StringValue(rule.ModelProfileName),
			}),
			"complexity": types.StringPointerValue((*string)(rule.Complexity)),
			"match_tags": types.ListValueMust(tagType.Type(), matchTags),
		}
		rules = append(rules, types.ObjectValueMust(map[string]attr.Type{
			"model_profile": modelProfileRefType("").GetType(),
			"complexity":    types.StringType,
			"match_tags":    types.ListType{ElemType: tagType.Type()},
		}, ruleObj))
	}
	data.Rules = types.ListValueMust(data.Rules.ElementType(ctx), rules)
	return diag
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
