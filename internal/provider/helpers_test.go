package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// setFields is the seam every resource's read path runs through, so the null,
// empty and pointer cases are worth pinning directly. A mistake here shows up
// as a perpetual diff in whichever resource happens to use that type.
func TestSetFields_scalars(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	str := "hello"
	num := 7
	yes := true

	var (
		gotString    types.String
		gotStringPtr types.String
		gotNilString types.String
		gotInt       types.Int64
		gotIntPtr    types.Int64
		gotNilInt    types.Int64
		gotBool      types.Bool
		gotNilBool   types.Bool
	)

	var nilStr *string
	var nilInt *int
	var nilBool *bool

	diags := setFields(ctx,
		&gotString, str,
		&gotStringPtr, &str,
		&gotNilString, nilStr,
		&gotInt, num,
		&gotIntPtr, &num,
		&gotNilInt, nilInt,
		&gotBool, yes,
		&gotNilBool, nilBool,
	)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	cases := []struct {
		name string
		got  attr.Value
		want attr.Value
	}{
		{"string", gotString, types.StringValue("hello")},
		{"string pointer", gotStringPtr, types.StringValue("hello")},
		{"nil string pointer", gotNilString, types.StringNull()},
		{"int", gotInt, types.Int64Value(7)},
		{"int pointer", gotIntPtr, types.Int64Value(7)},
		{"nil int pointer", gotNilInt, types.Int64Null()},
		{"bool", gotBool, types.BoolValue(true)},
		{"nil bool pointer", gotNilBool, types.BoolNull()},
	}
	for _, c := range cases {
		if !c.got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestSetFields_listsDistinguishNullFromEmpty pins the difference that produced
// real bugs in this provider: a nil source slice becomes a null list, and an
// empty-but-not-nil slice becomes an empty list. A schema whose attribute
// defaults to an empty list needs the second; an Optional-only attribute needs
// the first.
func TestSetFields_listsDistinguishNullFromEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fromNil := types.ListNull(types.StringType)
	fromEmpty := types.ListNull(types.StringType)

	var nilSlice []string

	diags := setFields(ctx,
		&fromNil, nilSlice,
		&fromEmpty, []string{},
	)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !fromNil.IsNull() {
		t.Errorf("a nil slice should map to a null list, got %v", fromNil)
	}
	if fromEmpty.IsNull() {
		t.Error("an empty slice should map to an empty list, not null")
	}
	if got := len(fromEmpty.Elements()); got != 0 {
		t.Errorf("empty list has %d elements", got)
	}
}

func TestSetFields_timestamps(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var ok timetypes.RFC3339
	if diags := setFields(ctx, &ok, "2026-09-23T12:00:00Z"); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if ok.IsNull() || ok.ValueString() != "2026-09-23T12:00:00Z" {
		t.Errorf("got %v", ok)
	}

	var bad timetypes.RFC3339
	if diags := setFields(ctx, &bad, "not a time"); !diags.HasError() {
		t.Error("an unparseable timestamp should produce a diagnostic")
	}
}

func TestSetFields_json(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var fromString jsontypes.Normalized
	var fromMap jsontypes.Normalized

	diags := setFields(ctx,
		&fromString, `{"b":2,"a":1}`,
		&fromMap, map[string]any{"a": 1, "b": 2},
	)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// Normalized JSON compares by meaning, so key order must not matter.
	equal, d := fromString.StringSemanticEquals(ctx, fromMap)
	if d.HasError() {
		t.Fatalf("comparing: %v", d)
	}
	if !equal {
		t.Errorf("%s and %s should compare equal", fromString, fromMap)
	}
}

// TestSetFields_rejectsOddArguments guards the pairing contract, since a
// mismatch is a silent mapping bug otherwise.
func TestSetFields_rejectsOddArguments(t *testing.T) {
	t.Parallel()

	var s types.String
	if diags := setFields(context.Background(), &s); !diags.HasError() {
		t.Error("an odd number of arguments should produce a diagnostic")
	}
}
