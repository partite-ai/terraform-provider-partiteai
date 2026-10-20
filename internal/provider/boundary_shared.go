package provider

import (
	"context"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// BoundaryBaseModel contains the common fields for boundary resources and data sources.
type BoundaryBaseModel struct {
	Id            types.String      `tfsdk:"id"`
	Name          types.String      `tfsdk:"name"`
	Description   types.String      `tfsdk:"description"`
	ReportOnly    types.Bool        `tfsdk:"report_only"`
	Rules         types.List        `tfsdk:"rules"`
	Modifications types.List        `tfsdk:"modifications"`
	Warnings      types.List        `tfsdk:"warnings"`
	CreatedAt     timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt     timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps a GraphQL BoundaryDetails response to the BoundaryBaseModel.
func (m *BoundaryBaseModel) MapFromGraphQL(ctx context.Context, boundaryDetails pgql.BoundaryDetails) diag.Diagnostics {
	return setFields(
		ctx,
		&m.Id, boundaryDetails.Id,
		&m.Name, boundaryDetails.Name,
		&m.Description, boundaryDetails.Description,
		&m.ReportOnly, boundaryDetails.ReportOnly,
		&m.Rules, newStructSliceMappable(boundaryDetails.Rules, boundaryRuleStruct{}),
		&m.Modifications, newStructSliceMappable(boundaryDetails.Modifications, boundaryModificationStruct{}),
		&m.Warnings, boundaryDetails.Warnings,
		&m.CreatedAt, boundaryDetails.CreatedAt,
		&m.UpdatedAt, boundaryDetails.UpdatedAt,
	)
}

type boundaryRuleStruct struct {
	Name       types.String `tfsdk:"name"`
	Action     types.String `tfsdk:"action"`
	When       types.String `tfsdk:"when"`
	Effect     types.String `tfsdk:"effect"`
	Message    types.String `tfsdk:"message"`
	ReportOnly types.Bool   `tfsdk:"report_only"`
}

func (r *boundaryRuleStruct) FromGQL(src pgql.BoundaryDetailsRulesBoundaryRule) diag.Diagnostics {
	r.Name = types.StringValue(src.Name)
	r.Action = types.StringValue(string(src.Action))
	r.When = types.StringPointerValue(src.When)
	r.Effect = types.StringValue(string(src.Effect))
	r.Message = types.StringPointerValue(src.Message)
	r.ReportOnly = types.BoolValue(src.ReportOnly)
	return nil
}

type boundaryModificationStruct struct {
	Name         types.String `tfsdk:"name"`
	Action       types.String `tfsdk:"action"`
	When         types.String `tfsdk:"when"`
	SetTags      types.Map    `tfsdk:"set_tags"`
	SetTracing   types.String `tfsdk:"set_tracing"`
	SetArguments types.Map    `tfsdk:"set_arguments"`
}

func (r *boundaryModificationStruct) FromGQL(src pgql.BoundaryDetailsModificationsBoundaryModification) diag.Diagnostics {
	var diags diag.Diagnostics
	r.Name = types.StringValue(src.Name)
	r.Action = types.StringValue(string(src.Action))
	r.When = types.StringPointerValue(src.When)
	r.SetTracing = types.StringPointerValue(src.SetTracing)

	tags := map[string]string{}
	for _, tag := range src.SetTags {
		tags[tag.Name] = tag.Expression
	}
	r.SetTags, diags = namedExpressionMap(tags)
	if diags.HasError() {
		return diags
	}
	arguments := map[string]string{}
	for _, argument := range src.SetArguments {
		arguments[argument.Name] = argument.Expression
	}
	r.SetArguments, diags = namedExpressionMap(arguments)
	return diags
}

// namedExpressionMap is expressions by name, or null when there are none, as
// a configuration that leaves the attribute out reads.
func namedExpressionMap(expressions map[string]string) (types.Map, diag.Diagnostics) {
	if len(expressions) == 0 {
		return types.MapNull(types.StringType), nil
	}
	return types.MapValueFrom(context.Background(), types.StringType, expressions)
}

// boundaryInputs reads the planned rules and modifications as the API takes them.
func boundaryInputs(ctx context.Context, rules, modifications types.List) ([]pgql.BoundaryRuleInput, []pgql.BoundaryModificationInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	var plannedRules []boundaryRuleStruct
	diags.Append(tfsdk.ValueAs(ctx, rules, &plannedRules)...)
	var plannedModifications []boundaryModificationStruct
	diags.Append(tfsdk.ValueAs(ctx, modifications, &plannedModifications)...)
	if diags.HasError() {
		return nil, nil, diags
	}

	ruleInputs := make([]pgql.BoundaryRuleInput, 0, len(plannedRules))
	for _, rule := range plannedRules {
		ruleInputs = append(ruleInputs, pgql.BoundaryRuleInput{
			Name:       rule.Name.ValueString(),
			Action:     pgql.BoundaryAction(rule.Action.ValueString()),
			When:       pgql.OptPtr(rule.When.ValueStringPointer()),
			Effect:     pgql.BoundaryEffect(rule.Effect.ValueString()),
			Message:    pgql.OptPtr(rule.Message.ValueStringPointer()),
			ReportOnly: rule.ReportOnly.ValueBool(),
		})
	}

	modificationInputs := make([]pgql.BoundaryModificationInput, 0, len(plannedModifications))
	for _, modification := range plannedModifications {
		setTags, d := namedExpressionInputs(ctx, modification.SetTags)
		diags.Append(d...)
		setArguments, d := namedExpressionInputs(ctx, modification.SetArguments)
		diags.Append(d...)
		modificationInputs = append(modificationInputs, pgql.BoundaryModificationInput{
			Name:         modification.Name.ValueString(),
			Action:       pgql.BoundaryAction(modification.Action.ValueString()),
			When:         pgql.OptPtr(modification.When.ValueStringPointer()),
			SetTags:      setTags,
			SetTracing:   pgql.OptPtr(modification.SetTracing.ValueStringPointer()),
			SetArguments: setArguments,
		})
	}
	return ruleInputs, modificationInputs, diags
}

// namedExpressionInputs is a map of expressions by name, in name order.
func namedExpressionInputs(ctx context.Context, expressions types.Map) ([]pgql.BoundaryNamedExpressionInput, diag.Diagnostics) {
	byName := map[string]string{}
	diags := expressions.ElementsAs(ctx, &byName, false)
	inputs := make([]pgql.BoundaryNamedExpressionInput, 0, len(byName))
	for _, name := range slices.Sorted(maps.Keys(byName)) {
		inputs = append(inputs, pgql.BoundaryNamedExpressionInput{Name: name, Expression: byName[name]})
	}
	return inputs, diags
}

// boundaryIDs are the IDs of the boundaries attached to a mesh or an access point.
func boundaryIDs[S any, PS interface {
	*S
	GetId() string
}](boundaries []S) []string {
	ids := make([]string, 0, len(boundaries))
	for i := range boundaries {
		ids = append(ids, PS(&boundaries[i]).GetId())
	}
	return ids
}

// boundaryAssociations attaches the boundaries of a planned boundary_ids set.
func boundaryAssociations(ctx context.Context, ids types.Set) ([]pgql.BoundaryAssociation, diag.Diagnostics) {
	var planned []string
	diags := ids.ElementsAs(ctx, &planned, false)
	associations := make([]pgql.BoundaryAssociation, 0, len(planned))
	for _, id := range planned {
		associations = append(associations, pgql.BoundaryAssociation{BoundaryID: id})
	}
	return associations, diags
}
