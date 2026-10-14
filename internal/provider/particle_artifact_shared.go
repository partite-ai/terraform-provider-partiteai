package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// ParticleArtifactBaseModel contains all the common fields for both resource
// and data source.
type ParticleArtifactBaseModel struct {
	Id           types.String      `tfsdk:"id"`
	SourcePath   types.String      `tfsdk:"source_path"`
	SourceHash   types.String      `tfsdk:"source_hash"`
	Name         types.String      `tfsdk:"name"`
	Version      types.String      `tfsdk:"version"`
	Description  types.String      `tfsdk:"description"`
	CreatedAt    timetypes.RFC3339 `tfsdk:"created_at"`
	Capabilities types.Object      `tfsdk:"capabilities"`
	Credentials  types.List        `tfsdk:"credentials"`
	Tools        types.List        `tfsdk:"tools"`
}

// MapFromGraphQL maps the artifact response onto the base model. SourceHash is
// the server's digest of the uploaded file. SourcePath is not touched: the
// resource never keeps it in state, and the data source has none.
func (m *ParticleArtifactBaseModel) MapFromGraphQL(ctx context.Context, src pgql.ParticleArtifactDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, src.Id,
		&m.SourceHash, src.Sha256,
		&m.Name, src.Name,
		&m.Version, src.Version,
		&m.Description, src.Description,
		&m.CreatedAt, src.CreatedAt,
	)
	if diags.HasError() {
		return diags
	}

	var allowedHosts []string
	if src.Capabilities.Http != nil {
		allowedHosts = src.Capabilities.Http.AllowedHosts
	} else {
		allowedHosts = []string{}
	}
	httpObj, hostDiags := types.ObjectValueFrom(ctx, particleHttpCapabilityAttrTypes(), particleHttpCapabilityStruct{AllowedHosts: allowedHosts})
	diags.Append(hostDiags...)
	if diags.HasError() {
		return diags
	}
	capsObj, capDiags := types.ObjectValue(particleCapabilitiesAttrTypes(), map[string]attr.Value{
		"http": httpObj,
	})
	diags.Append(capDiags...)
	if diags.HasError() {
		return diags
	}
	m.Capabilities = capsObj

	diags.Append(setFields(
		ctx,
		&m.Credentials, newStructSliceMappable(src.Credentials, particleArtifactCredentialStruct{}),
		&m.Tools, newStructSliceMappable(src.Tools, particleToolStruct{}),
	)...)
	return diags
}

type particleHttpCapabilityStruct struct {
	AllowedHosts []string `tfsdk:"allowed_hosts"`
}

type particleArtifactCredentialStruct struct {
	Name     string                           `tfsdk:"name"`
	Hosts    []string                         `tfsdk:"hosts"`
	Required bool                             `tfsdk:"required"`
	Methods  []particleCredentialMethodStruct `tfsdk:"methods"`
}

func (t *particleArtifactCredentialStruct) FromGQL(src pgql.ParticleArtifactDetailsCredentialsParticleArtifactCredential) diag.Diagnostics {
	var diags diag.Diagnostics
	t.Name = src.Name
	t.Hosts = src.Hosts
	t.Required = src.Required
	for _, m := range src.Methods {
		var method particleCredentialMethodStruct
		diags.Append(method.FromGQL(m)...)
		t.Methods = append(t.Methods, method)
	}
	return diags
}

type particleCredentialMethodStruct struct {
	Name           string                        `tfsdk:"name"`
	Type           string                        `tfsdk:"type"`
	Description    *string                       `tfsdk:"description"`
	ApiKeyLocation *particleApiKeyLocationStruct `tfsdk:"apikey_location"`
	OAuth2         *particleOAuth2DetailsStruct  `tfsdk:"oauth2"`
	SigningKey     *particleSigningKeyStruct     `tfsdk:"signing_key"`
}

func (t *particleCredentialMethodStruct) FromGQL(src pgql.ParticleArtifactDetailsCredentialsParticleArtifactCredentialMethodsParticleCredentialMethod) diag.Diagnostics {
	t.Name = src.Name
	t.Type = string(src.Type)
	t.Description = src.Description
	if src.ApiKeyLocation != nil {
		t.ApiKeyLocation = &particleApiKeyLocationStruct{
			Kind:   string(src.ApiKeyLocation.Kind),
			Name:   src.ApiKeyLocation.Name,
			Scheme: src.ApiKeyLocation.Scheme,
		}
	}
	if src.Oauth2 != nil {
		var flows []string
		for _, f := range src.Oauth2.Flows {
			flows = append(flows, string(f))
		}
		t.OAuth2 = &particleOAuth2DetailsStruct{
			AuthorizationUrl: src.Oauth2.AuthorizationUrl,
			TokenUrl:         src.Oauth2.TokenUrl,
			DeviceAuthUrl:    src.Oauth2.DeviceAuthUrl,
			Scopes:           src.Oauth2.Scopes,
			Flows:            flows,
		}
	}
	if src.SigningKey != nil {
		t.SigningKey = &particleSigningKeyStruct{Algorithm: src.SigningKey.Algorithm}
	}
	return nil
}

type particleApiKeyLocationStruct struct {
	Kind   string  `tfsdk:"kind"`
	Name   *string `tfsdk:"name"`
	Scheme *string `tfsdk:"scheme"`
}

type particleOAuth2DetailsStruct struct {
	AuthorizationUrl *string  `tfsdk:"authorization_url"`
	TokenUrl         *string  `tfsdk:"token_url"`
	DeviceAuthUrl    *string  `tfsdk:"device_auth_url"`
	Scopes           []string `tfsdk:"scopes"`
	Flows            []string `tfsdk:"flows"`
}

type particleSigningKeyStruct struct {
	Algorithm string `tfsdk:"algorithm"`
}

type particleToolStruct struct {
	Name        string               `tfsdk:"name"`
	Description *string              `tfsdk:"description"`
	InputSchema jsontypes.Normalized `tfsdk:"input_schema"`
}

func (t *particleToolStruct) FromGQL(src pgql.ParticleArtifactDetailsToolsParticleTool) diag.Diagnostics {
	t.Name = src.Name
	t.Description = src.Description
	t.InputSchema = jsontypes.NewNormalizedValue(string(src.InputSchema))
	return nil
}

func particleHttpCapabilityAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"allowed_hosts": types.ListType{ElemType: types.StringType},
	}
}

func particleCapabilitiesAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"http": types.ObjectType{AttrTypes: particleHttpCapabilityAttrTypes()},
	}
}
