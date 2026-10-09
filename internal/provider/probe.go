package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// probeAttribute is the write-only probe flag of a model or embedding profile.
// It only steers the write it is sent with, so nothing of it belongs in state.
var probeAttribute = schema.BoolAttribute{
	MarkdownDescription: "Whether the server sends one request to the provider before each create or update, " +
		"and refuses a profile the provider rejects. An unreachable provider does not fail the write. " +
		"Set false to write a profile for a model the provider does not serve yet. Defaults to true.",
	Optional:  true,
	WriteOnly: true,
}

// providerProbeAttribute is the write-only probe flag of a model provider.
var providerProbeAttribute = schema.BoolAttribute{
	MarkdownDescription: "Whether the server lists the provider's models with the credentials before a create, " +
		"or an update that sends new credentials, and refuses credentials the provider rejects. An unreachable " +
		"provider does not fail the write. Set false to write credentials the provider can't check yet. " +
		"It does not cover the provider's profiles, which each take their own `probe`. Defaults to true.",
	Optional:  true,
	WriteOnly: true,
}

// configProbe is the probe flag from config, or nil to leave it to the
// server. Being write-only, it is null in the plan.
func configProbe(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) *bool {
	var probe types.Bool
	diags.Append(config.GetAttribute(ctx, path.Root("probe"), &probe)...)
	return probe.ValueBoolPointer()
}
