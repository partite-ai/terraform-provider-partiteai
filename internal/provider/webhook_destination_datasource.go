package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &WebhookDestinationDataSource{}

func NewWebhookDestinationDataSource() datasource.DataSource {
	return &WebhookDestinationDataSource{
		BaseDataSource: NewBaseDataSource("webhook_destination"),
	}
}

// WebhookDestinationDataSource defines the data source implementation.
type WebhookDestinationDataSource struct {
	BaseDataSource
}

// WebhookDestinationDataSourceModel extends the base model with data source-specific fields.
type WebhookDestinationDataSourceModel struct {
	WebhookDestinationBaseModel
}

func (d *WebhookDestinationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Webhook Destination data source",
		Attributes: map[string]schema.Attribute{
			"id":          commonDataSourceSchemas.id,
			"name":        commonDataSourceSchemas.name,
			"description": commonDataSourceSchemas.description,
			"external_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "External ID for the webhook destination",
			},
			"mesh_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the mesh associated with this webhook destination",
			},
			"mesh_version_label": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Optional mesh version label",
			},
			"input_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the input to invoke",
			},
			"invoke_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URL to invoke the webhook destination",
			},
			"created_at": commonDataSourceSchemas.createdAt,
			"updated_at": commonDataSourceSchemas.updatedAt,
		},
	}
}

func (d *WebhookDestinationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WebhookDestinationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logDataSourceOperation(ctx, "reading", "webhook_destination", data.Id.ValueString())

	webhookResp, err := pgql.GetWebhookDestination(ctx, d.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "webhook_destination", data.Id.ValueString())
		return
	}

	// Use the shared mapping function directly
	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, &webhookResp.WebhookDestination)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.InvokeUrl = types.StringValue(d.baseURL.JoinPath("webhooks", data.ExternalId.ValueString()).String())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
