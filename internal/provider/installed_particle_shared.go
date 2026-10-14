package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// InstalledParticleBaseModel contains all the common fields for both resource and data source.
type InstalledParticleBaseModel struct {
	Id             types.String      `tfsdk:"id"`
	Name           types.String      `tfsdk:"name"`
	ArtifactId     types.String      `tfsdk:"artifact_id"`
	Credentials    types.List        `tfsdk:"credentials"`
	KvWorkspaceId  types.String      `tfsdk:"kv_workspace_id"`
	Mounts         types.Set         `tfsdk:"mounts"`
	ActiveArtifact types.Object      `tfsdk:"active_artifact"`
	CreatedAt      timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt      timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps the install response onto the base model. It does NOT
// touch Credentials — those are write-only and not returned by the API.
func (m *InstalledParticleBaseModel) MapFromGraphQL(ctx context.Context, src pgql.InstalledParticleDetails) diag.Diagnostics {
	diags := setFields(
		ctx,
		&m.Id, src.Id,
		&m.Name, src.Name,
		&m.ArtifactId, src.ActiveArtifact.Id,
		&m.CreatedAt, src.CreatedAt,
		&m.UpdatedAt, src.UpdatedAt,
	)
	if diags.HasError() {
		return diags
	}
	activeObj, objDiags := types.ObjectValueFrom(ctx, activeArtifactAttrTypes(), activeArtifactStruct{
		Id:      src.ActiveArtifact.Id,
		Name:    src.ActiveArtifact.Name,
		Version: src.ActiveArtifact.Version,
	})
	diags.Append(objDiags...)
	m.ActiveArtifact = activeObj

	// The API leaves kvWorkspace null unless the manifest grants KV.
	m.KvWorkspaceId = types.StringNull()
	if src.KvWorkspace != nil {
		m.KvWorkspaceId = types.StringValue(src.KvWorkspace.Id)
	}

	// mounts defaults to an empty set in the schema, so an install with none
	// maps to an empty set rather than null.
	bindings := make([]installedParticleMountInfo, 0, len(src.Mounts))
	for _, mount := range src.Mounts {
		bindings = append(bindings, installedParticleMountInfo{
			MountName:   types.StringValue(mount.MountName),
			WorkspaceId: types.StringValue(mount.Workspace.Id),
		})
	}
	mountSet, setDiags := types.SetValueFrom(ctx, mountAttrType(), bindings)
	diags.Append(setDiags...)
	m.Mounts = mountSet

	return diags
}

// installedParticleMountInfo is one mount binding as a configuration writes it.
type installedParticleMountInfo struct {
	MountName   types.String `tfsdk:"mount_name"`
	WorkspaceId types.String `tfsdk:"workspace_id"`
}

func mountAttrType() attr.Type {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"mount_name":   types.StringType,
		"workspace_id": types.StringType,
	}}
}

// mountInputs turns the configured mount set into the GraphQL input. The API
// takes a non-null list, so an unset or empty set sends an empty one.
func mountInputs(ctx context.Context, mounts types.Set) ([]pgql.NewInstalledParticleMount, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := []pgql.NewInstalledParticleMount{}

	if mounts.IsNull() || mounts.IsUnknown() {
		return out, diags
	}

	var bindings []installedParticleMountInfo
	diags.Append(mounts.ElementsAs(ctx, &bindings, false)...)
	if diags.HasError() {
		return nil, diags
	}

	for _, binding := range bindings {
		out = append(out, pgql.NewInstalledParticleMount{
			MountName:   binding.MountName.ValueString(),
			WorkspaceID: binding.WorkspaceId.ValueString(),
		})
	}
	return out, diags
}

type activeArtifactStruct struct {
	Id      string `tfsdk:"id"`
	Name    string `tfsdk:"name"`
	Version string `tfsdk:"version"`
}

func activeArtifactAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":      types.StringType,
		"name":    types.StringType,
		"version": types.StringType,
	}
}

// installedParticleCredentialInfo is the TF-side representation of a single
// credential slot entry. Exactly the _wo block matching Method should be set.
type installedParticleCredentialInfo struct {
	Name                      types.String `tfsdk:"name"`
	Method                    types.String `tfsdk:"method"`
	Version                   types.Int64  `tfsdk:"version"`
	BasicWo                   types.Object `tfsdk:"basic_wo"`
	ApikeyWo                  types.Object `tfsdk:"apikey_wo"`
	SigningKeyWo              types.Object `tfsdk:"signing_key_wo"`
	RawWo                     types.Object `tfsdk:"raw_wo"`
	OAuth2ClientCredentialsWo types.Object `tfsdk:"oauth2_client_credentials_wo"`
}

