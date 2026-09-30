package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &McpConnectionSubscriptionResource{}
var _ resource.ResourceWithImportState = &McpConnectionSubscriptionResource{}

func NewMcpConnectionSubscriptionResource() resource.Resource {
	return &McpConnectionSubscriptionResource{
		BaseResource: NewBaseResource("mcp_connection_subscription"),
	}
}

// McpConnectionSubscriptionResource manages one subscription of a tunnel
// connection.
//
// Subscriptions are their own resource rather than a list on the connection
// because a tunnel key has to reference one, and because they do not rank against
// each other: a conversation eligible for several is served by whichever relay
// polls first, so there is no ordering for a list to express.
type McpConnectionSubscriptionResource struct {
	BaseResource
}

type McpConnectionSubscriptionResourceModel struct {
	Id              types.String      `tfsdk:"id"`
	McpConnectionId types.String      `tfsdk:"mcp_connection_id"`
	Name            types.String      `tfsdk:"name"`
	Criteria        types.Set         `tfsdk:"criteria"`
	CreatedAt       timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt       timetypes.RFC3339 `tfsdk:"updated_at"`
}

// criterionStruct is one tag a conversation must carry to be eligible.
type criterionStruct struct {
	Name  string `tfsdk:"name"`
	Value string `tfsdk:"value"`
}

func (r *McpConnectionSubscriptionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One subscription of a tunnel MCP connection. A subscription decides which " +
			"relays serve which conversations: a conversation tagged `env=dev` reaches a development relay, " +
			"one tagged `developer=bob` reaches Bob's workstation, one tagged `tenant=acme` reaches that " +
			"customer's deployment.\n\n" +
			"Subscriptions do not rank against each other. A conversation may be eligible for several, and " +
			"whichever relay polls first takes the message, so an overlap is redundancy rather than a conflict.",
		Attributes: map[string]schema.Attribute{
			"id":         commonSchemas.id,
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
			"mcp_connection_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The tunnel connection this subscription belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Names the subscription within its connection.",
			},
			"criteria": schema.SetNestedAttribute{
				Optional: true,
				MarkdownDescription: "Tags a conversation must carry for this subscription to serve it. Every " +
					"criterion has to match, and matching is containment: a criterion of `env=dev` matches a " +
					"conversation tagged both `env=dev` and `env=staging`.\n\n" +
					"Left out, the subscription accepts every conversation on the connection.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Tag name.",
						},
						"value": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Tag value.",
						},
					},
				},
			},
		},
	}
}

func (r *McpConnectionSubscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// A subscription is reached through its connection, so importing needs both.
	connectionID, subscriptionID, ok := cutImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			"The import identifier must be in the format '<mcp_connection_id>:<subscription_id>'.\n\n"+
				"Example: 'cmco_1234567890abcdef:csub_1234567890abcdef'",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mcp_connection_id"), connectionID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), subscriptionID)...)
}

func (r *McpConnectionSubscriptionResource) criteriaInput(ctx context.Context, data *McpConnectionSubscriptionResourceModel, diags *diag.Diagnostics) []pgql.McpConnectionSubscriptionCriterionInput {
	if data.Criteria.IsNull() || data.Criteria.IsUnknown() {
		return nil
	}

	var criteria []criterionStruct
	diags.Append(data.Criteria.ElementsAs(ctx, &criteria, false)...)
	if diags.HasError() {
		return nil
	}

	out := make([]pgql.McpConnectionSubscriptionCriterionInput, 0, len(criteria))
	for _, c := range criteria {
		out = append(out, pgql.McpConnectionSubscriptionCriterionInput{Name: c.Name, Value: c.Value})
	}
	return out
}

func (r *McpConnectionSubscriptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data McpConnectionSubscriptionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "mcp_connection_subscription", data.Name.ValueString())

	criteria := r.criteriaInput(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := pgql.CreateMcpConnectionSubscription(ctx, r.GetClient(), pgql.NewMcpConnectionSubscription{
		McpConnectionID: data.McpConnectionId.ValueString(),
		Name:            data.Name.ValueString(),
		Criteria:        criteria,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create mcp connection subscription")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, created.CreateMcpConnectionSubscription)...)
	logResourceOperation(ctx, "created", "mcp_connection_subscription", data.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionSubscriptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpConnectionSubscriptionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "mcp_connection_subscription", data.Name.ValueString())

	found, diags := readMcpConnectionSubscription(ctx, r.GetClient(), data.McpConnectionId.ValueString(), data.Id.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if found == nil {
		// Gone from the API, so drop it from state and let Terraform recreate it.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, found.McpConnectionSubscriptionDetails)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionSubscriptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data McpConnectionSubscriptionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "mcp_connection_subscription", data.Name.ValueString())

	criteria := r.criteriaInput(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Criteria are always sent, so clearing them in the configuration clears them
	// on the subscription rather than leaving the previous set in place.
	name := data.Name.ValueString()
	updated, err := pgql.ModifyMcpConnectionSubscription(ctx, r.GetClient(), data.Id.ValueString(),
		pgql.McpConnectionSubscriptionChanges{
			Name:     &name,
			Criteria: criteria,
		})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "update mcp connection subscription")
		return
	}

	resp.Diagnostics.Append(r.mapResponse(ctx, &data, updated.UpdateMcpConnectionSubscription)...)
	logResourceOperation(ctx, "updated", "mcp_connection_subscription", data.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpConnectionSubscriptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpConnectionSubscriptionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "deleting", "mcp_connection_subscription", data.Name.ValueString())

	// Deleting takes the subscription's tunnel keys with it, so no working
	// credentials are left behind for a route that no longer exists.
	if _, err := pgql.DeleteMcpConnectionSubscription(ctx, r.GetClient(), data.Id.ValueString()); err != nil {
		handleAPIError(&resp.Diagnostics, err, "delete mcp connection subscription")
		return
	}
}

func (r *McpConnectionSubscriptionResource) mapResponse(ctx context.Context, data *McpConnectionSubscriptionResourceModel, details pgql.McpConnectionSubscriptionDetails) diag.Diagnostics {
	return mapMcpConnectionSubscription(ctx, &data.Id, &data.Name, &data.Criteria, &data.CreatedAt, &data.UpdatedAt, details)
}

func (r *McpConnectionSubscriptionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_mcp_connection_subscription", req.ProviderTypeName)
}
