package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &McpConnectionResource{}
var _ resource.ResourceWithImportState = &McpConnectionResource{}

func NewMcpConnectionResource() resource.Resource {
	return &McpConnectionResource{
		BaseResource: NewBaseResource("mcp_connection"),
	}
}

// McpConnectionResource defines the resource implementation.
type McpConnectionResource struct {
	BaseResource
}

// McpConnectionResourceModel describes the resource data model.
type McpConnectionResourceModel struct {
	Id                           types.String      `tfsdk:"id"`
	Name                         types.String      `tfsdk:"name"`
	HttpTransport                types.Object      `tfsdk:"http_transport"`
	StdioTransport               types.Object      `tfsdk:"stdio_transport"`
	ServerInitiatedHttpTransport types.Object      `tfsdk:"server_initiated_http_transport"`
	CreatedAt                    timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt                    timetypes.RFC3339 `tfsdk:"updated_at"`
}

var envVarType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "The name of the environment variable.",
			Required:            true,
		},
		"value": schema.StringAttribute{
			MarkdownDescription: "The value of the environment variable.",
			Required:            true,
			Sensitive:           true,
		},
	},
}

var oauthClientCredentialsType = schema.SingleNestedAttribute{
	MarkdownDescription: "OAuth client credentials for authentication.",
	Optional:            true,
	Attributes: map[string]schema.Attribute{
		"client_id": schema.StringAttribute{
			MarkdownDescription: "The client ID for OAuth authentication.",
			Required:            true,
		},
		"client_secret": schema.StringAttribute{
			MarkdownDescription: "The client secret for OAuth authentication.",
			Required:            true,
			Sensitive:           true,
		},
		"scopes": schema.ListAttribute{
			MarkdownDescription: "The scopes for OAuth authentication.",
			ElementType:         types.StringType,
			Optional:            true,
			Computed:            true,
			Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, nil)),
		},
	},
}

var basicAuthCredentialsType = schema.SingleNestedAttribute{
	MarkdownDescription: "Basic authentication credentials.",
	Optional:            true,
	Attributes: map[string]schema.Attribute{
		"username": schema.StringAttribute{
			MarkdownDescription: "The username for basic authentication.",
			Required:            true,
		},
		"password": schema.StringAttribute{
			MarkdownDescription: "The password for basic authentication.",
			Required:            true,
			Sensitive:           true,
		},
	},
}

var headerAuthCredentialsType = schema.SingleNestedAttribute{
	MarkdownDescription: "Header authentication credentials.",
	Optional:            true,
	Attributes: map[string]schema.Attribute{
		"header_name": schema.StringAttribute{
			MarkdownDescription: "The name of the header for authentication.",
			Required:            true,
		},
		"header_value": schema.StringAttribute{
			MarkdownDescription: "The value of the header for authentication.",
			Required:            true,
			Sensitive:           true,
		},
	},
}

func (r *McpConnectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_connection"
}

func (r *McpConnectionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "MCP connection resource",
		Attributes: map[string]schema.Attribute{
			"id":         commonSchemas.id,
			"name":       commonSchemas.name,
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
			"http_transport": schema.SingleNestedAttribute{
				MarkdownDescription: "HTTP transport configuration for the MCP connection.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "The URL of the HTTP endpoint.",
						Required:            true,
					},
					"oauth_client_credentials": oauthClientCredentialsType,
					"basic_credentials":        basicAuthCredentialsType,
					"header_credentials":       headerAuthCredentialsType,
				},
			},
			"stdio_transport": schema.SingleNestedAttribute{
				MarkdownDescription: "Standard I/O transport configuration for the MCP connection.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"command": schema.StringAttribute{
						MarkdownDescription: "The command to execute for the standard I/O transport.",
						Required:            true,
					},
					"arguments": schema.ListAttribute{
						MarkdownDescription: "The arguments to pass to the command.",
						ElementType:         types.StringType,
						Optional:            true,
						Computed:            true,
						Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, nil)),
					},
					"environment": schema.ListNestedAttribute{
						MarkdownDescription: "Environment variables for the command.",
						Optional:            true,
						Computed:            true,
						NestedObject:        envVarType,
						Default:             listdefault.StaticValue(types.ListValueMust(envVarType.Type(), nil)),
					},
				},
			},
			"server_initiated_http_transport": schema.SingleNestedAttribute{
				MarkdownDescription: "Server-initiated HTTP transport configuration for the MCP connection.",
				Optional:            true,
				Attributes:          map[string]schema.Attribute{},
			},
		},
	}
}

