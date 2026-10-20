package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// AccessPointBaseModel contains the common fields for access point resources and data sources.
type AccessPointBaseModel struct {
	Id                         types.String      `tfsdk:"id"`
	Name                       types.String      `tfsdk:"name"`
	Description                types.String      `tfsdk:"description"`
	Tags                       types.List        `tfsdk:"tags"`
	Url                        types.String      `tfsdk:"url"`
	AuthorizationServerUrl     types.String      `tfsdk:"authorization_server_url"`
	ClientTokenLifetimeSeconds types.Int64       `tfsdk:"client_token_lifetime_seconds"`
	Issuers                    types.List        `tfsdk:"issuers"`
	BoundaryIds                types.Set         `tfsdk:"boundary_ids"`
	CreatedAt                  timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt                  timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps a GraphQL AccessPointDetails response to the AccessPointBaseModel.
func (m *AccessPointBaseModel) MapFromGraphQL(ctx context.Context, accessPointDetails pgql.AccessPointDetails) diag.Diagnostics {
	return setFields(
		ctx,
		&m.Id, accessPointDetails.Id,
		&m.Name, accessPointDetails.Name,
		&m.Description, accessPointDetails.Description,
		&m.Tags, accessPointDetails.Tags,
		&m.Url, accessPointDetails.Url,
		&m.AuthorizationServerUrl, accessPointDetails.AuthorizationServerUrl,
		&m.ClientTokenLifetimeSeconds, accessPointDetails.ClientTokenLifetimeSeconds,
		&m.Issuers, newStructSliceMappable(accessPointDetails.Issuers, accessPointIssuerStruct{}),
		&m.BoundaryIds, boundaryIDs(accessPointDetails.Boundaries),
		&m.CreatedAt, accessPointDetails.CreatedAt,
		&m.UpdatedAt, accessPointDetails.UpdatedAt,
	)
}

type accessPointIssuerStruct struct {
	Issuer        types.String         `tfsdk:"issuer"`
	Audience      types.String         `tfsdk:"audience"`
	CheckAudience types.Bool           `tfsdk:"check_audience"`
	JwksUrl       types.String         `tfsdk:"jwks_url"`
	Jwks          jsontypes.Normalized `tfsdk:"jwks"`
}

func (i *accessPointIssuerStruct) FromGQL(src pgql.AccessPointDetailsIssuersAccessPointIssuer) diag.Diagnostics {
	i.Issuer = types.StringValue(src.Issuer)
	i.Audience = types.StringPointerValue(src.Audience)
	i.CheckAudience = types.BoolValue(src.Audience != nil)
	i.JwksUrl = types.StringPointerValue(src.JwksUrl)
	i.Jwks = jsontypes.NewNormalizedNull()
	if src.Jwks != nil {
		i.Jwks = jsontypes.NewNormalizedValue(string(*src.Jwks))
	}
	return nil
}

// accessPointIssuerInputs reads the planned issuers as the API takes them.
func accessPointIssuerInputs(ctx context.Context, issuers types.List) ([]pgql.AccessPointIssuerInput, diag.Diagnostics) {
	var planned []accessPointIssuerStruct
	diags := tfsdk.ValueAs(ctx, issuers, &planned)
	if diags.HasError() {
		return nil, diags
	}

	inputs := make([]pgql.AccessPointIssuerInput, 0, len(planned))
	for _, issuer := range planned {
		input := pgql.AccessPointIssuerInput{
			Issuer:  issuer.Issuer.ValueString(),
			JwksURL: pgql.OptPtr(issuer.JwksUrl.ValueStringPointer()),
		}
		// An audience that is left out is unknown in the plan, and the API
		// makes it the access point's URL.
		switch {
		case !issuer.CheckAudience.ValueBool():
			if !issuer.Audience.IsNull() && !issuer.Audience.IsUnknown() {
				diags.AddError("Invalid issuer", "Issuer "+issuer.Issuer.ValueString()+" sets audience with check_audience = false.")
			}
			input.Audience = pgql.OptNil[string](nil)
		case !issuer.Audience.IsUnknown():
			input.Audience = pgql.OptPtr(issuer.Audience.ValueStringPointer())
		}
		if !issuer.Jwks.IsNull() && !issuer.Jwks.IsUnknown() {
			input.JWKS = json.RawMessage(issuer.Jwks.ValueString())
		}
		inputs = append(inputs, input)
	}
	return inputs, diags
}
