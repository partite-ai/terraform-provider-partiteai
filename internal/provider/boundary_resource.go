package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BoundaryResource{}
var _ resource.ResourceWithImportState = &BoundaryResource{}

func NewBoundaryResource() resource.Resource {
	return &BoundaryResource{
		BaseResource: NewBaseResource("boundary"),
	}
}

// BoundaryResource defines the resource implementation.
type BoundaryResource struct {
	BaseResource
}

// BoundaryResourceModel describes the resource data model.
type BoundaryResourceModel struct {
	BoundaryBaseModel
}

var boundaryActionValidator = stringvalidator.OneOf(
	"CONVERSATION_CREATE",
	"REQUEST_CREATE",
	"SKILL_PROVIDE",
	"TOOL_PROVIDE",
	"MEMORY_SEARCH_PROVIDE",
	"CONVERSATION_HISTORY_PROVIDE",
	"TOOL_CALL",
)

var boundaryRuleType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Name of the rule, unique in the boundary",
			Required:            true,
		},
		"action": schema.StringAttribute{
			MarkdownDescription: "The action the rule checks: `CONVERSATION_CREATE`, `REQUEST_CREATE`, `SKILL_PROVIDE`, `TOOL_PROVIDE`, `MEMORY_SEARCH_PROVIDE`, `CONVERSATION_HISTORY_PROVIDE` or `TOOL_CALL`",
			Required:            true,
			Validators:          []validator.String{boundaryActionValidator},
		},
		"when": schema.StringAttribute{
			MarkdownDescription: "CEL expression that decides whether the rule applies. Left out, the rule applies to every action of its kind.",
			Optional:            true,
		},
		"effect": schema.StringAttribute{
			MarkdownDescription: "`DENY` refuses the action. `ASK` makes a tool call wait for the user to approve it, and is only for `TOOL_CALL`.",
			Required:            true,
			Validators:          []validator.String{stringvalidator.OneOf("DENY", "ASK")},
		},
		"message": schema.StringAttribute{
			MarkdownDescription: "Told to whoever the rule refuses, or asks to approve. It can embed CEL expressions as `${...}`, which read what the rule's `when` reads; `$${` writes a literal `${`.",
			Optional:            true,
		},
		"report_only": schema.BoolAttribute{
			MarkdownDescription: "The rule reports what it would refuse rather than refusing it",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
		},
	},
}

var boundaryModificationType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Name of the modification, unique in the boundary",
			Required:            true,
		},
		"action": schema.StringAttribute{
			MarkdownDescription: "The action the modification sets things on: `CONVERSATION_CREATE`, `REQUEST_CREATE` or `TOOL_CALL`",
			Required:            true,
			Validators:          []validator.String{stringvalidator.OneOf("CONVERSATION_CREATE", "REQUEST_CREATE", "TOOL_CALL")},
		},
		"when": schema.StringAttribute{
			MarkdownDescription: "CEL expression that decides whether the modification applies. It can't read a tool call's arguments. Left out, the modification applies to every action of its kind.",
			Optional:            true,
		},
		"set_tags": schema.MapAttribute{
			MarkdownDescription: "For `CONVERSATION_CREATE` and `REQUEST_CREATE`: tags to set on what the action creates, each name with a CEL expression that gives its values. A string sets one value, a list several, and an empty list removes the tag.",
			ElementType:         types.StringType,
			Optional:            true,
		},
		"set_tracing": schema.StringAttribute{
			MarkdownDescription: "For `CONVERSATION_CREATE`: a CEL expression that gives whether the conversation is traced",
			Optional:            true,
		},
		"set_arguments": schema.MapAttribute{
			MarkdownDescription: "For `TOOL_CALL`: arguments to set on the call, each name with a CEL expression that gives its value. The model doesn't see an argument a boundary sets.",
			ElementType:         types.StringType,
			Optional:            true,
		},
	},
}

func (r *BoundaryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A boundary: rules that each refuse something in conversations on the meshes, or created through the access points, it is attached to, and modifications that set things on what those conversations do. Attach it with `boundary_ids` on `partiteai_mesh` or `partiteai_access_point`.",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"report_only": schema.BoolAttribute{
				MarkdownDescription: "Every rule reports what it would refuse rather than refusing it, and no modification is made",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "The rules, checked in order",
				NestedObject:        boundaryRuleType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(boundaryRuleType.Type(), nil)),
			},
			"modifications": schema.ListNestedAttribute{
				MarkdownDescription: "The modifications, made in order",
				NestedObject:        boundaryModificationType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(boundaryModificationType.Type(), nil)),
			},
			"warnings": schema.ListAttribute{
				MarkdownDescription: "What is likely wrong with the boundary, such as an expression that reads something that can be missing without checking first",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func (r *BoundaryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BoundaryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "boundary", "")

	rules, modifications, diags := boundaryInputs(ctx, data.Rules, data.Modifications)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := pgql.CreateBoundary(ctx, r.GetClient(), &pgql.NewBoundary{
		Name:          data.Name.ValueString(),
		Description:   pgql.OptPtr(data.Description.ValueStringPointer()),
		ReportOnly:    data.ReportOnly.ValueBool(),
		Rules:         rules,
		Modifications: modifications,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create boundary")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, createResp.CreateBoundary)...)

	logResourceOperation(ctx, "created", "boundary", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BoundaryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BoundaryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "boundary", data.Id.ValueString())

	getResp, err := pgql.GetBoundary(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "boundary", data.Id.ValueString())
		return
	}
	if getResp.Boundary == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.Boundary)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BoundaryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData BoundaryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "boundary", planData.Id.ValueString())

	var update pgql.UpdateBoundary
	if !planData.Name.Equal(stateData.Name) {
		update.Name = pgql.Opt(planData.Name.ValueString())
	}
	if !planData.Description.Equal(stateData.Description) {
		update.Description = pgql.OptNil(planData.Description.ValueStringPointer())
	}
	if !planData.ReportOnly.Equal(stateData.ReportOnly) {
		update.ReportOnly = pgql.Opt(planData.ReportOnly.ValueBool())
	}
	if !planData.Rules.Equal(stateData.Rules) || !planData.Modifications.Equal(stateData.Modifications) {
		rules, modifications, diags := boundaryInputs(ctx, planData.Rules, planData.Modifications)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		update.Rules = pgql.Opt(rules)
		update.Modifications = pgql.Opt(modifications)
	}

	updateResp, err := pgql.ModifyBoundary(ctx, r.GetClient(), planData.Id.ValueString(), &update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "boundary", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, updateResp.UpdateBoundary)...)

	logResourceOperation(ctx, "updated", "boundary", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *BoundaryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BoundaryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "boundary", data.Id.ValueString())

	_, err := pgql.DeleteBoundary(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "boundary", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "boundary", data.Id.ValueString())
}
