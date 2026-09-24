package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// MemoryInteractionProfileBaseModel is what a memory interaction profile
// resource and data source share. A profile decides what memory goes in an
// agent's prompt and which memory tools it can call.
type MemoryInteractionProfileBaseModel struct {
	Id                  types.String      `tfsdk:"id"`
	Name                types.String      `tfsdk:"name"`
	Description         types.String      `tfsdk:"description"`
	AllowExpansion      types.Bool        `tfsdk:"allow_expansion"`
	ConversationHistory types.Object      `tfsdk:"conversation_history"`
	ContextInjections   types.List        `tfsdk:"context_injections"`
	QueryTools          types.List        `tfsdk:"query_tools"`
	WriteTools          types.List        `tfsdk:"write_tools"`
	CreatedAt           timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt           timetypes.RFC3339 `tfsdk:"updated_at"`
}

// The shapes a configuration writes, and the attribute types that match them.

type memoryQuerySourceModel struct {
	Kind types.String `tfsdk:"kind"`
	Text types.String `tfsdk:"text"`
}

type memorySelectionTagModel struct {
	Source        types.String `tfsdk:"source"`
	TagKey        types.String `tfsdk:"tag_key"`
	TagValue      types.String `tfsdk:"tag_value"`
	RequestTagKey types.String `tfsdk:"request_tag_key"`
}

type memorySelectionModel struct {
	Kind types.String `tfsdk:"kind"`
	Tags types.List   `tfsdk:"tags"`
}

type memoryConversationHistoryModel struct {
	TokenBudget types.Int64  `tfsdk:"token_budget"`
	Selection   types.String `tfsdk:"selection"`
	Query       types.Object `tfsdk:"query"`
}

type memoryContextInjectionModel struct {
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	TokenBudget types.Int64  `tfsdk:"token_budget"`
	RecordTypes types.List   `tfsdk:"record_types"`
	Selection   types.Object `tfsdk:"selection"`
	Query       types.Object `tfsdk:"query"`
}

type memoryEffortLevelModel struct {
	Level       types.String `tfsdk:"level"`
	TokenBudget types.Int64  `tfsdk:"token_budget"`
}

type memoryQueryToolModel struct {
	ToolName     types.String `tfsdk:"tool_name"`
	Description  types.String `tfsdk:"description"`
	RecordTypes  types.List   `tfsdk:"record_types"`
	Selection    types.Object `tfsdk:"selection"`
	TokenBudget  types.Int64  `tfsdk:"token_budget"`
	EffortLevels types.List   `tfsdk:"effort_levels"`
}

type memoryTagMappingModel struct {
	Kind     types.String `tfsdk:"kind"`
	Keys     types.List   `tfsdk:"keys"`
	TagKey   types.String `tfsdk:"tag_key"`
	TagValue types.String `tfsdk:"tag_value"`
}

type memoryWriteToolModel struct {
	ToolName    types.String `tfsdk:"tool_name"`
	Description types.String `tfsdk:"description"`
	TagMappings types.List   `tfsdk:"tag_mappings"`
}