func (r *McpConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data McpConnectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "mcp_connection", "")

	var createReq pgql.NewMcpConnection
	createReq.Name = data.Name.ValueString()
	if diag := fillMcpConnectionUpdateMessage(
		ctx, data,
		&createReq.Transport,
		&createReq.HttpTransport, &createReq.OauthClientCredentials, &createReq.BasicCredentials, &createReq.HeaderCredentials,
		&createReq.StdioTransport,
	); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	createResp, err := pgql.CreateMcpConnection(ctx, r.GetClient(), createReq)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create MCP connection")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, createResp.CreateMcpConnection)...)

	logResourceOperation(ctx, "created", "mcp_connection", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "mcp_connection", data.Id.ValueString())

	mcpConnectionResp, err := pgql.GetMcpConnection(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "mcp_connection", data.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, *mcpConnectionResp.McpConnection)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData McpConnectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "mcp_connection", planData.Id.ValueString())

	var update pgql.UpdateMcpConnection
	if !planData.Name.Equal(stateData.Name) {
		update.Name = planData.Name.ValueStringPointer()
	}

	if !planData.HttpTransport.Equal(stateData.HttpTransport) ||
		!planData.StdioTransport.Equal(stateData.StdioTransport) ||
		!planData.ServerInitiatedHttpTransport.Equal(stateData.ServerInitiatedHttpTransport) {
		var t pgql.McpTransport
		fillMcpConnectionUpdateMessage(
			ctx, planData, &t,
			&update.HttpTransport, &update.OauthClientCredentials, &update.BasicCredentials, &update.HeaderCredentials,
			&update.StdioTransport,
		)
		update.Transport = &t
	}

	updateResp, err := pgql.ModifyMcpConnection(ctx, r.GetClient(), planData.Id.ValueString(), update)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "mcp_connection", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, updateResp.UpdateMcpConnection)...)

	logResourceOperation(ctx, "updated", "mcp_connection", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *McpConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "mcp_connection", data.Id.ValueString())

	_, err := pgql.DeleteMcpConnection(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "mcp_connection", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "mcp_connection", data.Id.ValueString())
}

func (r *McpConnectionResource) mapResponse(ctx context.Context, data *McpConnectionResourceModel, mcpConnectionDetails pgql.McpConnectionDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&data.Id, mcpConnectionDetails.Id,
		&data.Name, mcpConnectionDetails.Name,
		&data.CreatedAt, mcpConnectionDetails.CreatedAt,
		&data.UpdatedAt, mcpConnectionDetails.UpdatedAt,
	)

	if diags.HasError() {
		return diags
	}

	switch mcpConnectionDetails.Transport {
	case pgql.McpTransportHttp:
		var t httpTransportStruct
		diags.Append(tfsdk.ValueAs(ctx, data.HttpTransport, &t)...)
		t.Url = mcpConnectionDetails.HttpTransport.Url
		switch mcpConnectionDetails.HttpTransport.AuthFlow {
		case pgql.McpAuthFlowOauth2AuthorizationCode, pgql.McpAuthFlowNone:
			t.OauthClientCredentials = nil
			t.BasicAuthCredentials = nil
			t.HeaderCredentials = nil
		case pgql.McpAuthFlowOauth2ClientCredentials:
			if t.OauthClientCredentials == nil {
				t.OauthClientCredentials = &oauthClientCredentialsStruct{
					ClientId:     "<unknown>",
					ClientSecret: "<unknown>",
				}
			}
			t.BasicAuthCredentials = nil
			t.HeaderCredentials = nil
		case pgql.McpAuthFlowBasic:
			if t.BasicAuthCredentials == nil {
				t.BasicAuthCredentials = &basicAuthCredentialsStruct{
					Username: "<unknown>",
					Password: "<unknown>",
				}
			}
			t.OauthClientCredentials = nil
			t.HeaderCredentials = nil
		case pgql.McpAuthFlowHeader:
			if t.HeaderCredentials == nil {
				t.HeaderCredentials = &headerAuthCredentialsStruct{
					HeaderName:  "<unknown>",
					HeaderValue: "<unknown>",
				}
			}
			t.OauthClientCredentials = nil
			t.BasicAuthCredentials = nil
		}
		diags.Append(setFields(ctx, &data.HttpTransport, t)...)
		data.StdioTransport = types.ObjectNull(data.StdioTransport.AttributeTypes(ctx))
		data.ServerInitiatedHttpTransport = types.ObjectNull(data.ServerInitiatedHttpTransport.AttributeTypes(ctx))
	case pgql.McpTransportStdio:
		diags.Append(setFields(ctx, &data.StdioTransport, newStructMappable(mcpConnectionDetails.StdioTransport, stdioTransportStruct{}))...)
		data.HttpTransport = types.ObjectNull(data.HttpTransport.AttributeTypes(ctx))
		data.ServerInitiatedHttpTransport = types.ObjectNull(data.ServerInitiatedHttpTransport.AttributeTypes(ctx))
	case pgql.McpTransportServerInitiatedHttp:
		if diag := tfsdk.ValueFrom(ctx, struct{}{}, data.ServerInitiatedHttpTransport.Type(ctx), &data.ServerInitiatedHttpTransport); diag.HasError() {
			return diag
		}
		data.HttpTransport = types.ObjectNull(data.HttpTransport.AttributeTypes(ctx))
		data.StdioTransport = types.ObjectNull(data.StdioTransport.AttributeTypes(ctx))
	}

	return diags
}

