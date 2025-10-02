package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// int64Ptr converts an int pointer to an int64 pointer
func int64Ptr(i *int) *int64 {
	if i == nil {
		return nil
	}
	v := int64(*i)
	return &v
}

// handleAPIError provides standardized error handling for GraphQL API calls
func handleAPIError(diags *diag.Diagnostics, err error, operation string) {
	diags.AddError("API Error", fmt.Sprintf("Unable to %s, got error: %s", operation, err))
}

// handleAPIErrorWithDetails provides detailed error handling with context
func handleAPIErrorWithDetails(diags *diag.Diagnostics, err error, operation, resourceType, resourceID string) {
	diags.AddError(
		"API Error",
		fmt.Sprintf("Unable to %s %s (ID: %s), got error: %s", operation, resourceType, resourceID, err),
	)
}

// SetCommonFields sets common fields (id, name, description, timestamps) on a resource model
func setFields(
	ctx context.Context,
	fields ...any,
) diag.Diagnostics {
	if len(fields)%2 != 0 {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Internal Error", "setFields requires an even number of arguments")}
	}
	var diags diag.Diagnostics
	for i := 0; i < len(fields); i += 2 {
		if srcMap, ok := fields[i+1].(tfMappable); ok {
			diags.Append(srcMap.mapToTF(ctx, fields[i].(tfTypeable))...)
			continue
		}
		switch v := fields[i].(type) {
		case *types.String:
			diags.Append(setStringField(ctx, v, fields[i+1])...)
		case *timetypes.RFC3339:
			diags.Append(setTimestampField(ctx, v, fields[i+1])...)
		case *jsontypes.Normalized:
			diags.Append(setJsonField(ctx, v, fields[i+1])...)
		case *types.Int64:
			diags.Append(setInt64Field(ctx, v, fields[i+1])...)
		case *types.List:
			diags.Append(setListField(ctx, v, fields[i+1])...)
		case *types.Set:
			diags.Append(setSetField(ctx, v, fields[i+1])...)
		case *types.Object:
			diags.Append(setObjectField(ctx, v, fields[i+1])...)
		case *types.Bool:
			diags.Append(setBoolField(ctx, v, fields[i+1])...)
		default:
			diags.AddError("Internal Error", fmt.Sprintf("Unsupported field type: %T", v))
		}
	}
	return diags
}

func setStringField(ctx context.Context, field *types.String, value any) diag.Diagnostics {
	var diags diag.Diagnostics
	switch v := value.(type) {
	case string:
		*field = types.StringValue(v)
	case *string:
		*field = types.StringPointerValue(v)
	default:
		diags.AddError("Internal Error", fmt.Sprintf("Unsupported string value type: %T", v))
	}
	return diags
}

func setTimestampField(ctx context.Context, field *timetypes.RFC3339, value any) diag.Diagnostics {
	var diags diag.Diagnostics
	switch v := value.(type) {
	case string:
		*field, diags = timetypes.NewRFC3339Value(v)
	case *string:
		*field, diags = timetypes.NewRFC3339PointerValue(v)
	default:
		diags.AddError("Internal Error", fmt.Sprintf("Unsupported timestamp value type: %T", v))
	}
	return diags
}

func setInt64Field(ctx context.Context, field *types.Int64, value any) diag.Diagnostics {
	var diags diag.Diagnostics
	switch v := value.(type) {
	case int64:
		*field = types.Int64Value(v)
	case int:
		*field = types.Int64Value(int64(v))
	case *int64:
		*field = types.Int64PointerValue(v)
	case *int:
		if v == nil {
			*field = types.Int64Null()
		} else {
			*field = types.Int64Value(int64(*v))
		}
	default:
		diags.AddError("Internal Error", fmt.Sprintf("Unsupported int64 value type: %T", v))
	}
	return diags
}

