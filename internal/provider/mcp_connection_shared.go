package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// McpConnectionBaseModel contains all the common fields for both resource and data source.
type McpConnectionBaseModel struct {
	Id              types.String      `tfsdk:"id"`
	Name            types.String      `tfsdk:"name"`
	HttpTransport   types.Object      `tfsdk:"http_transport"`
	TunnelTransport types.Object      `tfsdk:"tunnel_transport"`
	CreatedAt       timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt       timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model (copied from resource mapResponse).
func (m *McpConnectionBaseModel) MapFromGraphQL(ctx context.Context, mcpConnectionDetails pgql.McpConnectionDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, mcpConnectionDetails.Id,
		&m.Name, mcpConnectionDetails.Name,
		&m.CreatedAt, mcpConnectionDetails.CreatedAt,
		&m.UpdatedAt, mcpConnectionDetails.UpdatedAt,
	)

	if diags.HasError() {
		return diags
	}

	switch mcpConnectionDetails.Transport {
	case pgql.McpTransportHttp:
		// set the attribute of m.HttpTransport directly
		attrs := m.HttpTransport.Attributes()
		attrs["url"] = types.StringValue(mcpConnectionDetails.HttpTransport.Url)
		attrs["authentication_method"] = types.StringValue(string(mcpConnectionDetails.HttpTransport.AuthFlow))
		m.HttpTransport = types.ObjectValueMust(m.HttpTransport.AttributeTypes(ctx), attrs)
		m.TunnelTransport = types.ObjectNull(m.TunnelTransport.AttributeTypes(ctx))
	case pgql.McpTransportTunnel:
		if diag := tfsdk.ValueFrom(ctx, struct{}{}, m.TunnelTransport.Type(ctx), &m.TunnelTransport); diag.HasError() {
			return diag
		}
		m.HttpTransport = types.ObjectNull(m.HttpTransport.AttributeTypes(ctx))
	}

	return diags
}
