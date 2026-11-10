package provider

import (
	"context"

	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &WebhookDestinationResource{}
var _ resource.ResourceWithImportState = &WebhookDestinationResource{}
var _ resource.ResourceWithModifyPlan = &WebhookDestinationResource{}

func NewWebhookDestinationResource() resource.Resource {
	return &WebhookDestinationResource{
		BaseResource: NewBaseResource("webhook_destination"),
	}
}

type WebhookDestinationResource struct {
	BaseResource
}

// WebhookDestinationResourceModel extends the base model with resource-specific fields.
type WebhookDestinationResourceModel struct {
	WebhookDestinationBaseModel
	Handler types.Object `tfsdk:"handler"`
}

func (r *WebhookDestinationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Webhook Destination resource",
		Attributes: map[string]schema.Attribute{
			"id":          commonSchemas.id,
			"name":        commonSchemas.name,
			"description": commonSchemas.description,
			"external_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "External ID for the webhook destination",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"invoke_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Invoke URL for the webhook destination",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mesh_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the mesh associated with this webhook destination",
			},
			"mesh_version_label": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional mesh version label",
			},
			"input_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the input to invoke",
			},
			"handler":    webhookHandlerResourceSchema(),
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
		},
	}
}

func webhookHandlerResourceSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "Code that turns each request to the destination into the input's message, " +
			"and can refuse the request. Without it, the request body is the message.",
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "`JAVASCRIPT` or `WASM_COMPONENT`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("JAVASCRIPT", "WASM_COMPONENT")},
			},
			"source_path": schema.StringAttribute{
				MarkdownDescription: "Path to the handler's code: an ES module for `JAVASCRIPT`, or a wasm " +
					"component for `WASM_COMPONENT`. It is not kept in state: `source_hash` tracks the file's " +
					"content, so moving the file changes nothing.",
				Required:  true,
				WriteOnly: true,
			},
			"source_hash": schema.StringAttribute{
				MarkdownDescription: "SHA-256 hex digest of the code, as the server reports it. A file at " +
					"`source_path` with other content is uploaded on the next apply.",
				Computed: true,
			},
			"config": schema.MapAttribute{
				MarkdownDescription: "Settings the handler reads with `config.get`.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"secrets_wo": schema.MapAttribute{
				MarkdownDescription: "Values the handler reads with `secrets.get`, such as a signing key. " +
					"Write-only: never stored in state. Adding or removing a name takes effect on the next " +
					"apply. Terraform cannot see a value change, so increment `secrets_version` to send changed values.",
				ElementType: types.StringType,
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
			},
			"secrets_version": schema.Int64Attribute{
				MarkdownDescription: "The version of the handler's secrets. Increment it to send every value " +
					"in `secrets_wo` again.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(1),
			},
			"secret_names": schema.SetAttribute{
				MarkdownDescription: "The names of the secrets the server holds for the handler.",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

// webhookHandlerWriteOnly is what a configuration gives a handler that
// Terraform never keeps: where its code is, and its secrets.
type webhookHandlerWriteOnly struct {
	sourcePath string
	secrets    map[string]string
}

func readWebhookHandlerWriteOnly(ctx context.Context, config tfsdk.Config) (webhookHandlerWriteOnly, diag.Diagnostics) {
	var diags diag.Diagnostics
	var sourcePath types.String
	var secrets types.Map
	diags.Append(config.GetAttribute(ctx, path.Root("handler").AtName("source_path"), &sourcePath)...)
	diags.Append(config.GetAttribute(ctx, path.Root("handler").AtName("secrets_wo"), &secrets)...)
	out := webhookHandlerWriteOnly{sourcePath: sourcePath.ValueString(), secrets: map[string]string{}}
	if !secrets.IsNull() {
		diags.Append(secrets.ElementsAs(ctx, &out.secrets, false)...)
	}
	return out, diags
}

// webhookHandlerConfigMap is a handler's configured config, empty when there
// is none.
func webhookHandlerConfigMap(ctx context.Context, handler *webhookHandlerModel) (map[string]string, diag.Diagnostics) {
	config := map[string]string{}
	if handler.Config.IsNull() {
		return config, nil
	}
	diags := handler.Config.ElementsAs(ctx, &config, false)
	return config, diags
}

// handlerOf is the handler of data, or nil when it has none.
func handlerOf(ctx context.Context, data *WebhookDestinationResourceModel) (*webhookHandlerModel, diag.Diagnostics) {
	if data.Handler.IsNull() || data.Handler.IsUnknown() {
		return nil, nil
	}
	var handler webhookHandlerModel
	diags := data.Handler.As(ctx, &handler, basetypes.ObjectAsOptions{})
	return &handler, diags
}

// ModifyPlan plans the handler's computed attributes from its configuration:
// the hash of the file at source_path, and the names in secrets_wo. A file
// with other content, or a name added or removed, is then a change to apply.
func (r *WebhookDestinationResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	r.planWebhookHandler(ctx, req, resp)
	// Terraform sees no change when only the code file or the secrets'
	// names changed, so it kept updated_at. The update will change it.
	if !resp.Diagnostics.HasError() && !req.State.Raw.IsNull() && !resp.Plan.Raw.Equal(req.State.Raw) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("updated_at"), timetypes.NewRFC3339Unknown())...)
	}
}

