package graphql

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// CreateWebhookDestinationWithCode creates a webhook destination whose handler
// runs code, sent as a file named filename.
func CreateWebhookDestinationWithCode(
	ctx context.Context,
	httpClient *http.Client,
	graphqlURL *url.URL,
	input *NewWebhookDestination,
	filename string,
	code io.Reader,
) (*CreateWebhookDestinationResponse, error) {
	var data CreateWebhookDestinationResponse
	err := postWithUpload(ctx, httpClient, graphqlURL, CreateWebhookDestination_Operation, "CreateWebhookDestination",
		map[string]any{"input": input}, "variables.input.handler.code", filename, code, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// ModifyWebhookDestinationWithCode updates a webhook destination and gives its
// handler new code, sent as a file named filename.
func ModifyWebhookDestinationWithCode(
	ctx context.Context,
	httpClient *http.Client,
	graphqlURL *url.URL,
	id string,
	input *UpdateWebhookDestination,
	filename string,
	code io.Reader,
) (*ModifyWebhookDestinationResponse, error) {
	var data ModifyWebhookDestinationResponse
	err := postWithUpload(ctx, httpClient, graphqlURL, ModifyWebhookDestination_Operation, "ModifyWebhookDestination",
		map[string]any{"id": id, "input": input}, "variables.input.handler.code", filename, code, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}