// buildCredentialInput converts one credential entry from config into the
// GraphQL input shape.
//
// The kind of credential comes from whichever _wo block the configuration set,
// not from the method name. The API matches `method` against the keys under
// `credentials.<name>.methods` in the artifact's manifest, and an author names
// those freely — "basic", "pat", "deploy-key" — so the provider cannot treat
// the name as a kind.
func buildCredentialInput(ctx context.Context, entry installedParticleCredentialInfo) (pgql.ParticleCredentialInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	in := pgql.ParticleCredentialInput{
		CredentialName: entry.Name.ValueString(),
		Method:         entry.Method.ValueString(),
	}

	blocks := map[string]types.Object{
		"basic_wo":                     entry.BasicWo,
		"apikey_wo":                    entry.ApikeyWo,
		"signing_key_wo":               entry.SigningKeyWo,
		"raw_wo":                       entry.RawWo,
		"oauth2_client_credentials_wo": entry.OAuth2ClientCredentialsWo,
	}
	var set []string
	for name, block := range blocks {
		if !block.IsNull() {
			set = append(set, name)
		}
	}
	sort.Strings(set)

	if len(set) != 1 {
		// A schema validator catches this at plan time; this keeps the
		// function total if one ever reaches here anyway.
		diags.AddError(
			"Invalid credential",
			fmt.Sprintf("credential %q must set exactly one of basic_wo, apikey_wo, signing_key_wo, "+
				"raw_wo or oauth2_client_credentials_wo, but sets %d: %v",
				entry.Name.ValueString(), len(set), set),
		)
		return in, diags
	}

	opts := basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}

	switch set[0] {
	case "basic_wo":
		var sub struct {
			Username string `tfsdk:"username"`
			Password string `tfsdk:"password"`
		}
		diags.Append(entry.BasicWo.As(ctx, &sub, opts)...)
		in.Basic = &pgql.BasicParticleCredentialInput{Username: sub.Username, Password: sub.Password}
	case "apikey_wo":
		var sub struct {
			Key string `tfsdk:"key"`
		}
		diags.Append(entry.ApikeyWo.As(ctx, &sub, opts)...)
		in.Apikey = &pgql.ApiKeyParticleCredentialInput{Key: sub.Key}
	case "signing_key_wo":
		var sub struct {
			Algorithm string `tfsdk:"algorithm"`
			Key       string `tfsdk:"key"`
		}
		diags.Append(entry.SigningKeyWo.As(ctx, &sub, opts)...)
		in.SigningKey = &pgql.SigningKeyParticleCredentialInput{Algorithm: sub.Algorithm, Key: sub.Key}
	case "raw_wo":
		var sub struct {
			Value string `tfsdk:"value"`
		}
		diags.Append(entry.RawWo.As(ctx, &sub, opts)...)
		in.Raw = &pgql.RawParticleCredentialInput{Value: sub.Value}
	case "oauth2_client_credentials_wo":
		var sub struct {
			ClientId     string  `tfsdk:"client_id"`
			ClientSecret string  `tfsdk:"client_secret"`
			TokenUrl     *string `tfsdk:"token_url"`
		}
		diags.Append(entry.OAuth2ClientCredentialsWo.As(ctx, &sub, opts)...)
		in.OAuth2ClientCredentials = &pgql.OAuth2ClientCredentialsParticleCredentialInput{
			ClientID:     sub.ClientId,
			ClientSecret: sub.ClientSecret,
			TokenUrl:     sub.TokenUrl,
		}
	}

	return in, diags
}

func credentialEntryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":    types.StringType,
		"method":  types.StringType,
		"version": types.Int64Type,
		"basic_wo": types.ObjectType{AttrTypes: map[string]attr.Type{
			"username": types.StringType,
			"password": types.StringType,
		}},
		"apikey_wo": types.ObjectType{AttrTypes: map[string]attr.Type{
			"key": types.StringType,
		}},
		"signing_key_wo": types.ObjectType{AttrTypes: map[string]attr.Type{
			"algorithm": types.StringType,
			"key":       types.StringType,
		}},
		"raw_wo": types.ObjectType{AttrTypes: map[string]attr.Type{
			"value": types.StringType,
		}},
		"oauth2_client_credentials_wo": types.ObjectType{AttrTypes: map[string]attr.Type{
			"client_id":     types.StringType,
			"client_secret": types.StringType,
			"token_url":     types.StringType,
		}},
	}
}

// configuredCredentials is the credentials list for an install whose state
// has none, as it is just after an import: each configured credential by name
// and method, at version 1. The secrets are never read back.
func configuredCredentials(src []pgql.InstalledParticleDetailsCredentialsInstalledParticleCredential) (types.List, diag.Diagnostics) {
	entryType := types.ObjectType{AttrTypes: credentialEntryAttrTypes()}
	if len(src) == 0 {
		return types.ListNull(entryType), nil
	}
	var diags diag.Diagnostics
	entries := make([]attr.Value, 0, len(src))
	for _, c := range src {
		values := map[string]attr.Value{
			"name":    types.StringValue(c.Name),
			"method":  types.StringValue(c.Method),
			"version": types.Int64Value(1),
		}
		for name, typ := range credentialEntryAttrTypes() {
			if _, ok := values[name]; !ok {
				values[name] = types.ObjectNull(typ.(types.ObjectType).AttrTypes)
			}
		}
		entry, d := types.ObjectValue(entryType.AttrTypes, values)
		diags.Append(d...)
		entries = append(entries, entry)
	}
	list, d := types.ListValue(entryType, entries)
	diags.Append(d...)
	return list, diags
}