func (r *WebhookDestinationResource) planWebhookHandler(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var handler types.Object
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("handler"), &handler)...)
	if resp.Diagnostics.HasError() || handler.IsNull() || handler.IsUnknown() {
		return
	}

	var sourcePath types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("handler").AtName("source_path"), &sourcePath)...)
	if !sourcePath.IsNull() && !sourcePath.IsUnknown() {
		hash, err := hashFile(sourcePath.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Cannot hash handler.source_path", err.Error())
			return
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("handler").AtName("source_hash"), hash)...)
	}

	var secrets types.Map
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("handler").AtName("secrets_wo"), &secrets)...)
	namesPath := path.Root("handler").AtName("secret_names")
	if secrets.IsUnknown() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, namesPath, types.SetUnknown(types.StringType))...)
		return
	}
	names := []string{}
	for name := range secrets.Elements() {
		names = append(names, name)
	}
	namesValue, diags := types.SetValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, namesPath, namesValue)...)
}

func (r *WebhookDestinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WebhookDestinationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "webhook_destination", "")

	input := pgql.NewWebhookDestination{
		Name:             data.Name.ValueString(),
		Description:      pgql.OptPtr(data.Description.ValueStringPointer()),
		MeshID:           data.MeshId.ValueString(),
		MeshVersionLabel: pgql.OptPtr(data.MeshVersionLabel.ValueStringPointer()),
		InputName:        data.InputName.ValueString(),
	}
	handler, diags := handlerOf(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var created *pgql.WebhookDestinationDetails
	if handler == nil {
		createResp, err := pgql.CreateWebhookDestination(ctx, r.GetClient(), &input)
		if err != nil {
			handleAPIError(&resp.Diagnostics, err, "create webhook destination")
			return
		}
		created = &createResp.CreateWebhookDestination
	} else {
		writeOnly, diags := readWebhookHandlerWriteOnly(ctx, req.Config)
		resp.Diagnostics.Append(diags...)
		config, diags := webhookHandlerConfigMap(ctx, handler)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		handlerInput := pgql.WebhookHandlerCreateInput{Type: handler.Type.ValueString(), Config: pgql.Opt(config)}
		if len(writeOnly.secrets) > 0 {
			handlerInput.Secrets = pgql.Opt(writeOnly.secrets)
		}
		input.Handler = pgql.Opt(handlerInput)

		code, err := os.Open(writeOnly.sourcePath)
		if err != nil {
			resp.Diagnostics.AddError("Cannot open handler.source_path", err.Error())
			return
		}
		defer code.Close()
		createResp, err := pgql.CreateWebhookDestinationWithCode(ctx, r.GetHTTPClient(), r.GetGraphQLURL(), &input, filepath.Base(writeOnly.sourcePath), code)
		if err != nil {
			handleAPIError(&resp.Diagnostics, err, "create webhook destination")
			return
		}
		created = &createResp.CreateWebhookDestination
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, created, handler)...)
	data.InvokeUrl = types.StringValue(r.baseURL.JoinPath("webhooks", data.ExternalId.ValueString()).String())
	logResourceOperation(ctx, "created", "webhook_destination", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookDestinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WebhookDestinationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "webhook_destination", data.Id.ValueString())

	getResp, err := pgql.GetWebhookDestination(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "webhook_destination", data.Id.ValueString())
		return
	}

	prior, diags := handlerOf(ctx, &data)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(r.mapResponse(ctx, &data, &getResp.WebhookDestination, prior)...)
	data.InvokeUrl = types.StringValue(r.baseURL.JoinPath("webhooks", data.ExternalId.ValueString()).String())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookDestinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData WebhookDestinationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "webhook_destination", planData.Id.ValueString())

	input := pgql.UpdateWebhookDestination{
		Name:             pgql.OptNil(planData.Name.ValueStringPointer()),
		Description:      pgql.OptNil(planData.Description.ValueStringPointer()),
		MeshID:           pgql.OptNil(planData.MeshId.ValueStringPointer()),
		MeshVersionLabel: pgql.OptNil(planData.MeshVersionLabel.ValueStringPointer()),
		InputName:        pgql.OptNil(planData.InputName.ValueStringPointer()),
	}
	planHandler, diags := handlerOf(ctx, &planData)
	resp.Diagnostics.Append(diags...)
	stateHandler, diags := handlerOf(ctx, &stateData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	upload := false
	var writeOnly webhookHandlerWriteOnly
	switch {
	case planHandler == nil && stateHandler != nil:
		input.Handler = pgql.Optional[pgql.WebhookHandlerUpdateInput]{Set: true, Nil: true}
	case planHandler != nil:
		writeOnly, diags = readWebhookHandlerWriteOnly(ctx, req.Config)
		resp.Diagnostics.Append(diags...)
		config, diags := webhookHandlerConfigMap(ctx, planHandler)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		handlerInput := pgql.WebhookHandlerUpdateInput{Type: pgql.Opt(planHandler.Type.ValueString()), Config: pgql.Opt(config)}

		// A new handler, or one whose code or type changed, is sent its code.
		upload = stateHandler == nil ||
			!planHandler.SourceHash.Equal(stateHandler.SourceHash) ||
			!planHandler.Type.Equal(stateHandler.Type)
		if upload {
			handlerInput.Code = &pgql.FileUpload{}
		}

		// Names the server holds that aren't configured are deleted, and new
		// names are sent. Every value is sent when the handler is new or
		// secrets_version changed, since Terraform cannot see values change.
		var serverNames []string
		resendAll := stateHandler == nil
		if stateHandler != nil {
			resp.Diagnostics.Append(stateHandler.SecretNames.ElementsAs(ctx, &serverNames, false)...)
			resendAll = !planHandler.SecretsVersion.Equal(stateHandler.SecretsVersion)
		}
		if changes := webhookHandlerSecretChanges(writeOnly.secrets, serverNames, resendAll); len(changes) > 0 {
			handlerInput.Secrets = pgql.Opt(changes)
		}
		input.Handler = pgql.Opt(handlerInput)
	}

	var updated *pgql.WebhookDestinationDetails
	if upload {
		code, err := os.Open(writeOnly.sourcePath)
		if err != nil {
			resp.Diagnostics.AddError("Cannot open handler.source_path", err.Error())
			return
		}
		defer code.Close()
		updateResp, err := pgql.ModifyWebhookDestinationWithCode(ctx, r.GetHTTPClient(), r.GetGraphQLURL(), planData.Id.ValueString(), &input, filepath.Base(writeOnly.sourcePath), code)
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "webhook_destination", planData.Id.ValueString())
			return
		}
		updated = &updateResp.UpdateWebhookDestination
	} else {
		updateResp, err := pgql.ModifyWebhookDestination(ctx, r.GetClient(), planData.Id.ValueString(), &input)
		if err != nil {
			handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "webhook_destination", planData.Id.ValueString())
			return
		}
		updated = &updateResp.UpdateWebhookDestination
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &planData, updated, planHandler)...)

	logResourceOperation(ctx, "updated", "webhook_destination", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *WebhookDestinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WebhookDestinationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "webhook_destination", data.Id.ValueString())

	_, err := pgql.DeleteWebhookDestination(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "webhook_destination", data.Id.ValueString())
		return
	}

	logResourceOperation(ctx, "deleted", "webhook_destination", data.Id.ValueString())
}

