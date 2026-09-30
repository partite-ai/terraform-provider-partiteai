package provider

import (
	"context"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// mcpConnectionSubscriptionCriterionType is one entry of the criteria set.
var mcpConnectionSubscriptionCriterionType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"name":  types.StringType,
		"value": types.StringType,
	},
}

// cutImportID splits a "<parent>:<child>" import identifier.
func cutImportID(id string) (parent, child string, ok bool) {
	parent, child, ok = strings.Cut(id, ":")
	if !ok || parent == "" || child == "" {
		return "", "", false
	}
	return parent, child, true
}

// readMcpConnectionSubscription finds one subscription of a connection.
//
// Subscriptions have no top-level query, so they are read through the connection
// that owns them. A nil result means the subscription is gone.
func readMcpConnectionSubscription(ctx context.Context, client graphql.Client, connectionID, subscriptionID string) (*pgql.McpConnectionSubscriptionWithKeys, diag.Diagnostics) {
	var diags diag.Diagnostics

	resp, err := pgql.GetMcpConnectionSubscriptions(ctx, client, connectionID)
	if err != nil {
		handleAPIErrorWithDetails(&diags, err, "read", "mcp_connection_subscription", subscriptionID)
		return nil, diags
	}
	if resp.McpConnection.TunnelTransport == nil {
		return nil, diags
	}

	for _, sub := range resp.McpConnection.TunnelTransport.Subscriptions {
		if sub.Id == subscriptionID {
			return &sub, diags
		}
	}
	return nil, diags
}

// mapMcpConnectionSubscription writes a subscription onto a model's fields, shared
// by the resource and the data source.
func mapMcpConnectionSubscription(
	ctx context.Context,
	id *types.String,
	name *types.String,
	criteria *types.Set,
	createdAt *timetypes.RFC3339,
	updatedAt *timetypes.RFC3339,
	details pgql.McpConnectionSubscriptionDetails,
) diag.Diagnostics {
	diags := setFields(
		ctx,
		id, details.Id,
		name, details.Name,
		createdAt, details.CreatedAt,
		updatedAt, details.UpdatedAt,
	)
	if diags.HasError() {
		return diags
	}

	// An empty criteria set and an absent one mean the same thing to the API — the
	// subscription accepts everything — so a subscription with none reads back null
	// rather than an empty set, which is what an omitted attribute looks like.
	if len(details.Criteria) == 0 {
		*criteria = types.SetNull(mcpConnectionSubscriptionCriterionType)
		return diags
	}

	elements := make([]attr.Value, 0, len(details.Criteria))
	for _, c := range details.Criteria {
		elements = append(elements, types.ObjectValueMust(
			mcpConnectionSubscriptionCriterionType.AttrTypes,
			map[string]attr.Value{
				"name":  types.StringValue(c.Name),
				"value": types.StringValue(c.Value),
			},
		))
	}

	set, setDiags := types.SetValue(mcpConnectionSubscriptionCriterionType, elements)
	diags.Append(setDiags...)
	*criteria = set
	return diags
}

// optionalRFC3339 renders a nullable API timestamp.
func optionalRFC3339(value *string) timetypes.RFC3339 {
	if value == nil {
		return timetypes.NewRFC3339Null()
	}
	parsed, diags := timetypes.NewRFC3339Value(*value)
	if diags.HasError() {
		return timetypes.NewRFC3339Null()
	}
	return parsed
}

// readMcpConnectionTunnelKey finds one key of a subscription. A nil result means
// the key is gone.
//
// Keys hang off subscriptions, which hang off connections, and neither has a query
// of its own, so the connection has to be named rather than discovered.
func readMcpConnectionTunnelKey(ctx context.Context, client graphql.Client, connectionID, subscriptionID, keyID string) (*pgql.McpConnectionTunnelKeyDetails, diag.Diagnostics) {
	var diags diag.Diagnostics

	resp, err := pgql.GetMcpConnectionSubscriptions(ctx, client, connectionID)
	if err != nil {
		handleAPIErrorWithDetails(&diags, err, "read", "mcp_connection_tunnel_key", keyID)
		return nil, diags
	}
	if resp.McpConnection.TunnelTransport == nil {
		return nil, diags
	}

	for _, sub := range resp.McpConnection.TunnelTransport.Subscriptions {
		if sub.Id != subscriptionID {
			continue
		}
		for _, key := range sub.TunnelKeys {
			if key.Id == keyID {
				return &key, diags
			}
		}
		return nil, diags
	}
	return nil, diags
}

// mcpConnectionTunnelKeyType is one entry of a subscription's tunnel_keys. The
// credential is not among them: it is readable only when a key is created.
var mcpConnectionTunnelKeyType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":         types.StringType,
		"label":      types.StringType,
		"attached":   types.BoolType,
		"expires_at": timetypes.RFC3339Type{},
		"revoked_at": timetypes.RFC3339Type{},
		"created_at": timetypes.RFC3339Type{},
	},
}

// tunnelKeyList renders a subscription's keys.
func tunnelKeyList(_ context.Context, keys []pgql.McpConnectionTunnelKeyDetails) (types.List, diag.Diagnostics) {
	elements := make([]attr.Value, 0, len(keys))
	for _, key := range keys {
		created, diags := timetypes.NewRFC3339Value(key.CreatedAt)
		if diags.HasError() {
			created = timetypes.NewRFC3339Null()
		}
		elements = append(elements, types.ObjectValueMust(
			mcpConnectionTunnelKeyType.AttrTypes,
			map[string]attr.Value{
				"id":         types.StringValue(key.Id),
				"label":      types.StringValue(key.Label),
				"attached":   types.BoolValue(key.Attached),
				"expires_at": optionalRFC3339(key.ExpiresAt),
				"revoked_at": optionalRFC3339(key.RevokedAt),
				"created_at": created,
			},
		))
	}
	return types.ListValue(mcpConnectionTunnelKeyType, elements)
}
