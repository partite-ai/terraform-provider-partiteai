package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// ClientTaskDefinitionBaseModel contains all the common fields for both resource and data source.
type ClientTaskDefinitionBaseModel struct {
	Id                types.String         `tfsdk:"id"`
	Name              types.String         `tfsdk:"name"`
	Description       types.String         `tfsdk:"description"`
	InputMessageType  jsontypes.Normalized `tfsdk:"input_message_type"`
	OutputMessageType jsontypes.Normalized `tfsdk:"output_message_type"`
	UIForms           types.Map            `tfsdk:"ui_forms"`
	CreatedAt         timetypes.RFC3339    `tfsdk:"created_at"`
	UpdatedAt         timetypes.RFC3339    `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *ClientTaskDefinitionBaseModel) MapFromGraphQL(ctx context.Context, td *pgql.ClientTaskDefinitionDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Append(setFields(
		ctx,
		&m.Id, td.Id,
		&m.Name, td.Name,
		&m.Description, td.Description,
		&m.InputMessageType, td.InputMessageType,
		&m.OutputMessageType, td.OutputMessageType,
		&m.CreatedAt, td.CreatedAt,
		&m.UpdatedAt, td.UpdatedAt,
	)...)

	forms := make(map[string]string, len(td.UiForms))
	for _, f := range td.UiForms {
		forms[string(f.Type)] = f.Definition
	}
	formsValue, d := types.MapValueFrom(ctx, types.StringType, forms)
	diags.Append(d...)
	m.UIForms = formsValue

	return diags
}

// clientTaskUIFormInputs turns the ui_forms map of a configuration into UI form
// inputs, in type order so a request is the same each time.
func clientTaskUIFormInputs(ctx context.Context, forms types.Map) ([]pgql.ClientTaskUIFormInput, diag.Diagnostics) {
	out := []pgql.ClientTaskUIFormInput{}
	if forms.IsNull() || forms.IsUnknown() {
		return out, nil
	}
	var byType map[string]string
	diags := forms.ElementsAs(ctx, &byType, false)
	if diags.HasError() {
		return nil, diags
	}
	formTypes := make([]string, 0, len(byType))
	for t := range byType {
		formTypes = append(formTypes, t)
	}
	sort.Strings(formTypes)
	for _, t := range formTypes {
		out = append(out, pgql.ClientTaskUIFormInput{Type: pgql.ClientTaskUIFormType(t), Definition: byType[t]})
	}
	return out, diags
}

// clientTaskUIFormTypes are the keys a ui_forms map can have.
func clientTaskUIFormTypes() []string {
	out := make([]string, 0, len(pgql.AllClientTaskUIFormType))
	for _, t := range pgql.AllClientTaskUIFormType {
		out = append(out, string(t))
	}
	return out
}
