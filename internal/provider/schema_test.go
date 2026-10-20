package provider

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

const schemaGoldenPath = "testdata/provider_schema.json"

// TestProviderSchemaSnapshot pins the exact schema Terraform sees. Any change
// to an attribute — its type, whether it is required, computed, sensitive or
// write-only — shows up here as a diff. Run with -update to accept a change.
//
// This is the cheapest regression net in the suite: it needs no backend and no
// Terraform binary, and it catches accidental attribute removals and renames,
// which silently break every configuration in the wild.
func TestProviderSchemaSnapshot(t *testing.T) {
	t.Parallel()

	got, err := json.MarshalIndent(providerSchema(t), "", "  ")
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	got = append(got, '\n')

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(schemaGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(schemaGoldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", schemaGoldenPath)
		return
	}

	want, err := os.ReadFile(schemaGoldenPath)
	if err != nil {
		t.Fatalf("read golden: %v\nRun: go test ./internal/provider -run TestProviderSchemaSnapshot -update", err)
	}

	if string(got) == string(want) {
		return
	}

	t.Errorf("provider schema changed.\n%s\n\n"+
		"If the change is intended, run:\n"+
		"  go test ./internal/provider -run TestProviderSchemaSnapshot -update\n"+
		"and review the diff in %s before committing.",
		firstDiff(string(want), string(got)), schemaGoldenPath)
}

// TestSchemaMatchesModel checks that every resource and data source model
// struct lines up with its schema. A tfsdk field with no matching attribute
// (or an attribute with no field) panics at runtime inside Terraform, usually
// only on the code path that touches it.
func TestSchemaMatchesModel(t *testing.T) {
	t.Parallel()

	schemas := providerSchema(t)

	// The registry is hand-written, so a resource added to the provider but
	// not to it would be covered by nothing at all, silently. Check it is
	// complete before trusting anything it says.
	for name := range schemas.ResourceSchemas {
		if _, ok := resourceModels[name]; !ok {
			t.Errorf("resource %s is not in resourceModels, so nothing checks its model", name)
		}
	}
	for name := range schemas.DataSourceSchemas {
		if _, ok := dataSourceModels[name]; !ok {
			t.Errorf("data source %s is not in dataSourceModels, so nothing checks its model", name)
		}
	}

	for name, model := range resourceModels {
		t.Run("resource/"+name, func(t *testing.T) {
			t.Parallel()
			s, ok := schemas.ResourceSchemas[name]
			if !ok {
				t.Fatalf("no schema for resource %s", name)
			}
			compareFields(t, attributeNames(s.Block), tfsdkFields(model))
		})
	}

	for name, model := range dataSourceModels {
		t.Run("datasource/"+name, func(t *testing.T) {
			t.Parallel()
			s, ok := schemas.DataSourceSchemas[name]
			if !ok {
				t.Fatalf("no schema for data source %s", name)
			}
			compareFields(t, attributeNames(s.Block), tfsdkFields(model))
		})
	}
}

// TestEveryResourceHasADataSource keeps the two halves of the provider in step.
func TestEveryResourceHasADataSource(t *testing.T) {
	t.Parallel()

	schemas := providerSchema(t)

	// Resources with no read-only counterpart by design.
	exempt := map[string]bool{
		// A tunnel key is its credential, and that is readable only when the key is
		// created. A data source could return nothing a subscription does not
		// already expose.
		"partiteai_mcp_connection_tunnel_key": true,
		// A client secret is readable only when it is made, and the client's
		// data source already names the client.
		"partiteai_access_point_client_secret": true,
	}

	for name := range schemas.ResourceSchemas {
		if exempt[name] {
			continue
		}
		if _, ok := schemas.DataSourceSchemas[name]; !ok {
			t.Errorf("resource %s has no matching data source", name)
		}
	}
}

// TestIDIsComputed guards the import contract: every resource is imported by
// its id, so id must be computed and never required.
func TestIDIsComputed(t *testing.T) {
	t.Parallel()

	// mesh_input is addressed by mesh_id + name, not by an id of its own, and
	// implements its own ImportState to match.
	exempt := map[string]bool{"partiteai_mesh_input": true}

	for name, s := range providerSchema(t).ResourceSchemas {
		if exempt[name] {
			continue
		}
		var id *tfprotov6.SchemaAttribute
		for _, a := range s.Block.Attributes {
			if a.Name == "id" {
				id = a
				break
			}
		}
		if id == nil {
			t.Errorf("resource %s has no id attribute", name)
			continue
		}
		if !id.Computed {
			t.Errorf("resource %s: id must be Computed", name)
		}
		if id.Required {
			t.Errorf("resource %s: id must not be Required", name)
		}
	}
}

func attributeNames(block *tfprotov6.SchemaBlock) []string {
	names := make([]string, 0, len(block.Attributes)+len(block.BlockTypes))
	for _, a := range block.Attributes {
		names = append(names, a.Name)
	}
	for _, b := range block.BlockTypes {
		names = append(names, b.TypeName)
	}
	sort.Strings(names)
	return names
}

func tfsdkFields(model any) []string {
	var names []string
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag, ok := f.Tag.Lookup("tfsdk")
			if ok {
				names = append(names, tag)
				continue
			}
			// Embedded structs contribute their own fields.
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				walk(f.Type)
			}
		}
	}
	walk(reflect.TypeOf(model))
	sort.Strings(names)
	return names
}

func compareFields(t *testing.T, schemaAttrs, modelFields []string) {
	t.Helper()

	inSchema := map[string]bool{}
	for _, a := range schemaAttrs {
		inSchema[a] = true
	}
	inModel := map[string]bool{}
	for _, f := range modelFields {
		inModel[f] = true
	}

	for _, a := range schemaAttrs {
		if !inModel[a] {
			t.Errorf("schema attribute %q has no tfsdk field on the model", a)
		}
	}
	for _, f := range modelFields {
		if !inSchema[f] {
			t.Errorf("model tfsdk field %q has no schema attribute", f)
		}
	}
}

// firstDiff reports the first differing line, which is far easier to read than
// a full dump of two large schemas.
func firstDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) && i < len(gotLines); i++ {
		if wantLines[i] != gotLines[i] {
			return "first difference at line " + strconv.Itoa(i+1) + ":\n" +
				"  golden: " + wantLines[i] + "\n" +
				"  actual: " + gotLines[i]
		}
	}
	return "golden has " + strconv.Itoa(len(wantLines)) + " lines, actual has " + strconv.Itoa(len(gotLines))
}
