package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// MemoryStoreBaseModel is what a memory store resource and data source share.
type MemoryStoreBaseModel struct {
	Id                         types.String      `tfsdk:"id"`
	Name                       types.String      `tfsdk:"name"`
	Description                types.String      `tfsdk:"description"`
	IngestConversations        types.Bool        `tfsdk:"ingest_conversations"`
	IndexConversationMessages  types.Bool        `tfsdk:"index_conversation_messages"`
	EmbeddingProfileId         types.String      `tfsdk:"embedding_profile_id"`
	UserNameTagKey             types.String      `tfsdk:"user_name_tag_key"`
	ExtractIntervalSeconds     types.Int64       `tfsdk:"extract_interval_seconds"`
	ConsolidateIntervalSeconds types.Int64       `tfsdk:"consolidate_interval_seconds"`
	Scopes                     types.List        `tfsdk:"scopes"`
	CreatedAt                  timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt                  timetypes.RFC3339 `tfsdk:"updated_at"`
}

// memoryStoreScopeModel is one scope of a store, as a configuration writes it.
//
// The id is read back so that an update can name the scope it changes. A key,
// a kind and a value never change for a scope that already produced records,
// because those records are tagged with them.
type memoryStoreScopeModel struct {
	Id                          types.String `tfsdk:"id"`
	Description                 types.String `tfsdk:"description"`
	Kind                        types.String `tfsdk:"kind"`
	Key                         types.String `tfsdk:"key"`
	Value                       types.String `tfsdk:"value"`
	Match                       types.String `tfsdk:"match"`
	ExtractionEnabled           types.Bool   `tfsdk:"extraction_enabled"`
	ExtractionPrompt            types.String `tfsdk:"extraction_prompt"`
	ExtractionModelProfileId    types.String `tfsdk:"extraction_model_profile_id"`
	ConsolidationEnabled        types.Bool   `tfsdk:"consolidation_enabled"`
	ConsolidationPrompt         types.String `tfsdk:"consolidation_prompt"`
	ConsolidationModelProfileId types.String `tfsdk:"consolidation_model_profile_id"`
}

var memoryStoreScopeAttributeTypes = map[string]attr.Type{
	"id":                             types.StringType,
	"description":                    types.StringType,
	"kind":                           types.StringType,
	"key":                            types.StringType,
	"value":                          types.StringType,
	"match":                          types.StringType,
	"extraction_enabled":             types.BoolType,
	"extraction_prompt":              types.StringType,
	"extraction_model_profile_id":    types.StringType,
	"consolidation_enabled":          types.BoolType,
	"consolidation_prompt":           types.StringType,
	"consolidation_model_profile_id": types.StringType,
}

var memoryStoreScopeObjectType = types.ObjectType{AttrTypes: memoryStoreScopeAttributeTypes}

// MapFromGraphQL reads one memory store into the model.
func (m *MemoryStoreBaseModel) MapFromGraphQL(
	ctx context.Context,
	details pgql.MemoryStoreDetails,
) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, details.Id,
		&m.Name, details.Name,
		&m.Description, details.Description,
		&m.CreatedAt, details.CreatedAt,
		&m.UpdatedAt, details.UpdatedAt,
	)

	m.IngestConversations = types.BoolValue(details.IngestConversations)
	m.IndexConversationMessages = types.BoolValue(details.IndexConversationMessages)
	m.UserNameTagKey = types.StringPointerValue(details.UserNameTagKey)
	m.ExtractIntervalSeconds = types.Int64Value(int64(details.ExtractIntervalSeconds))
	m.ConsolidateIntervalSeconds = types.Int64Value(int64(details.ConsolidateIntervalSeconds))

	m.EmbeddingProfileId = types.StringNull()
	if details.EmbeddingProfile != nil {
		m.EmbeddingProfileId = types.StringValue(details.EmbeddingProfile.Id)
	}

	scopes := make([]attr.Value, 0, len(details.Scopes))
	for _, scope := range details.Scopes {
		value, d := types.ObjectValue(memoryStoreScopeAttributeTypes, map[string]attr.Value{
			"id":                             types.StringValue(scope.Id),
			"description":                    types.StringPointerValue(scope.Description),
			"kind":                           types.StringValue(string(scope.Kind)),
			"key":                            types.StringValue(scope.Key),
			"value":                          emptyAsNull(scope.Value),
			"match":                          emptyAsNull(scope.Match),
			"extraction_enabled":             types.BoolValue(scope.ExtractionEnabled),
			"extraction_prompt":              types.StringPointerValue(scope.ExtractionPrompt),
			"extraction_model_profile_id":    modelProfileID(scope.ExtractionModelProfile),
			"consolidation_enabled":          types.BoolValue(scope.ConsolidationEnabled),
			"consolidation_prompt":           types.StringPointerValue(scope.ConsolidationPrompt),
			"consolidation_model_profile_id": modelProfileID(scope.ConsolidationModelProfile),
		})
		diags.Append(d...)
		scopes = append(scopes, value)
	}
	list, d := types.ListValue(memoryStoreScopeObjectType, scopes)
	diags.Append(d...)
	m.Scopes = list

	return diags
}

// emptyAsNull reads a string the API always sends as null where it is empty, so
// that a configuration that leaves it out matches what comes back.
func emptyAsNull(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

// modelProfileID is the profile a scope runs on, and null where it has none.
// Extraction and consolidation each get their own generated type, so this takes
// whichever one carries the id.
func modelProfileID[T any, P interface {
	*T
	GetId() string
}](profile P) types.String {
	if profile == nil {
		return types.StringNull()
	}
	return types.StringValue(profile.GetId())
}
