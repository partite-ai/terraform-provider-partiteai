package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

// McpConnectionResourceModel extends the base model with resource-specific fields (none currently)
type McpConnectionResourceModel struct {
	McpConnectionBaseModel
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
	WriteOnly:           true,
	Attributes: map[string]schema.Attribute{
		"client_id": schema.StringAttribute{
			MarkdownDescription: "The client ID for OAuth authentication.",
			Required:            true,
			WriteOnly:           true,
		},
		"client_secret": schema.StringAttribute{
			MarkdownDescription: "The client secret for OAuth authentication.",
			Required:            true,
			Sensitive:           true,
			WriteOnly:           true,
		},
		"scopes": schema.ListAttribute{
			MarkdownDescription: "The scopes for OAuth authentication.",
			ElementType:         types.StringType,
			Optional:            true,
			WriteOnly:           true,
		},
	},
}

var basicAuthCredentialsType = schema.SingleNestedAttribute{
	MarkdownDescription: "Basic authentication credentials.",
	Optional:            true,
	WriteOnly:           true,
	Attributes: map[string]schema.Attribute{
		"username": schema.StringAttribute{
			MarkdownDescription: "The username for basic authentication.",
			Required:            true,
			WriteOnly:           true,
		},
		"password": schema.StringAttribute{
			MarkdownDescription: "The password for basic authentication.",
			Required:            true,
			Sensitive:           true,
			WriteOnly:           true,
		},
	},
}

