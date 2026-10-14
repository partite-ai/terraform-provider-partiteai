package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &InstalledParticleResource{}
var _ resource.ResourceWithImportState = &InstalledParticleResource{}

func NewInstalledParticleResource() resource.Resource {
	return &InstalledParticleResource{
		BaseResource: NewBaseResource("installed_particle"),
	}
}

type InstalledParticleResource struct {
	BaseResource
}

type InstalledParticleResourceModel struct {
	InstalledParticleBaseModel
}

func (r *InstalledParticleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An installed particle backed by a particle artifact.",
		Attributes: map[string]schema.Attribute{
			"id":         commonSchemas.id,
			"name":       commonSchemas.name,
			"created_at": commonSchemas.createdAt,
			"updated_at": commonSchemas.updatedAt,
			"artifact_id": schema.StringAttribute{
				MarkdownDescription: "ID of the particle artifact backing this install.",
				Required:            true,
			},
			"active_artifact": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"id":      schema.StringAttribute{Computed: true},
					"name":    schema.StringAttribute{Computed: true},
					"version": schema.StringAttribute{Computed: true},
				},
			},
			"kv_workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace backing this install's key-value state. Required when the " +
					"artifact's manifest grants the KV capability, and rejected when it does not.",
				Optional: true,
			},
			"mounts": schema.SetNestedAttribute{
				MarkdownDescription: "Mount bindings, each mapping a mount name declared in the artifact's " +
					"manifest to the workspace exposed at it. The same workspace may appear under several names.",
				Optional:     true,
				Computed:     true,
				Default:      setdefault.StaticValue(types.SetValueMust(installedParticleMountType.Type(), nil)),
				NestedObject: installedParticleMountType,
			},
			"credentials": schema.ListNestedAttribute{
				MarkdownDescription: "Credentials to provision on the install. Each entry corresponds to one named credential declared in the artifact's manifest.",
				Optional:            true,
				NestedObject:        installedParticleCredentialSchema(),
			},
		},
	}
}

var installedParticleMountType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"mount_name": schema.StringAttribute{
			MarkdownDescription: "Mount name declared in the artifact's manifest.",
			Required:            true,
		},
		"workspace_id": schema.StringAttribute{
			MarkdownDescription: "Workspace exposed at that mount.",
			Required:            true,
		},
	},
}

func installedParticleCredentialSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Credential declared in the artifact's manifest, a key in its top-level `credentials` map.",
				Required:            true,
			},
			"method": schema.StringAttribute{
				MarkdownDescription: "Method being provisioned, named as the manifest names it under " +
					"`credentials.<name>.methods`. An author picks these names freely, so this is not one of a " +
					"fixed set: what kind of credential it is comes from whichever `*_wo` block is set below.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"version": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(1),
			},
			"basic_wo": schema.SingleNestedAttribute{
				Optional:  true,
				WriteOnly: true,
				Attributes: map[string]schema.Attribute{
					"username": schema.StringAttribute{Required: true, WriteOnly: true},
					"password": schema.StringAttribute{Required: true, Sensitive: true, WriteOnly: true},
				},
				// The block that is set decides the kind of credential, so
				// exactly one of them has to be.
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("apikey_wo"),
						path.MatchRelative().AtParent().AtName("signing_key_wo"),
						path.MatchRelative().AtParent().AtName("raw_wo"),
						path.MatchRelative().AtParent().AtName("oauth2_client_credentials_wo"),
					),
				},
			},
			"apikey_wo": schema.SingleNestedAttribute{
				Optional:  true,
				WriteOnly: true,
				Attributes: map[string]schema.Attribute{
					"key": schema.StringAttribute{Required: true, Sensitive: true, WriteOnly: true},
				},
			},
			"signing_key_wo": schema.SingleNestedAttribute{
				Optional:  true,
				WriteOnly: true,
				Attributes: map[string]schema.Attribute{
					"algorithm": schema.StringAttribute{Required: true, WriteOnly: true},
					"key":       schema.StringAttribute{Required: true, Sensitive: true, WriteOnly: true},
				},
			},
			"raw_wo": schema.SingleNestedAttribute{
				Optional:  true,
				WriteOnly: true,
				Attributes: map[string]schema.Attribute{
					"value": schema.StringAttribute{Required: true, Sensitive: true, WriteOnly: true},
				},
			},
			"oauth2_client_credentials_wo": schema.SingleNestedAttribute{
				Optional:  true,
				WriteOnly: true,
				Attributes: map[string]schema.Attribute{
					"client_id":     schema.StringAttribute{Required: true, WriteOnly: true},
					"client_secret": schema.StringAttribute{Required: true, Sensitive: true, WriteOnly: true},
					"token_url":     schema.StringAttribute{Optional: true, WriteOnly: true},
				},
			},
		},
	}
}

