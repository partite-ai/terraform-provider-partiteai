package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

var _ resource.Resource = &ParticleArtifactResource{}
var _ resource.ResourceWithImportState = &ParticleArtifactResource{}
var _ resource.ResourceWithModifyPlan = &ParticleArtifactResource{}

func NewParticleArtifactResource() resource.Resource {
	return &ParticleArtifactResource{
		BaseResource: NewBaseResource("particle_artifact"),
	}
}

type ParticleArtifactResource struct {
	BaseResource
}

type ParticleArtifactResourceModel struct {
	ParticleArtifactBaseModel
}

func (r *ParticleArtifactResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A particle artifact (.particle tarball) uploaded to the Partite AI mesh.",
		Attributes: map[string]schema.Attribute{
			"id":         commonSchemas.id,
			"created_at": commonSchemas.createdAt,
			"source_path": schema.StringAttribute{
				MarkdownDescription: "Path to the local `.particle` tarball. It is not kept in state: the " +
					"artifact is the file's content, which `source_hash` tracks, so moving the file changes nothing.",
				Required:  true,
				WriteOnly: true,
			},
			"source_hash": schema.StringAttribute{
				MarkdownDescription: "SHA-256 hex digest of the uploaded file, as the server reports it. A file " +
					"at `source_path` with other content forces replacement.",
				Computed: true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Manifest name.",
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Manifest version.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Manifest description.",
				Computed:            true,
			},
			"capabilities": schema.SingleNestedAttribute{
				MarkdownDescription: "Capabilities declared by the manifest.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"http": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"allowed_hosts": schema.ListAttribute{
								ElementType: types.StringType,
								Computed:    true,
							},
						},
					},
				},
			},
			"credentials": schema.ListNestedAttribute{
				MarkdownDescription: "Credential slots declared in the manifest. Each entry describes one named credential the install configures separately.",
				Computed:            true,
				NestedObject:        particleArtifactCredentialResourceSchema(),
			},
			"tools": schema.ListNestedAttribute{
				MarkdownDescription: "Tools declared in the manifest.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
						"input_schema": schema.StringAttribute{
							CustomType: jsontypes.NormalizedType{},
							Computed:   true,
						},
					},
				},
			},
		},
	}
}

func particleArtifactCredentialResourceSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Computed: true},
			"hosts":    schema.ListAttribute{ElementType: types.StringType, Computed: true},
			"required": schema.BoolAttribute{Computed: true},
			"methods": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":        schema.StringAttribute{Computed: true},
						"type":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
						"apikey_location": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"kind":   schema.StringAttribute{Computed: true},
								"name":   schema.StringAttribute{Computed: true},
								"scheme": schema.StringAttribute{Computed: true},
							},
						},
						"oauth2": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"authorization_url": schema.StringAttribute{Computed: true},
								"token_url":         schema.StringAttribute{Computed: true},
								"device_auth_url":   schema.StringAttribute{Computed: true},
								"scopes":            schema.ListAttribute{ElementType: types.StringType, Computed: true},
								"flows":             schema.ListAttribute{ElementType: types.StringType, Computed: true},
							},
						},
						"signing_key": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"algorithm": schema.StringAttribute{Computed: true},
							},
						},
					},
				},
			},
		},
	}
}

func (r *ParticleArtifactResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_particle_artifact"
}

func (r *ParticleArtifactResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ParticleArtifactResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "creating", "particle_artifact", "")

	var srcPath string
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("source_path"), &srcPath)...)
	if resp.Diagnostics.HasError() {
		return
	}
	f, err := os.Open(srcPath)
	if err != nil {
		resp.Diagnostics.AddError("Cannot open source_path", err.Error())
		return
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		resp.Diagnostics.AddError("Cannot hash source_path", err.Error())
		return
	}
	hashHex := hex.EncodeToString(hasher.Sum(nil))

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		resp.Diagnostics.AddError("Cannot rewind source_path", err.Error())
		return
	}

	details, err := pgql.UploadParticleArtifact(ctx, r.GetHTTPClient(), r.GetGraphQLURL(), filepath.Base(srcPath), f)
	if err != nil {
		handleAPIError(&resp.Diagnostics, err, "upload particle artifact")
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, details)...)
	if data.SourceHash.ValueString() != hashHex {
		resp.Diagnostics.AddError("Upload changed the file",
			fmt.Sprintf("The server's digest %s of the uploaded file is not the file's digest %s.", data.SourceHash.ValueString(), hashHex))
		return
	}
	data.SourcePath = types.StringNull()

	logResourceOperation(ctx, "created", "particle_artifact", data.Id.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ParticleArtifactResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ParticleArtifactResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	logResourceOperation(ctx, "reading", "particle_artifact", data.Id.ValueString())

	getResp, err := pgql.GetParticleArtifact(ctx, r.GetClient(), data.Id.ValueString())
	if err != nil {
		handleAPIErrorWithDetails(&resp.Diagnostics, err, "read", "particle_artifact", data.Id.ValueString())
		return
	}
	if getResp.ParticleArtifact == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(data.MapFromGraphQL(ctx, *getResp.ParticleArtifact)...)
	// State from before source_path was write-only still holds it.
	data.SourcePath = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ParticleArtifactResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Unexpected update",
		"partiteai_particle_artifact is replace-only; Update should not be reached.",
	)
}

func (r *ParticleArtifactResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Artifacts are garbage-collected by the server when no install references them.
}

func (r *ParticleArtifactResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// ModifyPlan replaces the artifact when the file at source_path no longer has
// the content that was uploaded. An artifact never changes, so new content is a
// new artifact.
func (r *ParticleArtifactResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var srcPath types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("source_path"), &srcPath)...)
	var stateHash types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("source_hash"), &stateHash)...)
	if resp.Diagnostics.HasError() || srcPath.IsUnknown() || srcPath.IsNull() {
		return
	}

	hashHex, err := hashFile(srcPath.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Cannot hash source_path", err.Error())
		return
	}
	if hashHex == stateHash.ValueString() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("source_hash"), hashHex)...)
	resp.RequiresReplace = append(resp.RequiresReplace, path.Root("source_hash"))
}

// hashFile is the SHA-256 hex digest of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
