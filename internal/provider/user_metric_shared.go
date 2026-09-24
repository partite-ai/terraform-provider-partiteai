package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// UserMetricBaseModel contains all the common fields for both resource and
// data source.
type UserMetricBaseModel struct {
	Id          types.String      `tfsdk:"id"`
	Name        types.String      `tfsdk:"name"`
	Description types.String      `tfsdk:"description"`
	Prompt      types.String      `tfsdk:"prompt"`
	Complexity  types.String      `tfsdk:"complexity"`
	Examples    types.List        `tfsdk:"examples"`
	InputFields types.List        `tfsdk:"input_fields"`
	CreatedAt   timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt   timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *UserMetricBaseModel) MapFromGraphQL(ctx context.Context, src *pgql.UserMetricDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, src.Id,
		&m.Name, src.Name,
		&m.Description, src.Description,
		&m.Prompt, src.Prompt,
		&m.Complexity, string(src.Complexity),
		&m.CreatedAt, src.CreatedAt,
		&m.UpdatedAt, src.UpdatedAt,
	)
	m.Examples = scoreExampleList(ctx, src.Examples, &diags)
	m.InputFields = judgeInputFieldList(ctx, src.InputFields, &diags)
	return diags
}

// scoreExampleInputs reads the configured examples into the GraphQL input.
func scoreExampleInputs(ctx context.Context, list types.List) ([]pgql.NewScoreExample, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := []pgql.NewScoreExample{}
	if list.IsNull() || list.IsUnknown() {
		return out, diags
	}

	var entries []scoreExampleInfo
	diags.Append(list.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil, diags
	}
	for _, e := range entries {
		out = append(out, pgql.NewScoreExample{
			Rating:   int(e.Rating.ValueInt64()),
			Scenario: e.Scenario.ValueString(),
			Judgment: e.Judgment.ValueString(),
		})
	}
	return out, diags
}

// judgeInputFieldInputs reads the configured input fields into the GraphQL input.
func judgeInputFieldInputs(ctx context.Context, list types.List) ([]pgql.NewJudgeInputField, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := []pgql.NewJudgeInputField{}
	if list.IsNull() || list.IsUnknown() {
		return out, diags
	}

	var entries []judgeInputFieldInfo
	diags.Append(list.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil, diags
	}
	for _, f := range entries {
		out = append(out, pgql.NewJudgeInputField{
			Name:        f.Name.ValueString(),
			Label:       f.Label.ValueString(),
			Description: f.Description.ValueString(),
			Type:        pgql.JudgeInputFieldType(f.Type.ValueString()),
			Required:    f.Required.ValueBool(),
		})
	}
	return out, diags
}
