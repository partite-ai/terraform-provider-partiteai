package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &McpConnectionResource{}
var _ resource.ResourceWithImportState = &McpConnectionResource{}

func NewMcpConnectionResource() resource.Resource {
	return &McpConnectionResource{}
}

// McpConnectionResource defines the resource implementation.
type McpConnectionResource struct {
	client graphql.Client
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
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "MCP connection resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "MCP connection identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "MCP connection name",
				Required:            true,
			},
			"created_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the MCP connection was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				CustomType:          timetypes.RFC3339Type{},
				Computed:            true,
				MarkdownDescription: "Timestamp when the MCP connection was last updated",
			},
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

func (r *McpConnectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(graphql.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected graphql.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *McpConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data McpConnectionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

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

	createResp, err := pgql.CreateMcpConnection(ctx, r.client, createReq)

	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create agent, got error: %s", err))
		return
	}

	if diag := r.mapFields(ctx, &data, createResp.CreateMcpConnection); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpConnectionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	mcpConnectionResp, err := pgql.GetMcpConnection(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent, got error: %s", err))
		return
	}

	if diag := r.mapFields(ctx, &data, *mcpConnectionResp.McpConnection); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData McpConnectionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

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

	updateResp, err := pgql.ModifyMcpConnection(ctx, r.client, planData.Id.ValueString(), update)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update mcp connection, got error: %s", err))
		return
	}

	if diag := r.mapFields(ctx, &planData, updateResp.UpdateMcpConnection); diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *McpConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpConnectionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := pgql.DeleteMcpConnection(ctx, r.client, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete mcp connection, got error: %s", err))
		return
	}
}