func fillMcpConnectionUpdateMessage(
	ctx context.Context,
	data McpConnectionResourceModel,
	pTransport *pgql.McpTransport,
	httpTransport **pgql.McpConnectionHttpTransportSettings,
	oauthClientCreds **pgql.McpConnectionOauthClientCredentials,
	basicCreds **pgql.McpConnectionBasicCredentials,
	headerCreds **pgql.McpConnectionHeaderCredentials,
	stdioTransport **pgql.McpConnectionStdioTransportSettings,
) diag.Diagnostics {
	switch {
	case !data.HttpTransport.IsNull() && !data.HttpTransport.IsUnknown():
		*pTransport = pgql.McpTransportHttp
		ht := &pgql.McpConnectionHttpTransportSettings{}
		if diag := tfsdk.ValueAs(ctx, data.HttpTransport.Attributes()["url"], &ht.Url); diag.HasError() {
			return diag
		}

		if v, ok := data.HttpTransport.Attributes()["oauth_client_credentials"]; ok && !v.IsNull() && !v.IsUnknown() {
			ht.AuthFlow = pgql.McpAuthFlowOauth2ClientCredentials
			var creds struct {
				ClientId     string   `tfsdk:"client_id"`
				ClientSecret string   `tfsdk:"client_secret"`
				Scopes       []string `tfsdk:"scopes"`
			}
			if diag := tfsdk.ValueAs(ctx, v, &creds); diag.HasError() {
				return diag
			}
			*oauthClientCreds = &pgql.McpConnectionOauthClientCredentials{
				ClientID:     creds.ClientId,
				ClientSecret: creds.ClientSecret,
				Scopes:       creds.Scopes,
			}
		} else if v, ok := data.HttpTransport.Attributes()["basic_credentials"]; ok && !v.IsNull() && !v.IsUnknown() {
			ht.AuthFlow = pgql.McpAuthFlowBasic
			var creds struct {
				Username string `tfsdk:"username"`
				Password string `tfsdk:"password"`
			}
			if diag := tfsdk.ValueAs(ctx, v, &creds); diag.HasError() {
				return diag
			}
			*basicCreds = &pgql.McpConnectionBasicCredentials{
				Username: creds.Username,
				Password: creds.Password,
			}
		} else if v, ok := data.HttpTransport.Attributes()["header_credentials"]; ok && !v.IsNull() && !v.IsUnknown() {
			ht.AuthFlow = pgql.McpAuthFlowHeader
			var creds struct {
				HeaderName  string `tfsdk:"header_name"`
				HeaderValue string `tfsdk:"header_value"`
			}
			if diag := tfsdk.ValueAs(ctx, v, &creds); diag.HasError() {
				return diag
			}
			*headerCreds = &pgql.McpConnectionHeaderCredentials{
				Name:  creds.HeaderName,
				Value: creds.HeaderValue,
			}
		} else {
			ht.AuthFlow = pgql.McpAuthFlowNone
		}
		*httpTransport = ht
	case !data.StdioTransport.IsNull() && !data.StdioTransport.IsUnknown():
		*pTransport = pgql.McpTransportStdio
		var st pgql.McpConnectionStdioTransportSettings
		if diag := tfsdk.ValueAs(ctx, data.StdioTransport.Attributes()["command"], &st.Command); diag.HasError() {
			return diag
		}
		if v, ok := data.StdioTransport.Attributes()["arguments"]; ok && !v.IsNull() && !v.IsUnknown() {
			if diag := tfsdk.ValueAs(ctx, v, &st.Arguments); diag.HasError() {
				return diag
			}
		}

		var envVars []struct {
			Name  string `tfsdk:"name"`
			Value string `tfsdk:"value"`
		}
		if v, ok := data.StdioTransport.Attributes()["environment"]; ok && !v.IsNull() && !v.IsUnknown() {
			if diag := tfsdk.ValueAs(ctx, v, &envVars); diag.HasError() {
				return diag
			}
		}
		for _, ev := range envVars {
			st.Environment = append(st.Environment, fmt.Sprintf("%s=%s", ev.Name, ev.Value))
		}
		*stdioTransport = &st
	case !data.ServerInitiatedHttpTransport.IsNull() && !data.ServerInitiatedHttpTransport.IsUnknown():
		*pTransport = pgql.McpTransportServerInitiatedHttp
	}
	return nil
}

type httpTransportStruct struct {
	Url                    string                        `tfsdk:"url"`
	OauthClientCredentials *oauthClientCredentialsStruct `tfsdk:"oauth_client_credentials"`
	BasicAuthCredentials   *basicAuthCredentialsStruct   `tfsdk:"basic_credentials"`
	HeaderCredentials      *headerAuthCredentialsStruct  `tfsdk:"header_credentials"`
}

type oauthClientCredentialsStruct struct {
	ClientId     string   `tfsdk:"client_id"`
	ClientSecret string   `tfsdk:"client_secret"`
	Scopes       []string `tfsdk:"scopes"`
}

type basicAuthCredentialsStruct struct {
	Username string `tfsdk:"username"`
	Password string `tfsdk:"password"`
}

type headerAuthCredentialsStruct struct {
	HeaderName  string `tfsdk:"header_name"`
	HeaderValue string `tfsdk:"header_value"`
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
