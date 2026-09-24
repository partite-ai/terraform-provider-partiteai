package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// BaseResource provides common functionality for all resources.
type BaseResource struct {
	client     graphql.Client
	baseURL    *url.URL
	httpClient *http.Client
	graphqlURL *url.URL
	typeName   string
	enterprise bool
}

// NewBaseResource creates a new base resource with the given type name.
func NewBaseResource(typeName string) BaseResource {
	return BaseResource{
		typeName: typeName,
	}
}

// Configure implements the common Configure method for all resources.
func (r *BaseResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerData.client
	r.baseURL = providerData.baseURL
	r.httpClient = providerData.httpClient
	r.graphqlURL = providerData.graphqlURL
	r.enterprise = providerData.enterprise
}

// Metadata implements the common Metadata method for all resources.
func (r *BaseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.typeName
}

// ImportState implements the common ImportState method for all resources.
func (r *BaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// GetClient returns the configured GraphQL client.
func (r *BaseResource) GetClient() graphql.Client {
	return r.client
}

// GetHTTPClient returns the configured HTTP client (with the auth transport).
func (r *BaseResource) GetHTTPClient() *http.Client {
	return r.httpClient
}

// GetGraphQLURL returns the un-stripped GraphQL endpoint URL (used for multipart uploads).
func (r *BaseResource) GetGraphQLURL() *url.URL {
	return r.graphqlURL
}

// RequireEnterprise reports whether this resource may be used against the
// configured deployment, adding an error if it may not.
func (r *BaseResource) RequireEnterprise(diags *diag.Diagnostics) bool {
	return requireEnterprise(diags, r.enterprise, r.typeName)
}

// RequireEnterprise reports whether the configured endpoint is an enterprise
// deployment, adding an error if it is not. A resource that only exists there
// calls this before every operation.
func requireEnterprise(diags *diag.Diagnostics, enterprise bool, typeName string) bool {
	if enterprise {
		return true
	}
	diags.AddError(
		"Enterprise feature",
		fmt.Sprintf("partiteai_%s is only available on a Partite AI enterprise deployment.\n\n"+
			"Set `enterprise = true` on the provider block if this is one.", typeName),
	)
	return false
}
