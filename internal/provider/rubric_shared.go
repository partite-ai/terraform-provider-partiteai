package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// RubricBaseModel contains all the common fields for both resource and data
// source.
type RubricBaseModel struct {
	Id          types.String      `tfsdk:"id"`
	Name        types.String      `tfsdk:"name"`
	Description types.String      `tfsdk:"description"`
	Metrics     types.Set         `tfsdk:"metrics"`
	CreatedAt   timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt   timetypes.RFC3339 `tfsdk:"updated_at"`
}

// rubricMetricInfo is one metric a rubric scores against. A metric is either
// one of yours or one that came with the product, so exactly one of
// user_metric_id and system_metric_key is set.
//
// The remaining fields are settings the rubric puts on the metric. Which ones
// mean anything depends on the metric: a judged metric reads trial_count,
// complexity and input_values, and a computed one reads the score bounds.
type rubricMetricInfo struct {
	UserMetricId    types.String         `tfsdk:"user_metric_id"`
	SystemMetricKey types.String         `tfsdk:"system_metric_key"`
	TrialCount      types.Int64          `tfsdk:"trial_count"`
	Complexity      types.String         `tfsdk:"complexity"`
	InputValues     jsontypes.Normalized `tfsdk:"input_values"`
	ScoreFloor      types.Float64        `tfsdk:"score_floor"`
	ScoreCeiling    types.Float64        `tfsdk:"score_ceiling"`
	LowerIsBetter   types.Bool           `tfsdk:"lower_is_better"`
	Weight          types.Float64        `tfsdk:"weight"`
}

// key is what matches a configured metric to one the API already holds. A
// rubric names each metric once, so the metric itself is the identity.
func (m rubricMetricInfo) key() string {
	if !m.UserMetricId.IsNull() {
		return "user:" + m.UserMetricId.ValueString()
	}
	return "system:" + m.SystemMetricKey.ValueString()
}

func rubricMetricAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"user_metric_id":    types.StringType,
		"system_metric_key": types.StringType,
		"trial_count":       types.Int64Type,
		"complexity":        types.StringType,
		"input_values":      jsontypes.NormalizedType{},
		"score_floor":       types.Float64Type,
		"score_ceiling":     types.Float64Type,
		"lower_is_better":   types.BoolType,
		"weight":            types.Float64Type,
	}
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *RubricBaseModel) MapFromGraphQL(ctx context.Context, src *pgql.RubricDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, src.Id,
		&m.Name, src.Name,
		&m.Description, src.Description,
		&m.CreatedAt, src.CreatedAt,
		&m.UpdatedAt, src.UpdatedAt,
	)

	entries := make([]rubricMetricInfo, 0, len(src.Metrics))
	for _, metric := range src.Metrics {
		entry, _, d := rubricMetricFromGraphQL(metric)
		diags.Append(d...)
		if d.HasError() {
			continue
		}
		entries = append(entries, entry)
	}

	set, d := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: rubricMetricAttrTypes()}, entries)
	diags.Append(d...)
	m.Metrics = set
	return diags
}

// rubricMetricFromGraphQL reads whichever member of the union came back.
// rubricMetricFromGraphQL reads whichever member of the union came back, with
// the id the API gave the entry. That id is what an update names the entry by;
// it is not in the schema, because it would make two configurations that say
// the same thing look different.
func rubricMetricFromGraphQL(metric pgql.RubricDetailsMetricsRubricMetric) (rubricMetricInfo, string, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch src := metric.(type) {
	case *pgql.RubricDetailsMetricsRubricUserMetric:
		return rubricMetricInfo{
			UserMetricId:    types.StringValue(src.UserMetric.Id),
			SystemMetricKey: types.StringNull(),
			TrialCount:      types.Int64Value(int64(src.UserTrialCount)),
			Complexity:      complexityValue(src.Complexity),
			InputValues:     rawJSONValue(src.InputValues),
			ScoreFloor:      types.Float64Null(),
			ScoreCeiling:    types.Float64Null(),
			LowerIsBetter:   types.BoolNull(),
			Weight:          types.Float64PointerValue(src.Weight),
		}, src.UserEntryID, diags

	case *pgql.RubricDetailsMetricsRubricSystemMetric:
		trials := types.Int64Null()
		if src.SystemTrialCount != nil {
			trials = types.Int64Value(int64(*src.SystemTrialCount))
		}
		return rubricMetricInfo{
			UserMetricId:    types.StringNull(),
			SystemMetricKey: types.StringValue(src.SystemMetric.Key),
			TrialCount:      trials,
			Complexity:      complexityValue(src.Complexity),
			InputValues:     rawJSONValue(src.InputValues),
			ScoreFloor:      types.Float64PointerValue(src.ScoreFloor),
			ScoreCeiling:    types.Float64PointerValue(src.ScoreCeiling),
			LowerIsBetter:   types.BoolPointerValue(src.LowerIsBetter),
			Weight:          types.Float64PointerValue(src.Weight),
		}, src.SystemEntryID, diags

	default:
		diags.AddError("Unexpected metric", fmt.Sprintf("the API returned a rubric metric of type %T", metric))
		return rubricMetricInfo{}, "", diags
	}
}

