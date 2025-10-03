package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// McpConnectionBaseModel contains all the common fields for both resource and data source
type McpConnectionBaseModel struct {
	Id                           types.String      `tfsdk:"id"`
	Name                         types.String      `tfsdk:"name"`
	HttpTransport                types.Object      `tfsdk:"http_transport"`
	StdioTransport               types.Object      `tfsdk:"stdio_transport"`
	ServerInitiatedHttpTransport types.Object      `tfsdk:"server_initiated_http_transport"`
	CreatedAt                    timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt                    timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model (copied from resource mapResponse)
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
		m.StdioTransport = types.ObjectNull(m.StdioTransport.AttributeTypes(ctx))
		m.ServerInitiatedHttpTransport = types.ObjectNull(m.ServerInitiatedHttpTransport.AttributeTypes(ctx))
	case pgql.McpTransportStdio:
		diags.Append(setFields(ctx, &m.StdioTransport, newStructMappable(mcpConnectionDetails.StdioTransport, stdioTransportStruct{}))...)
		m.HttpTransport = types.ObjectNull(m.HttpTransport.AttributeTypes(ctx))
		m.ServerInitiatedHttpTransport = types.ObjectNull(m.ServerInitiatedHttpTransport.AttributeTypes(ctx))
	case pgql.McpTransportServerInitiatedHttp:
		if diag := tfsdk.ValueFrom(ctx, struct{}{}, m.ServerInitiatedHttpTransport.Type(ctx), &m.ServerInitiatedHttpTransport); diag.HasError() {
			return diag
		}
		m.HttpTransport = types.ObjectNull(m.HttpTransport.AttributeTypes(ctx))
		m.StdioTransport = types.ObjectNull(m.StdioTransport.AttributeTypes(ctx))
	}

	return diags
}

type stdioTransportStruct struct {
	Command     string         `tfsdk:"command"`
	Arguments   []string       `tfsdk:"arguments"`
	Environment []envVarStruct `tfsdk:"environment"`
}

func (t *stdioTransportStruct) FromGQL(src *pgql.McpConnectionDetailsStdioTransportStdioMcpTransport) diag.Diagnostics {
	var diags diag.Diagnostics
	t.Command = src.Command
	t.Arguments = src.Arguments

	for _, ev := range src.Environment {
		name, value, _ := strings.Cut(ev, "=")
		t.Environment = append(t.Environment, envVarStruct{Name: name, Value: value})
	}
	return diags
}

type envVarStruct struct {
	Name  string `tfsdk:"name"`
	Value string `tfsdk:"value"`
}
