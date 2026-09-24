package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	pgql "github.com/partite-ai/terraform-provider-partiteai/internal/graphql"
)

// Judge metrics — whether an author wrote them or they came with the product —
// share the scoring examples and input fields below.

// scoreExampleInfo is one worked example shown to the judging model.
type scoreExampleInfo struct {
	Rating   types.Int64  `tfsdk:"rating"`
	Scenario types.String `tfsdk:"scenario"`
	Judgment types.String `tfsdk:"judgment"`
}

func scoreExampleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"rating":   types.Int64Type,
		"scenario": types.StringType,
		"judgment": types.StringType,
	}
}

// judgeInputFieldInfo is one value a rubric supplies to the metric when it runs.
type judgeInputFieldInfo struct {
	Name        types.String `tfsdk:"name"`
	Label       types.String `tfsdk:"label"`
	Description types.String `tfsdk:"description"`
	Type        types.String `tfsdk:"type"`
	Required    types.Bool   `tfsdk:"required"`
}

func judgeInputFieldAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":        types.StringType,
		"label":       types.StringType,
		"description": types.StringType,
		"type":        types.StringType,
		"required":    types.BoolType,
	}
}

var judgeInputFieldTypes = []string{"STRING", "NUMBER", "BOOLEAN", "STRING_LIST", "JSON_OBJECT"}

var scoreExampleResourceObject = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"rating": schema.Int64Attribute{
			MarkdownDescription: "Score this example earns.",
			Required:            true,
		},
		"scenario": schema.StringAttribute{
			MarkdownDescription: "What happened, as the judge would see it.",
			Required:            true,
		},
		"judgment": schema.StringAttribute{
			MarkdownDescription: "Why that scenario earns that rating.",
			Required:            true,
		},
	},
}

var judgeInputFieldResourceObject = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Key a rubric supplies this value under.",
			Required:            true,
		},
		"label": schema.StringAttribute{
			MarkdownDescription: "Name shown to a person filling it in.",
			Required:            true,
		},
		"description": schema.StringAttribute{
			MarkdownDescription: "What the value is for.",
			Required:            true,
		},
		"type": schema.StringAttribute{
			MarkdownDescription: "One of `STRING`, `NUMBER`, `BOOLEAN`, `STRING_LIST` or `JSON_OBJECT`.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf(judgeInputFieldTypes...),
			},
		},
		"required": schema.BoolAttribute{
			MarkdownDescription: "Whether a rubric has to supply it.",
			Required:            true,
		},
	},
}

var scoreExampleDataSourceObject = dsschema.NestedAttributeObject{
	Attributes: map[string]dsschema.Attribute{
		"rating":   dsschema.Int64Attribute{Computed: true},
		"scenario": dsschema.StringAttribute{Computed: true},
		"judgment": dsschema.StringAttribute{Computed: true},
	},
}

var judgeInputFieldDataSourceObject = dsschema.NestedAttributeObject{
	Attributes: map[string]dsschema.Attribute{
		"name":        dsschema.StringAttribute{Computed: true},
		"label":       dsschema.StringAttribute{Computed: true},
		"description": dsschema.StringAttribute{Computed: true},
		"type":        dsschema.StringAttribute{Computed: true},
		"required":    dsschema.BoolAttribute{Computed: true},
	},
}

// scoreExample reads one example from any of the generated shapes that carry
// the same three fields.
type scoreExampleSource interface {
	GetRating() int
	GetScenario() string
	GetJudgment() string
}

type judgeInputFieldSource interface {
	GetName() string
	GetLabel() string
	GetDescription() string
	GetType() pgql.JudgeInputFieldType
	GetRequired() bool
}

// The generated accessors hang off pointers, so each helper takes the value
// type and its pointer, the same way the mapping helpers in helpers.go do.

func scoreExampleList[S any, PS interface {
	*S
	scoreExampleSource
}](ctx context.Context, src []S, diags *diag.Diagnostics) types.List {
	out := make([]scoreExampleInfo, 0, len(src))
	for i := range src {
		e := PS(&src[i])
		out = append(out, scoreExampleInfo{
			Rating:   types.Int64Value(int64(e.GetRating())),
			Scenario: types.StringValue(e.GetScenario()),
			Judgment: types.StringValue(e.GetJudgment()),
		})
	}
	list, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: scoreExampleAttrTypes()}, out)
	diags.Append(d...)
	return list
}

func judgeInputFieldList[S any, PS interface {
	*S
	judgeInputFieldSource
}](ctx context.Context, src []S, diags *diag.Diagnostics) types.List {
	out := make([]judgeInputFieldInfo, 0, len(src))
	for i := range src {
		f := PS(&src[i])
		out = append(out, judgeInputFieldInfo{
			Name:        types.StringValue(f.GetName()),
			Label:       types.StringValue(f.GetLabel()),
			Description: types.StringValue(f.GetDescription()),
			Type:        types.StringValue(string(f.GetType())),
			Required:    types.BoolValue(f.GetRequired()),
		})
	}
	list, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: judgeInputFieldAttrTypes()}, out)
	diags.Append(d...)
	return list
}