// rubricEntryIDs is the id the API holds for each metric of a rubric, keyed the
// way a configuration names them.
func rubricEntryIDs(src *pgql.RubricDetails, diags *diag.Diagnostics) map[string]string {
	out := make(map[string]string, len(src.Metrics))
	for _, metric := range src.Metrics {
		entry, id, d := rubricMetricFromGraphQL(metric)
		diags.Append(d...)
		if d.HasError() {
			continue
		}
		out[entry.key()] = id
	}
	return out
}

func complexityValue(c *pgql.ModelComplexity) types.String {
	if c == nil {
		return types.StringNull()
	}
	return types.StringValue(string(*c))
}

func rawJSONValue(raw *json.RawMessage) jsontypes.Normalized {
	if raw == nil {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(*raw))
}

// newRubricMetric is one configured metric as the create mutation takes it.
func newRubricMetric(entry rubricMetricInfo) pgql.NewRubricMetric {
	in := pgql.NewRubricMetric{
		SystemMetricKey: entry.SystemMetricKey.ValueStringPointer(),
		UserMetricID:    entry.UserMetricId.ValueStringPointer(),
		ScoreFloor:      entry.ScoreFloor.ValueFloat64Pointer(),
		ScoreCeiling:    entry.ScoreCeiling.ValueFloat64Pointer(),
		LowerIsBetter:   entry.LowerIsBetter.ValueBoolPointer(),
		Weight:          entry.Weight.ValueFloat64Pointer(),
		Complexity:      complexityPointer(entry.Complexity),
		InputValues:     rawJSONPointer(entry.InputValues),
	}
	if !entry.TrialCount.IsNull() && !entry.TrialCount.IsUnknown() {
		trials := int(entry.TrialCount.ValueInt64())
		in.TrialCount = &trials
	}
	return in
}

// updateRubricMetric is one configured metric as the update mutation takes it,
// against the entry id the API gave it.
func updateRubricMetric(id string, entry rubricMetricInfo) pgql.UpdateRubricMetric {
	in := pgql.UpdateRubricMetric{
		ID:            id,
		ScoreFloor:    entry.ScoreFloor.ValueFloat64Pointer(),
		ScoreCeiling:  entry.ScoreCeiling.ValueFloat64Pointer(),
		LowerIsBetter: entry.LowerIsBetter.ValueBoolPointer(),
		Weight:        entry.Weight.ValueFloat64Pointer(),
		Complexity:    complexityPointer(entry.Complexity),
		InputValues:   rawJSONPointer(entry.InputValues),
	}
	if !entry.TrialCount.IsNull() && !entry.TrialCount.IsUnknown() {
		trials := int(entry.TrialCount.ValueInt64())
		in.TrialCount = &trials
	}
	return in
}

func complexityPointer(v types.String) *pgql.ModelComplexity {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	c := pgql.ModelComplexity(v.ValueString())
	return &c
}

func rawJSONPointer(v jsontypes.Normalized) *json.RawMessage {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	raw := json.RawMessage(v.ValueString())
	return &raw
}

// rubricMetricEntries reads the configured metrics.
func rubricMetricEntries(ctx context.Context, set types.Set) ([]rubricMetricInfo, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}
	var entries []rubricMetricInfo
	diags.Append(set.ElementsAs(ctx, &entries, false)...)
	return entries, diags
}
