package provider

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/attr"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// WebhookDestinationBaseModel contains all the common fields for both resource and data source.
type WebhookDestinationBaseModel struct {
	Id               types.String      `tfsdk:"id"`
	Name             types.String      `tfsdk:"name"`
	Description      types.String      `tfsdk:"description"`
	ExternalId       types.String      `tfsdk:"external_id"`
	InvokeUrl        types.String      `tfsdk:"invoke_url"`
	MeshId           types.String      `tfsdk:"mesh_id"`
	MeshVersionLabel types.String      `tfsdk:"mesh_version_label"`
	InputName        types.String      `tfsdk:"input_name"`
	CreatedAt        timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt        timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *WebhookDestinationBaseModel) MapFromGraphQL(ctx context.Context, webhookDetails *pgql.WebhookDestinationDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Append(setFields(
		ctx,
		&m.Id, webhookDetails.Id,
		&m.Name, webhookDetails.Name,
		&m.Description, webhookDetails.Description,
		&m.ExternalId, webhookDetails.ExternalID,
		&m.MeshId, webhookDetails.Mesh.Id,
		&m.MeshVersionLabel, webhookDetails.MeshVersionLabel,
		&m.InputName, webhookDetails.InputName,
		&m.CreatedAt, webhookDetails.CreatedAt,
		&m.UpdatedAt, webhookDetails.UpdatedAt,
	)...)
	return diags
}

// webhookHandlerModel is a webhook destination's handler, as the resource
// configures it.
type webhookHandlerModel struct {
	Type           types.String `tfsdk:"type"`
	SourcePath     types.String `tfsdk:"source_path"`
	SourceHash     types.String `tfsdk:"source_hash"`
	Config         types.Map    `tfsdk:"config"`
	SecretsWO      types.Map    `tfsdk:"secrets_wo"`
	SecretsVersion types.Int64  `tfsdk:"secrets_version"`
	SecretNames    types.Set    `tfsdk:"secret_names"`
}

func webhookHandlerAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":            types.StringType,
		"source_path":     types.StringType,
		"source_hash":     types.StringType,
		"config":          types.MapType{ElemType: types.StringType},
		"secrets_wo":      types.MapType{ElemType: types.StringType},
		"secrets_version": types.Int64Type,
		"secret_names":    types.SetType{ElemType: types.StringType},
	}
}

// webhookHandlerDataModel is a webhook destination's handler, as the data
// source reads it.
type webhookHandlerDataModel struct {
	Type        types.String `tfsdk:"type"`
	SourceHash  types.String `tfsdk:"source_hash"`
	Config      types.Map    `tfsdk:"config"`
	SecretNames types.Set    `tfsdk:"secret_names"`
}

func webhookHandlerDataAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":         types.StringType,
		"source_hash":  types.StringType,
		"config":       types.MapType{ElemType: types.StringType},
		"secret_names": types.SetType{ElemType: types.StringType},
	}
}

// webhookHandlerConfig reads a handler's config from the server. An empty
// config is null when keepNull is set, so a configuration that leaves config
// out matches it.
func webhookHandlerConfig(ctx context.Context, raw *json.RawMessage, keepNull bool) (types.Map, diag.Diagnostics) {
	config := map[string]string{}
	if raw != nil && string(*raw) != "null" {
		if err := json.Unmarshal(*raw, &config); err != nil {
			var diags diag.Diagnostics
			diags.AddError("Cannot read the webhook handler's config", err.Error())
			return types.MapNull(types.StringType), diags
		}
	}
	if len(config) == 0 && keepNull {
		return types.MapNull(types.StringType), nil
	}
	return types.MapValueFrom(ctx, types.StringType, config)
}

// webhookHandlerSecretChanges is what an update sends for a handler's secrets:
// configured values for names the server doesn't hold, or for every name when
// resendAll is set, and nil for each name the server holds that isn't
// configured. It is empty when nothing changes.
func webhookHandlerSecretChanges(configured map[string]string, serverNames []string, resendAll bool) map[string]*string {
	changes := map[string]*string{}
	for name, value := range configured {
		if resendAll || !slices.Contains(serverNames, name) {
			changes[name] = &value
		}
	}
	for _, name := range serverNames {
		if _, ok := configured[name]; !ok {
			changes[name] = nil
		}
	}
	return changes
}