// mapResponse sets data from the server's destination. prior is the handler
// as configured or last stored, if any, which keeps what the server doesn't
// return: secrets_version, and a config left out.
func (r *WebhookDestinationResource) mapResponse(ctx context.Context, data *WebhookDestinationResourceModel, webhookDestination *pgql.WebhookDestinationDetails, prior *webhookHandlerModel) diag.Diagnostics {
	diags := data.MapFromGraphQL(ctx, webhookDestination)
	server := webhookDestination.Handler
	if server == nil {
		data.Handler = types.ObjectNull(webhookHandlerAttrTypes())
		return diags
	}

	handler := webhookHandlerModel{
		Type:           types.StringValue(string(server.Type)),
		SourcePath:     types.StringNull(),
		SourceHash:     types.StringValue(server.Code.Sha256),
		SecretsWO:      types.MapNull(types.StringType),
		SecretsVersion: types.Int64Value(1),
	}
	if prior != nil && !prior.SecretsVersion.IsNull() && !prior.SecretsVersion.IsUnknown() {
		handler.SecretsVersion = prior.SecretsVersion
	}
	config, d := webhookHandlerConfig(ctx, server.Config, prior == nil || prior.Config.IsNull())
	diags.Append(d...)
	handler.Config = config
	names, d := types.SetValueFrom(ctx, types.StringType, server.SecretNames)
	diags.Append(d...)
	handler.SecretNames = names

	obj, d := types.ObjectValueFrom(ctx, webhookHandlerAttrTypes(), handler)
	diags.Append(d...)
	data.Handler = obj
	return diags
}
