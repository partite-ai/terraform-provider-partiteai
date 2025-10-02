package provider

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// BaseResource provides common functionality for all resources
type BaseResource struct {
	client   graphql.Client
	typeName string
}

// NewBaseResource creates a new base resource with the given type name
func NewBaseResource(typeName string) BaseResource {
	return BaseResource{
		typeName: typeName,
	}
}

// Configure implements the common Configure method for all resources
func (r *BaseResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Metadata implements the common Metadata method for all resources
func (r *BaseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.typeName
}

// ImportState implements the common ImportState method for all resources
func (r *BaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// GetClient returns the configured GraphQL client
func (r *BaseResource) GetClient() graphql.Client {
	return r.client
}