var headerAuthCredentialsType = schema.SingleNestedAttribute{
	MarkdownDescription: "Header authentication credentials.",
	Optional:            true,
	WriteOnly:           true,
	Attributes: map[string]schema.Attribute{
		"header_name": schema.StringAttribute{
			MarkdownDescription: "The name of the header for authentication.",
			Required:            true,
			WriteOnly:           true,
		},
		"header_value": schema.StringAttribute{
			MarkdownDescription: "The value of the header for authentication.",
			Required:            true,
			Sensitive:           true,
			WriteOnly:           true,
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
					"authentication_method": schema.StringAttribute{
						MarkdownDescription: "The authentication method to use. Valid values are 'NONE', 'BASIC', 'HEADER', 'OAUTH2_AUTHORIZATION_CODE', 'OAUTH2_CLIENT_CREDENTIALS'.",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("NONE"),
						Validators: []validator.String{
							stringvalidator.OneOf("NONE", "BASIC", "HEADER", "OAUTH2_AUTHORIZATION_CODE", "OAUTH2_CLIENT_CREDENTIALS"),
						},
					},
					"oauth_client_credentials_wo": oauthClientCredentialsType,
					"basic_credentials_wo":        basicAuthCredentialsType,
					"header_credentials_wo":       headerAuthCredentialsType,
					"credentials_version": schema.Int64Attribute{
						MarkdownDescription: "The version of the credentials. Increment this value to rotate credentials.",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(1),
					},
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

	var attrs = data.HttpTransport.Attributes()
	oauthCredsAttr := attrs["oauth_client_credentials_wo"]
	basicCredsAttr := attrs["basic_credentials_wo"]
	headerCredsAttr := attrs["header_credentials_wo"]
	resp.Diagnostics.Append(req.Config.GetAttribute(
		ctx, path.Root("http_transport").AtName("oauth_client_credentials_wo"), &oauthCredsAttr,
	)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(
		ctx, path.Root("http_transport").AtName("basic_credentials_wo"), &basicCredsAttr,
	)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(
		ctx, path.Root("http_transport").AtName("header_credentials_wo"), &headerCredsAttr,
	)...)
	attrs["oauth_client_credentials_wo"] = oauthCredsAttr
	attrs["basic_credentials_wo"] = basicCredsAttr
	attrs["header_credentials_wo"] = headerCredsAttr
	data.HttpTransport = types.ObjectValueMust(data.HttpTransport.AttributeTypes(ctx), attrs)

	var createReq pgql.NewMcpConnection
	createReq.Name = data.Name.ValueString()
	if diag := fillMcpConnectionUpdateMessage(
		ctx, data,
		&createReq.Transport,
		&createReq.HttpTransport, &createReq.OauthClientCredentials, &createReq.BasicCredentials, &createReq.HeaderCredentials,
		&createReq.StdioTransport,
		true,
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
		updateCreds := false
		if !planData.HttpTransport.Equal(stateData.HttpTransport) {
			authModeChanged := false
			credentialsVersionChanged := false
			if !planData.HttpTransport.IsNull() && !planData.HttpTransport.IsUnknown() {
				if !planData.HttpTransport.Attributes()["authentication_method"].Equal(stateData.HttpTransport.Attributes()["authentication_method"]) {
					authModeChanged = true
				}
				if !planData.HttpTransport.Attributes()["credentials_version"].Equal(stateData.HttpTransport.Attributes()["credentials_version"]) {
					credentialsVersionChanged = true
				}
			}
			updateCreds = authModeChanged || credentialsVersionChanged

			if updateCreds {
				var attrs = planData.HttpTransport.Attributes()
				oauthCredsAttr := attrs["oauth_client_credentials_wo"]
				basicCredsAttr := attrs["basic_credentials_wo"]
				headerCredsAttr := attrs["header_credentials_wo"]
				resp.Diagnostics.Append(req.Config.GetAttribute(
					ctx, path.Root("http_transport").AtName("oauth_client_credentials_wo"), &oauthCredsAttr,
				)...)
				resp.Diagnostics.Append(req.Config.GetAttribute(
					ctx, path.Root("http_transport").AtName("basic_credentials_wo"), &basicCredsAttr,
				)...)
				resp.Diagnostics.Append(req.Config.GetAttribute(
					ctx, path.Root("http_transport").AtName("header_credentials_wo"), &headerCredsAttr,
				)...)
				attrs["oauth_client_credentials_wo"] = oauthCredsAttr
				attrs["basic_credentials_wo"] = basicCredsAttr
				attrs["header_credentials_wo"] = headerCredsAttr
				planData.HttpTransport = types.ObjectValueMust(planData.HttpTransport.AttributeTypes(ctx), attrs)
			}
		}

		var t pgql.McpTransport
		resp.Diagnostics.Append(fillMcpConnectionUpdateMessage(
			ctx, planData, &t,
			&update.HttpTransport, &update.OauthClientCredentials, &update.BasicCredentials, &update.HeaderCredentials,
			&update.StdioTransport,
			updateCreds,
		)...)
		update.Transport = &t

		if resp.Diagnostics.HasError() {
			return
		}
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
	return data.MapFromGraphQL(ctx, mcpConnectionDetails)
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
	updateCredentials bool,
) diag.Diagnostics {
	switch {
	case !data.HttpTransport.IsNull() && !data.HttpTransport.IsUnknown():
		*pTransport = pgql.McpTransportHttp
		ht := &pgql.McpConnectionHttpTransportSettings{}
		if diag := tfsdk.ValueAs(ctx, data.HttpTransport.Attributes()["url"], &ht.Url); diag.HasError() {
			return diag
		}
		if diag := tfsdk.ValueAs(ctx, data.HttpTransport.Attributes()["authentication_method"], &ht.AuthFlow); diag.HasError() {
			return diag
		}

		if updateCredentials {
			switch ht.AuthFlow {
			case pgql.McpAuthFlowOauth2ClientCredentials:
				var creds struct {
					ClientId     string   `tfsdk:"client_id"`
					ClientSecret string   `tfsdk:"client_secret"`
					Scopes       []string `tfsdk:"scopes"`
				}
				if diag := tfsdk.ValueAs(ctx, data.HttpTransport.Attributes()["oauth_client_credentials_wo"], &creds); diag.HasError() {
					return diag
				}
				*oauthClientCreds = &pgql.McpConnectionOauthClientCredentials{
					ClientID:     creds.ClientId,
					ClientSecret: creds.ClientSecret,
					Scopes:       creds.Scopes,
				}
			case pgql.McpAuthFlowBasic:
				var creds struct {
					Username string `tfsdk:"username"`
					Password string `tfsdk:"password"`
				}
				if diag := tfsdk.ValueAs(ctx, data.HttpTransport.Attributes()["basic_credentials_wo"], &creds); diag.HasError() {
					return diag
				}
				*basicCreds = &pgql.McpConnectionBasicCredentials{
					Username: creds.Username,
					Password: creds.Password,
				}
			case pgql.McpAuthFlowHeader:
				var creds struct {
					HeaderName  string `tfsdk:"header_name"`
					HeaderValue string `tfsdk:"header_value"`
				}
				if diag := tfsdk.ValueAs(ctx, data.HttpTransport.Attributes()["header_credentials_wo"], &creds); diag.HasError() {
					return diag
				}
				*headerCreds = &pgql.McpConnectionHeaderCredentials{
					Name:  creds.HeaderName,
					Value: creds.HeaderValue,
				}
			}
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
