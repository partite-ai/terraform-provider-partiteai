package graphql

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// uploadParticleQuery is the GraphQL operation sent through postWithUpload.
// Its response is parsed into the same ParticleArtifactDetails type that
// GetParticleArtifact produces.
const uploadParticleQuery = `mutation UploadParticleArtifact($bytes: Upload!) {
  uploadParticleArtifact(bytes: $bytes) {
    id
    name
    version
    sha256
    description
    capabilities {
      http {
        allowedHosts
      }
    }
    credentials {
      name
      hosts
      required
      methods {
        name
        type
        description
        apiKeyLocation {
          kind
          name
          scheme
        }
        oauth2 {
          authorizationUrl
          tokenUrl
          deviceAuthUrl
          scopes
          flows
        }
        signingKey {
          algorithm
        }
      }
    }
    tools {
      name
      description
      inputSchema
    }
    createdAt
  }
}`

// UploadParticleArtifact uploads a particle artifact and returns it.
func UploadParticleArtifact(
	ctx context.Context,
	httpClient *http.Client,
	graphqlURL *url.URL,
	filename string,
	body io.Reader,
) (ParticleArtifactDetails, error) {
	var data struct {
		UploadParticleArtifact ParticleArtifactDetails `json:"uploadParticleArtifact"`
	}
	err := postWithUpload(ctx, httpClient, graphqlURL, uploadParticleQuery, "UploadParticleArtifact",
		map[string]any{"bytes": FileUpload{}}, "variables.bytes", filename, body, &data)
	if err != nil {
		return ParticleArtifactDetails{}, err
	}
	return data.UploadParticleArtifact, nil
}