func (r *McpConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *McpConnectionResource) mapFields(ctx context.Context, data *McpConnectionResourceModel, mcpConnectionDetails pgql.McpConnectionDetails) diag.Diagnostics {
	var diag diag.Diagnostics
	data.Id = types.StringValue(mcpConnectionDetails.Id)
	data.CreatedAt, diag = timetypes.NewRFC3339Value(mcpConnectionDetails.CreatedAt)
	if diag.HasError() {
		return diag
	}
	data.UpdatedAt, diag = timetypes.NewRFC3339Value(mcpConnectionDetails.UpdatedAt)
	if diag.HasError() {
		return diag
	}
	switch mcpConnectionDetails.Transport {
	case pgql.McpTransportHttp:
		var httpTransport struct {
			Url                    types.String `tfsdk:"url"`
			OauthClientCredentials types.Object `tfsdk:"oauth_client_credentials"`
			BasicAuthCredentials   types.Object `tfsdk:"basic_credentials"`
			HeaderCredentials      types.Object `tfsdk:"header_credentials"`
		}

		if !data.HttpTransport.IsNull() && !data.HttpTransport.IsUnknown() {
			if diag := tfsdk.ValueAs(ctx, data.HttpTransport, &httpTransport); diag.HasError() {
				return diag
			}
		}
		httpTransport.Url = types.StringValue(mcpConnectionDetails.HttpTransport.Url)
		switch mcpConnectionDetails.HttpTransport.AuthFlow {
		case pgql.McpAuthFlowOauth2AuthorizationCode, pgql.McpAuthFlowNone:
			httpTransport.OauthClientCredentials = types.ObjectNull(attrsToTypes(oauthClientCredentialsType.Attributes))
			httpTransport.BasicAuthCredentials = types.ObjectNull(attrsToTypes(basicAuthCredentialsType.Attributes))
			httpTransport.HeaderCredentials = types.ObjectNull(attrsToTypes(headerAuthCredentialsType.Attributes))
		case pgql.McpAuthFlowOauth2ClientCredentials:
			if httpTransport.OauthClientCredentials.IsNull() || httpTransport.OauthClientCredentials.IsUnknown() {
				httpTransport.OauthClientCredentials = types.ObjectValueMust(
					attrsToTypes(oauthClientCredentialsType.Attributes),
					map[string]attr.Value{
						"client_id":     types.StringValue("<unknown>"),
						"client_secret": types.StringValue("<unknown>"),
						"scopes":        types.ListValueMust(types.StringType, nil),
					},
				)
			}
			httpTransport.BasicAuthCredentials = types.ObjectNull(attrsToTypes(basicAuthCredentialsType.Attributes))
			httpTransport.HeaderCredentials = types.ObjectNull(attrsToTypes(headerAuthCredentialsType.Attributes))
		case pgql.McpAuthFlowBasic:
			if httpTransport.BasicAuthCredentials.IsNull() || httpTransport.BasicAuthCredentials.IsUnknown() {
				httpTransport.BasicAuthCredentials = types.ObjectValueMust(
					attrsToTypes(basicAuthCredentialsType.Attributes),
					map[string]attr.Value{
						"username": types.StringValue("<unknown>"),
						"password": types.StringValue("<unknown>"),
					},
				)
			}
			httpTransport.OauthClientCredentials = types.ObjectNull(attrsToTypes(oauthClientCredentialsType.Attributes))
			httpTransport.HeaderCredentials = types.ObjectNull(attrsToTypes(headerAuthCredentialsType.Attributes))
		case pgql.McpAuthFlowHeader:
			if httpTransport.HeaderCredentials.IsNull() || httpTransport.HeaderCredentials.IsUnknown() {
				httpTransport.HeaderCredentials = types.ObjectValueMust(
					attrsToTypes(headerAuthCredentialsType.Attributes),
					map[string]attr.Value{
						"header_name":  types.StringValue("<unknown>"),
						"header_value": types.StringValue("<unknown>"),
					},
				)
			}
			httpTransport.OauthClientCredentials = types.ObjectNull(attrsToTypes(oauthClientCredentialsType.Attributes))
			httpTransport.BasicAuthCredentials = types.ObjectNull(attrsToTypes(basicAuthCredentialsType.Attributes))
		}
		if diag := tfsdk.ValueFrom(ctx, httpTransport, data.HttpTransport.Type(ctx), &data.HttpTransport); diag.HasError() {
			return diag
		}
		data.StdioTransport = types.ObjectNull(data.StdioTransport.AttributeTypes(ctx))
		data.ServerInitiatedHttpTransport = types.ObjectNull(data.ServerInitiatedHttpTransport.AttributeTypes(ctx))
	case pgql.McpTransportStdio:
		type envVar struct {
			Name  string `tfsdk:"name"`
			Value string `tfsdk:"value"`
		}
		var stdioTransport struct {
			Command     string   `tfsdk:"command"`
			Arguments   []string `tfsdk:"arguments"`
			Environment []envVar `tfsdk:"environment"`
		}

		stdioTransport.Command = mcpConnectionDetails.StdioTransport.Command
		stdioTransport.Arguments = mcpConnectionDetails.StdioTransport.Arguments
		for _, ev := range mcpConnectionDetails.StdioTransport.Environment {
			name, value, _ := strings.Cut(ev, "=")
			stdioTransport.Environment = append(stdioTransport.Environment, envVar{Name: name, Value: value})
		}

		if diag := tfsdk.ValueFrom(ctx, stdioTransport, data.StdioTransport.Type(ctx), &data.StdioTransport); diag.HasError() {
			return diag
		}
		data.HttpTransport = types.ObjectNull(data.HttpTransport.AttributeTypes(ctx))
		data.ServerInitiatedHttpTransport = types.ObjectNull(data.ServerInitiatedHttpTransport.AttributeTypes(ctx))
	case pgql.McpTransportServerInitiatedHttp:
		if diag := tfsdk.ValueFrom(ctx, struct{}{}, data.ServerInitiatedHttpTransport.Type(ctx), &data.ServerInitiatedHttpTransport); diag.HasError() {
			return diag
		}
		data.HttpTransport = types.ObjectNull(data.HttpTransport.AttributeTypes(ctx))
		data.StdioTransport = types.ObjectNull(data.StdioTransport.AttributeTypes(ctx))
	}

	return diag
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

func attrsToTypes(attrs map[string]schema.Attribute) map[string]attr.Type {
	types := make(map[string]attr.Type, len(attrs))
	for k, v := range attrs {
		types[k] = v.GetType()
	}
	return types
}
