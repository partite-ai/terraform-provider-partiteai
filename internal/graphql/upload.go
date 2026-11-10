package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
)

// FileUpload marks the variable a file in a multipart request fills in. It is
// sent as null, as the GraphQL multipart request spec asks.
type FileUpload struct{}

func (FileUpload) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

// postWithUpload sends a GraphQL operation as a graphql-multipart-request-spec
// request. The file fills the variable at variablePath, such as
// "variables.input.handler.code", and the response's data is decoded into out.
// genqlient cannot send files, so operations that take one go through here.
func postWithUpload(
	ctx context.Context,
	httpClient *http.Client,
	graphqlURL *url.URL,
	operation, operationName string,
	variables any,
	variablePath, filename string,
	file io.Reader,
	out any,
) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	opBytes, err := json.Marshal(map[string]any{
		"query":         operation,
		"operationName": operationName,
		"variables":     variables,
	})
	if err != nil {
		return fmt.Errorf("marshal operations: %w", err)
	}
	if err := mw.WriteField("operations", string(opBytes)); err != nil {
		return fmt.Errorf("write operations field: %w", err)
	}
	mapBytes, err := json.Marshal(map[string][]string{"0": {variablePath}})
	if err != nil {
		return fmt.Errorf("marshal map: %w", err)
	}
	if err := mw.WriteField("map", string(mapBytes)); err != nil {
		return fmt.Errorf("write map field: %w", err)
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="0"; filename=%q`, filename))
	h.Set("Content-Type", "application/octet-stream")
	filePart, err := mw.CreatePart(h)
	if err != nil {
		return fmt.Errorf("create file part: %w", err)
	}
	if _, err := io.Copy(filePart, file); err != nil {
		return fmt.Errorf("copy file body: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL.String(), &buf)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("upload returned HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	parsed := struct {
		Data   any `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}{Data: out}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBytes))
	}
	if len(parsed.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", parsed.Errors[0].Message)
	}
	return nil
}
