package provider

import (
	"context"
	"net/http"
	"net/url"
	"os"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure PartiteAIProvider satisfies various provider interfaces.
var _ provider.Provider = &PartiteAIProvider{}
var _ provider.ProviderWithFunctions = &PartiteAIProvider{}
var _ provider.ProviderWithEphemeralResources = &PartiteAIProvider{}

// PartiteAIProvider defines the provider implementation.
type PartiteAIProvider struct {
	version string
	client  graphql.Client
}

// PartiteAIProviderModel describes the provider data model.
type PartiteAIProviderModel struct {
	Endpoint  types.String `tfsdk:"endpoint"`
	APIKey    types.String `tfsdk:"api_key"`
	APISecret types.String `tfsdk:"api_secret"`
}

func (p *PartiteAIProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "partiteai"
	resp.Version = p.version
}

func (p *PartiteAIProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "The base URL for the Partite AI API. For example, `https://api.partite.ai/graphql`",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The API key used to authenticate with the Partite AI API.",
				Optional:            true,
				Sensitive:           false,
			},
			"api_secret": schema.StringAttribute{
				MarkdownDescription: "The API secret used to authenticate with the Partite AI API.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *PartiteAIProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config PartiteAIProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.
	if config.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Unknown endpoint",
			"The provider cannot create the Partite.ai API client as there is an unknown configuration value for the Partite.ai API endpoint. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the PARTITE_AI_ENDPOINT environment variable.",
		)
	}

	if config.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown API Key",
			"The provider cannot create the Partite.ai API client as there is an unknown configuration value for the API key. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the PARTITE_AI_API_KEY environment variable.",
		)
	}

	if config.APISecret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_secret"),
			"Unknown API Secret",
			"The provider cannot create the Partite.ai API client as there is an unknown configuration value for the API secret. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the PARTITE_AI_API_SECRET environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.
	endpoint := "https://api.partite.ai/graphql"
	if env := os.Getenv("PARTITE_AI_ENDPOINT"); env != "" {
		endpoint = env
	}
	apiKey := os.Getenv("PARTITE_AI_API_KEY")
	apiSecret := os.Getenv("PARTITE_AI_API_SECRET")

	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}

	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}

	if !config.APISecret.IsNull() {
		apiSecret = config.APISecret.ValueString()
	}

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing endpoint",
			"The provider cannot create the Partite.ai API client as there is a missing or empty value for the Partite.ai API endpoint. "+
				"Set the endpoint value in the configuration or use the PARTITE_AI_ENDPOINT environment variable. "+
				"If you are using the environment variable, ensure it is not empty.",
		)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing API Key",
			"The provider cannot create the Partite.ai API client as there is a missing or empty value for the API key. "+
				"Set the API key value in the configuration or use the PARTITE_AI_API_KEY environment variable. "+
				"If you are using the environment variable, ensure it is not empty.",
		)
	}

	if apiSecret == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_secret"),
			"Missing API Secret",
			"The provider cannot create the Partite.ai API client as there is a missing or empty value for the API secret. "+
				"Set the API secret value in the configuration or use the PARTITE_AI_API_SECRET environment variable. "+
				"If you are using the environment variable, ensure it is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if p.client != nil {
		// Use the configured client if it was provided (e.g. for testing).
		resp.DataSourceData = p.client
		resp.ResourceData = p.client
		return
	}

	httpClient := &http.Client{
		Transport: &authTransport{
			apiKey:    apiKey,
			apiSecret: apiSecret,
		},
	}

	client := graphql.NewClient(endpoint, httpClient)

	baseURL, err := url.Parse(endpoint)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid endpoint URL",
			"The provider cannot create the Partite.ai API client because the endpoint URL is invalid: "+err.Error(),
		)
		return
	}
	baseURL.Path = ""

	resp.DataSourceData = &providerData{
		client:  client,
		baseURL: baseURL,
	}
	resp.ResourceData = &providerData{
		client:  client,
		baseURL: baseURL,
	}
}

func (p *PartiteAIProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAgentResource,
		NewMcpConnectionResource,
		NewApiKeyResource,
		NewWebhookDestinationResource,
		NewModelProviderResource,
		NewModelProfileResource,
		NewModelSetResource,
		NewMeshResource,
		NewMeshLinkResource,
		NewMeshInputResource,
	}
}

func (p *PartiteAIProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *PartiteAIProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAgentDataSource,
		NewApiKeyDataSource,
		NewWebhookDestinationDataSource,
		NewMcpConnectionDataSource,
		NewMeshDataSource,
		NewMeshInputDataSource,
		NewMeshLinkDataSource,
		NewModelProviderDataSource,
		NewModelProfileDataSource,
		NewModelSetDataSource,
	}
}

func (p *PartiteAIProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PartiteAIProvider{
			version: version,
		}
	}
}

func NewWithClient(version string, client graphql.Client) func() provider.Provider {
	return func() provider.Provider {
		return &PartiteAIProvider{
			version: version,
			client:  client,
		}
	}
}

type authTransport struct {
	apiKey    string
	apiSecret string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.apiKey, t.apiSecret)
	return http.DefaultTransport.RoundTrip(req)
}

type providerData struct {
	client  graphql.Client
	baseURL *url.URL
}
