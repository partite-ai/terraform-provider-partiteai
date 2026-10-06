package provider

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// SkillBaseModel contains all the common fields for both resource and data source.
type SkillBaseModel struct {
	Id           types.String      `tfsdk:"id"`
	Name         types.String      `tfsdk:"name"`
	DeclaredName types.String      `tfsdk:"declared_name"`
	Description  types.String      `tfsdk:"description"`
	Instructions types.String      `tfsdk:"instructions"`
	Files        types.Map         `tfsdk:"files"`
	CreatedAt    timetypes.RFC3339 `tfsdk:"created_at"`
	UpdatedAt    timetypes.RFC3339 `tfsdk:"updated_at"`
}

// MapFromGraphQL maps GraphQL response to the base model.
func (m *SkillBaseModel) MapFromGraphQL(ctx context.Context, s *pgql.SkillDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Append(setFields(
		ctx,
		&m.Id, s.Id,
		&m.Name, s.Name,
		&m.DeclaredName, s.DeclaredName,
		&m.Description, s.Description,
		&m.Instructions, s.Instructions,
		&m.CreatedAt, s.CreatedAt,
		&m.UpdatedAt, s.UpdatedAt,
	)...)

	files := make(map[string]string, len(s.Files))
	for _, f := range s.Files {
		files[f.Path] = f.Content
	}
	filesValue, d := types.MapValueFrom(ctx, types.StringType, files)
	diags.Append(d...)
	m.Files = filesValue

	return diags
}

// skillSourceFiles turns the files map of a configuration into the files of a
// skill folder, in path order so a request is the same each time.
func skillSourceFiles(ctx context.Context, files types.Map) ([]pgql.SkillSourceFile, diag.Diagnostics) {
	var byPath map[string]string
	diags := files.ElementsAs(ctx, &byPath, false)
	if diags.HasError() {
		return nil, diags
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]pgql.SkillSourceFile, 0, len(paths))
	for _, p := range paths {
		out = append(out, pgql.SkillSourceFile{Path: p, Text: byPath[p]})
	}
	return out, diags
}

// skillFilePathsAreClean refuses a file path the API would store differently,
// such as "./SKILL.md" for "SKILL.md". The configuration would otherwise
// differ from what is read back on every plan.
var skillFilePathsAreClean = mapvalidator.KeysAre(cleanSkillFilePathValidator{})

type cleanSkillFilePathValidator struct{}

var _ validator.String = cleanSkillFilePathValidator{}

func (cleanSkillFilePathValidator) Description(context.Context) string {
	return "a path relative to the skill folder, with no ./, .. or leading /"
}

func (v cleanSkillFilePathValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (cleanSkillFilePathValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	p := req.ConfigValue.ValueString()
	clean := path.Clean(p)
	if p == "" || clean != p || path.IsAbs(p) || clean == ".." || strings.HasPrefix(clean, "../") {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid skill file path",
			fmt.Sprintf("%q must be a path relative to the skill folder, such as \"SKILL.md\" or \"reference/forms.md\", with no ./, .. or leading /", p))
	}
}
