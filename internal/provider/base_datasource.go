package provider

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// BaseDataSource provides common functionality for all data sources
type BaseDataSource struct {
	client   graphql.Client
	typeName string
}

// NewBaseDataSource creates a new base data source with the given type name
func NewBaseDataSource(typeName string) BaseDataSource {
	return BaseDataSource{
		typeName: typeName,
	}
}

// Configure implements the common Configure method for all data sources
func (d *BaseDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(graphql.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected graphql.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

// Metadata implements the common Metadata method for all data sources
func (d *BaseDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.typeName
}

// GetClient returns the configured GraphQL client
func (d *BaseDataSource) GetClient() graphql.Client {
	return d.client
}