var (
	memoryQuerySourceTypes = map[string]attr.Type{
		"kind": types.StringType,
		"text": types.StringType,
	}
	memoryQuerySourceObject = types.ObjectType{AttrTypes: memoryQuerySourceTypes}

	memorySelectionTagTypes = map[string]attr.Type{
		"source":          types.StringType,
		"tag_key":         types.StringType,
		"tag_value":       types.StringType,
		"request_tag_key": types.StringType,
	}
	memorySelectionTagObject = types.ObjectType{AttrTypes: memorySelectionTagTypes}

	memorySelectionTypes = map[string]attr.Type{
		"kind": types.StringType,
		"tags": types.ListType{ElemType: memorySelectionTagObject},
	}
	memorySelectionObject = types.ObjectType{AttrTypes: memorySelectionTypes}

	memoryConversationHistoryTypes = map[string]attr.Type{
		"token_budget": types.Int64Type,
		"selection":    types.StringType,
		"query":        memoryQuerySourceObject,
	}

	memoryContextInjectionTypes = map[string]attr.Type{
		"name":         types.StringType,
		"description":  types.StringType,
		"token_budget": types.Int64Type,
		"record_types": types.ListType{ElemType: types.StringType},
		"selection":    memorySelectionObject,
		"query":        memoryQuerySourceObject,
	}
	memoryContextInjectionObject = types.ObjectType{AttrTypes: memoryContextInjectionTypes}

	memoryEffortLevelTypes = map[string]attr.Type{
		"level":        types.StringType,
		"token_budget": types.Int64Type,
	}
	memoryEffortLevelObject = types.ObjectType{AttrTypes: memoryEffortLevelTypes}

	memoryQueryToolTypes = map[string]attr.Type{
		"tool_name":     types.StringType,
		"description":   types.StringType,
		"record_types":  types.ListType{ElemType: types.StringType},
		"selection":     memorySelectionObject,
		"token_budget":  types.Int64Type,
		"effort_levels": types.ListType{ElemType: memoryEffortLevelObject},
	}
	memoryQueryToolObject = types.ObjectType{AttrTypes: memoryQueryToolTypes}

	memoryTagMappingTypes = map[string]attr.Type{
		"kind":      types.StringType,
		"keys":      types.ListType{ElemType: types.StringType},
		"tag_key":   types.StringType,
		"tag_value": types.StringType,
	}
	memoryTagMappingObject = types.ObjectType{AttrTypes: memoryTagMappingTypes}

	memoryWriteToolTypes = map[string]attr.Type{
		"tool_name":    types.StringType,
		"description":  types.StringType,
		"tag_mappings": types.ListType{ElemType: memoryTagMappingObject},
	}
	memoryWriteToolObject = types.ObjectType{AttrTypes: memoryWriteToolTypes}
)

// MapFromGraphQL reads one memory interaction profile into the model.
func (m *MemoryInteractionProfileBaseModel) MapFromGraphQL(
	ctx context.Context,
	details pgql.MemoryInteractionProfileDetails,
) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, details.Id,
		&m.Name, details.Name,
		&m.Description, details.Description,
		&m.CreatedAt, details.CreatedAt,
		&m.UpdatedAt, details.UpdatedAt,
	)
	m.AllowExpansion = types.BoolValue(details.AllowExpansion)

	m.ConversationHistory = types.ObjectNull(memoryConversationHistoryTypes)
	if history := details.ConversationHistory; history != nil {
		query := types.ObjectNull(memoryQuerySourceTypes)
		if history.Query != nil {
			query = querySourceValue(history.Query.Kind, history.Query.Text, &diags)
		}
		value, d := types.ObjectValue(memoryConversationHistoryTypes, map[string]attr.Value{
			"token_budget": types.Int64Value(int64(history.TokenBudget)),
			"selection":    types.StringValue(string(history.Selection)),
			"query":        query,
		})
		diags.Append(d...)
		m.ConversationHistory = value
	}

	injections := make([]attr.Value, 0, len(details.ContextInjections))
	for _, injection := range details.ContextInjections {
		value, d := types.ObjectValue(memoryContextInjectionTypes, map[string]attr.Value{
			"name":         types.StringValue(injection.Name),
			"description":  types.StringPointerValue(injection.Description),
			"token_budget": types.Int64Value(int64(injection.TokenBudget)),
			"record_types": recordTypeList(injection.RecordTypes, &diags),
			"selection":    selectionValue(injection.Selection, &diags),
			"query":        querySourceValue(injection.Query.Kind, injection.Query.Text, &diags),
		})
		diags.Append(d...)
		injections = append(injections, value)
	}
	m.ContextInjections = listValue(memoryContextInjectionObject, injections, &diags)

	queryTools := make([]attr.Value, 0, len(details.QueryTools))
	for _, tool := range details.QueryTools {
		levels := make([]attr.Value, 0, len(tool.EffortLevels))
		for _, level := range tool.EffortLevels {
			value, d := types.ObjectValue(memoryEffortLevelTypes, map[string]attr.Value{
				"level":        types.StringValue(level.Level),
				"token_budget": types.Int64Value(int64(level.TokenBudget)),
			})
			diags.Append(d...)
			levels = append(levels, value)
		}
		budget := types.Int64Null()
		if tool.TokenBudget != nil {
			budget = types.Int64Value(int64(*tool.TokenBudget))
		}
		value, d := types.ObjectValue(memoryQueryToolTypes, map[string]attr.Value{
			"tool_name":     types.StringValue(tool.ToolName),
			"description":   types.StringValue(tool.Description),
			"record_types":  recordTypeList(tool.RecordTypes, &diags),
			"selection":     selectionValue(tool.Selection, &diags),
			"token_budget":  budget,
			"effort_levels": listValue(memoryEffortLevelObject, levels, &diags),
		})
		diags.Append(d...)
		queryTools = append(queryTools, value)
	}
	m.QueryTools = listValue(memoryQueryToolObject, queryTools, &diags)

	writeTools := make([]attr.Value, 0, len(details.WriteTools))
	for _, tool := range details.WriteTools {
		mappings := make([]attr.Value, 0, len(tool.TagMappings))
		for _, mapping := range tool.TagMappings {
			keys := make([]attr.Value, 0, len(mapping.Keys))
			for _, key := range mapping.Keys {
				keys = append(keys, types.StringValue(key))
			}
			value, d := types.ObjectValue(memoryTagMappingTypes, map[string]attr.Value{
				"kind":      types.StringValue(string(mapping.Kind)),
				"keys":      listValue(types.StringType, keys, &diags),
				"tag_key":   types.StringPointerValue(mapping.TagKey),
				"tag_value": types.StringPointerValue(mapping.TagValue),
			})
			diags.Append(d...)
			mappings = append(mappings, value)
		}
		value, d := types.ObjectValue(memoryWriteToolTypes, map[string]attr.Value{
			"tool_name":    types.StringValue(tool.ToolName),
			"description":  types.StringValue(tool.Description),
			"tag_mappings": listValue(memoryTagMappingObject, mappings, &diags),
		})
		diags.Append(d...)
		writeTools = append(writeTools, value)
	}
	m.WriteTools = listValue(memoryWriteToolObject, writeTools, &diags)

	return diags
}