func setListField(ctx context.Context, field *types.List, value any) diag.Diagnostics {
	listVal, diags := types.ListValueFrom(ctx, field.ElementType(ctx), value)
	if diags.HasError() {
		return diags
	}
	*field = listVal
	return diags
}

func setSetField(ctx context.Context, field *types.Set, value any) diag.Diagnostics {
	setVal, diags := types.SetValueFrom(ctx, field.ElementType(ctx), value)
	if diags.HasError() {
		return diags
	}
	*field = setVal
	return diags
}

func setJsonField(ctx context.Context, field *jsontypes.Normalized, value any) diag.Diagnostics {
	var diags diag.Diagnostics
	switch v := value.(type) {
	case string:
		jsonVal := jsontypes.NewNormalizedValue(v)
		*field = jsonVal
	case *string:
		jsonVal := jsontypes.NewNormalizedPointerValue(v)
		*field = jsonVal
	case map[string]any:
		seriVal, err := json.Marshal(v)
		if err != nil {
			diags.AddError("Internal Error", fmt.Sprintf("Failed to marshal JSON map: %s", err))
			return diags
		}
		jsonVal := jsontypes.NewNormalizedValue(string(seriVal))
		*field = jsonVal
	default:
		diags.AddError("Internal Error", fmt.Sprintf("Unsupported JSON value type: %T", v))
	}
	return diags
}

func setObjectField(ctx context.Context, field *types.Object, value any) diag.Diagnostics {
	objVal, diags := types.ObjectValueFrom(ctx, field.AttributeTypes(ctx), value)
	if diags.HasError() {
		return diags
	}
	*field = objVal
	return diags
}

func setBoolField(ctx context.Context, field *types.Bool, value any) diag.Diagnostics {
	var diags diag.Diagnostics
	switch v := value.(type) {
	case bool:
		*field = types.BoolValue(v)
	case *bool:
		*field = types.BoolPointerValue(v)
	default:
		diags.AddError("Internal Error", fmt.Sprintf("Unsupported bool value type: %T", v))
	}
	return diags
}

// logResourceOperation logs a resource operation for debugging
func logResourceOperation(ctx context.Context, operation, resourceType, resourceID string) {
	tflog.Debug(ctx, fmt.Sprintf("%s %s", operation, resourceType),
		map[string]interface{}{
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"operation":     operation,
		})
}

type tfTypeable interface {
	Type(ctx context.Context) attr.Type
}
type tfMappable interface {
	mapToTF(ctx context.Context, target tfTypeable) diag.Diagnostics
}

type fromGQL[T any] interface {
	FromGQL(src T) diag.Diagnostics
}

type structSliceMappable[S any, T any, PT interface {
	*T
	fromGQL[S]
}] struct {
	src []S
}

func (l structSliceMappable[S, T, PT]) mapToTF(ctx context.Context, target tfTypeable) diag.Diagnostics {
	var diags diag.Diagnostics

	converted := make([]PT, len(l.src))
	for i, s := range l.src {
		var t PT = new(T)
		diags.Append(t.FromGQL(s)...)
		converted[i] = t
	}

	if diags.HasError() {
		return diags
	}

	return tfsdk.ValueFrom(ctx, converted, target.Type(ctx), target)
}

func newStructSliceMappable[S any, T any, PT interface {
	*T
	fromGQL[S]
}](src []S, targetExample T) tfMappable {
	return structSliceMappable[S, T, PT]{src: src}
}

type structMappable[S any, T any, PT interface {
	*T
	fromGQL[S]
}] struct {
	src S
}

func (l structMappable[S, T, PT]) mapToTF(ctx context.Context, target tfTypeable) diag.Diagnostics {
	var diags diag.Diagnostics
	var t T
	diags.Append(PT(&t).FromGQL(l.src)...)
	return tfsdk.ValueFrom(ctx, t, target.Type(ctx), target)
}

func newStructMappable[S any, T any, PT interface {
	*T
	fromGQL[S]
}](src S, targetExample T) tfMappable {
	return structMappable[S, T, PT]{src: src}
}
