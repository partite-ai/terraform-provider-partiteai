package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

// rubricMetricInfo is one metric a rubric scores against: a metric a model
// judges or one the product computes, so exactly one of judge_metric_id and
// computed_metric_key is set.
//
// The remaining fields are settings the rubric puts on the metric. A judge
// metric takes trial_count, complexity and input_values, and a computed one
// takes the score bounds. A computed metric reads back a trial count of 1,
// which is what the API holds for it, so the schema's default matches.
type rubricMetricInfo struct {
	JudgeMetricId     types.String         `tfsdk:"judge_metric_id"`
	ComputedMetricKey types.String         `tfsdk:"computed_metric_key"`
	TrialCount        types.Int64          `tfsdk:"trial_count"`
	Complexity        types.String         `tfsdk:"complexity"`
	InputValues       jsontypes.Normalized `tfsdk:"input_values"`
	ScoreFloor        types.Float64        `tfsdk:"score_floor"`
	ScoreCeiling      types.Float64        `tfsdk:"score_ceiling"`
	LowerIsBetter     types.Bool           `tfsdk:"lower_is_better"`
	Weight            types.Float64        `tfsdk:"weight"`
}

// computedTrialCount is the trial count a computed metric reads back with.
const computedTrialCount = 1

func (m rubricMetricInfo) isJudge() bool {
	return !m.JudgeMetricId.IsNull()
}

// key is what matches a configured metric to one the API already holds. A
// rubric names each metric once, so the metric itself is the identity.
func (m rubricMetricInfo) key() string {
	if m.isJudge() {
		return "judge:" + m.JudgeMetricId.ValueString()
	}
	return "computed:" + m.ComputedMetricKey.ValueString()
}

func rubricMetricAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"judge_metric_id":     types.StringType,
		"computed_metric_key": types.StringType,
		"trial_count":         types.Int64Type,
		"complexity":          types.StringType,
		"input_values":        jsontypes.NormalizedType{},
		"score_floor":         types.Float64Type,
		"score_ceiling":       types.Float64Type,
		"lower_is_better":     types.BoolType,
		"weight":              types.Float64Type,
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

// rubricMetricFromGraphQL reads whichever member of the union came back, with
// the id the API gave the entry. That id is what an update names the entry by;
// it is not in the schema, because it would make two configurations that say
// the same thing look different.
func rubricMetricFromGraphQL(metric pgql.RubricDetailsMetricsRubricMetric) (rubricMetricInfo, string, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch src := metric.(type) {
	case *pgql.RubricDetailsMetricsRubricJudgeMetric:
		return rubricMetricInfo{
			JudgeMetricId:     types.StringValue(src.JudgeMetric.Id),
			ComputedMetricKey: types.StringNull(),
			TrialCount:        types.Int64Value(int64(src.TrialCount)),
			Complexity:        complexityValue(src.Complexity),
			InputValues:       inputValuesValue(src.InputValues),
			ScoreFloor:        types.Float64Null(),
			ScoreCeiling:      types.Float64Null(),
			LowerIsBetter:     types.BoolNull(),
			Weight:            types.Float64PointerValue(src.JudgeWeight),
		}, src.JudgeEntryID, diags

	case *pgql.RubricDetailsMetricsRubricComputedMetric:
		return rubricMetricInfo{
			JudgeMetricId:     types.StringNull(),
			ComputedMetricKey: types.StringValue(src.ComputedMetric.Key),
			TrialCount:        types.Int64Value(computedTrialCount),
			Complexity:        types.StringNull(),
			InputValues:       jsontypes.NewNormalizedNull(),
			ScoreFloor:        types.Float64PointerValue(src.ScoreFloor),
			ScoreCeiling:      types.Float64PointerValue(src.ScoreCeiling),
			LowerIsBetter:     types.BoolPointerValue(src.LowerIsBetter),
			Weight:            types.Float64PointerValue(src.ComputedWeight),
		}, src.ComputedEntryID, diags

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

// inputValuesValue reads a judge entry's input values. The API stores an
// empty object for an entry given none, so an empty object reads back as none,
// the way the configuration left it.
func inputValuesValue(raw *json.RawMessage) jsontypes.Normalized {
	if raw == nil || isEmptyJSONObject(*raw) {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(*raw))
}

func isEmptyJSONObject(raw []byte) bool {
	var object map[string]any
	return json.Unmarshal(raw, &object) == nil && object != nil && len(object) == 0
}

// inputValuesNotEmpty refuses an empty object for input values. It reads back
// as none, so it would never match the configuration; leaving the attribute
// out says the same thing.
type inputValuesNotEmpty struct{}

var _ validator.String = inputValuesNotEmpty{}

func (inputValuesNotEmpty) Description(context.Context) string {
	return "must not be an empty object; leave input_values out instead"
}

func (v inputValuesNotEmpty) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (inputValuesNotEmpty) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if isEmptyJSONObject([]byte(req.ConfigValue.ValueString())) {
		resp.Diagnostics.AddAttributeError(req.Path, "Empty input values",
			"input_values is an empty object; leave input_values out instead")
	}
}

// newRubricMetric is one configured metric as the create mutation takes it.
// The API refuses a field that does not apply to the kind of metric, even as
// null, so only the fields of the entry's kind are sent.
func newRubricMetric(entry rubricMetricInfo) pgql.NewRubricMetric {
	in := pgql.NewRubricMetric{Weight: entry.Weight.ValueFloat64Pointer()}
	if entry.isJudge() {
		in.JudgeMetricID = entry.JudgeMetricId.ValueStringPointer()
		in.TrialCount = trialCountPointer(entry.TrialCount)
		in.Complexity = complexityPointer(entry.Complexity)
		in.InputValues = rawJSONPointer(entry.InputValues)
		return in
	}
	in.ComputedMetricKey = entry.ComputedMetricKey.ValueStringPointer()
	in.ScoreFloor = entry.ScoreFloor.ValueFloat64Pointer()
	in.ScoreCeiling = entry.ScoreCeiling.ValueFloat64Pointer()
	in.LowerIsBetter = entry.LowerIsBetter.ValueBoolPointer()
	return in
}

// updateRubricMetric is one configured metric as the update mutation takes it,
// against the entry id the API gave it, with only the fields of its kind.
func updateRubricMetric(id string, entry rubricMetricInfo) pgql.UpdateRubricMetric {
	in := pgql.UpdateRubricMetric{ID: id, Weight: entry.Weight.ValueFloat64Pointer()}
	if entry.isJudge() {
		in.TrialCount = trialCountPointer(entry.TrialCount)
		in.Complexity = complexityPointer(entry.Complexity)
		in.InputValues = rawJSONPointer(entry.InputValues)
		return in
	}
	in.ScoreFloor = entry.ScoreFloor.ValueFloat64Pointer()
	in.ScoreCeiling = entry.ScoreCeiling.ValueFloat64Pointer()
	in.LowerIsBetter = entry.LowerIsBetter.ValueBoolPointer()
	return in
}

func trialCountPointer(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	trials := int(v.ValueInt64())
	return &trials
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