// listValue is a list of values already built, with the diagnostics collected.
// Every list it builds is Optional without a default, so nothing maps to a null
// list. An empty list there would differ from the null the configuration left
// behind, which Terraform rejects as an inconsistent result after apply.
func listValue(of attr.Type, values []attr.Value, diags *diag.Diagnostics) types.List {
	if len(values) == 0 {
		return types.ListNull(of)
	}
	list, d := types.ListValue(of, values)
	diags.Append(d...)
	return list
}

// recordTypeList are the kinds of record an injection or a query tool reads.
func recordTypeList[T ~string](kinds []T, diags *diag.Diagnostics) types.List {
	values := make([]attr.Value, 0, len(kinds))
	for _, kind := range kinds {
		values = append(values, types.StringValue(string(kind)))
	}
	return listValue(types.StringType, values, diags)
}

// querySourceValue is the text an injection or a history searches for.
func querySourceValue[T ~string](kind T, text *string, diags *diag.Diagnostics) types.Object {
	value, d := types.ObjectValue(memoryQuerySourceTypes, map[string]attr.Value{
		"kind": types.StringValue(string(kind)),
		"text": types.StringPointerValue(text),
	})
	diags.Append(d...)
	return value
}

// selectionValue is what narrows the memory one injection or tool reads. An
// injection and a query tool each get their own generated type for the same
// fragment, so this takes either one.
func selectionValue[T any, P interface {
	*T
	GetKind() pgql.MemorySelectionKind
	GetTags() []pgql.MemorySelectionDetailsTagsMemorySelectionTag
}](from T, diags *diag.Diagnostics) types.Object {
	selection := P(&from)
	selectionTags := selection.GetTags()
	tags := make([]attr.Value, 0, len(selectionTags))
	for _, tag := range selectionTags {
		value, d := types.ObjectValue(memorySelectionTagTypes, map[string]attr.Value{
			"source":          types.StringValue(string(tag.Source)),
			"tag_key":         types.StringValue(tag.TagKey),
			"tag_value":       types.StringPointerValue(tag.TagValue),
			"request_tag_key": types.StringPointerValue(tag.RequestTagKey),
		})
		diags.Append(d...)
		tags = append(tags, value)
	}
	value, d := types.ObjectValue(memorySelectionTypes, map[string]attr.Value{
		"kind": types.StringValue(string(selection.GetKind())),
		"tags": listValue(memorySelectionTagObject, tags, diags),
	})
	diags.Append(d...)
	return value
}
