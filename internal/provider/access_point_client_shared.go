package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// AccessPointClientBaseModel contains the common fields for access point client resources and data sources.
type AccessPointClientBaseModel struct {
	Id            types.String         `tfsdk:"id"`
	AccessPointId types.String         `tfsdk:"access_point_id"`
	ClientId      types.String         `tfsdk:"client_id"`
	Name          types.String         `tfsdk:"name"`
	Claims        jsontypes.Normalized `tfsdk:"claims"`
	RedirectUris  types.Set            `tfsdk:"redirect_uris"`
	CreatedAt     timetypes.RFC3339    `tfsdk:"created_at"`
	UpdatedAt     timetypes.RFC3339    `tfsdk:"updated_at"`
}

// MapFromGraphQL maps a GraphQL AccessPointClientDetails response to the AccessPointClientBaseModel.
func (m *AccessPointClientBaseModel) MapFromGraphQL(ctx context.Context, clientDetails pgql.AccessPointClientDetails) diag.Diagnostics {
	// A client with no claims has an empty object, which a configuration
	// that leaves claims out reads as null.
	var claims map[string]any
	if err := json.Unmarshal(clientDetails.Claims, &claims); err == nil && len(claims) == 0 {
		m.Claims = jsontypes.NewNormalizedNull()
	} else {
		m.Claims = jsontypes.NewNormalizedValue(string(clientDetails.Claims))
	}
	return setFields(
		ctx,
		&m.Id, clientDetails.Id,
		&m.AccessPointId, clientDetails.AccessPointID,
		&m.ClientId, clientDetails.ClientId,
		&m.Name, clientDetails.Name,
		&m.RedirectUris, clientDetails.RedirectUris,
		&m.CreatedAt, clientDetails.CreatedAt,
		&m.UpdatedAt, clientDetails.UpdatedAt,
	)
}