func (r *InstalledParticleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_installed_particle"
}

// readConfigCredentials pulls the (write-only) credentials set from config and
// converts each entry into a GraphQL input.
func (r *InstalledParticleResource) readConfigCredentials(ctx context.Context, config tfsdk.Config) ([]pgql.ParticleCredentialInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	var credList types.List
	diags.Append(config.GetAttribute(ctx, path.Root("credentials"), &credList)...)
	if diags.HasError() {
		return nil, diags
	}
	if credList.IsNull() || credList.IsUnknown() {
		return nil, diags
	}
	var entries []installedParticleCredentialInfo
	diags.Append(credList.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil, diags
	}
	var out []pgql.ParticleCredentialInput
	for _, e := range entries {
		in, d := buildCredentialInput(ctx, e)
		diags.Append(d...)
		if d.HasError() {
			continue
		}
		out = append(out, in)
	}
	return out, diags
}

func (r *InstalledParticleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data InstalledParticleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "installed_particle", "")

	creds, diags := r.readConfigCredentials(ctx, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	mounts, diags := mountInputs(ctx, data.Mounts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := pgql.CreateInstalledParticle(ctx, r.GetClient(), &pgql.CreateInstalledParticleInput{
		Name:          data.Name.ValueString(),
		ArtifactID:    data.ArtifactId.ValueString(),
		Credentials:   creds,
		KvWorkspaceID: pgql.OptPtr(data.KvWorkspaceId.ValueStringPointer()),
		Mounts:        mounts,
	})
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "create installed particle")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, createResp.CreateInstalledParticle)...)

	logResourceOperation(ctx, "created", "installed_particle", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstalledParticleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data InstalledParticleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "installed_particle", data.Id.ValueString())

	getResp, err := pgql.GetInstalledParticle(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "installed_particle", data.Id.ValueString())
		return
	}
	if getResp.InstalledParticle == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.InstalledParticle)...)
	if data.Credentials.IsNull() {
		credentials, diags := configuredCredentials(getResp.InstalledParticle.Credentials)
		resp.Diagnostics.Append(diags...)
		data.Credentials = credentials
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstalledParticleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData InstalledParticleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "updating", "installed_particle", planData.Id.ValueString())

	in := pgql.UpdateInstalledParticleInput{
		InstalledParticleID: planData.Id.ValueString(),
	}

	if !planData.Name.Equal(stateData.Name) {
		in.Name = pgql.OptPtr(planData.Name.ValueStringPointer())
	}

	artifactChanged := !planData.ArtifactId.Equal(stateData.ArtifactId)
	credsChanged := !planData.Credentials.Equal(stateData.Credentials)

	if artifactChanged {
		in.ArtifactID = pgql.OptPtr(planData.ArtifactId.ValueStringPointer())
	}

	if artifactChanged || credsChanged {
		creds, diags := r.readConfigCredentials(ctx, req.Config)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		in.Credentials = pgql.Opt(creds)
	}

	if !planData.KvWorkspaceId.Equal(stateData.KvWorkspaceId) {
		in.KvWorkspaceID = pgql.OptPtr(planData.KvWorkspaceId.ValueStringPointer())
	}

	// Mounts are replaced wholesale rather than merged, so the whole set goes
	// whenever any of it changed.
	if !planData.Mounts.Equal(stateData.Mounts) {
		mounts, diags := mountInputs(ctx, planData.Mounts)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		in.Mounts = pgql.Opt(mounts)
	}

	updateResp, err := pgql.ModifyInstalledParticle(ctx, r.GetClient(), &in)
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "update", "installed_particle", planData.Id.ValueString())
		return
	}

	resp.Diagnostics.Append(planData.MapFromGraphQL(ctx, updateResp.UpdateInstalledParticle)...)
	logResourceOperation(ctx, "updated", "installed_particle", planData.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *InstalledParticleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data InstalledParticleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	logResourceOperation(ctx, "deleting", "installed_particle", data.Id.ValueString())
	_, err := pgql.DeleteInstalledParticle(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "delete", "installed_particle", data.Id.ValueString())
	}
}

func (r *InstalledParticleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
