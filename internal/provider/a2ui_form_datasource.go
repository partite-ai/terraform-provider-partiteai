package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/partite-ai/terraform-provider-partiteai/internal/a2ui"
)

var _ datasource.DataSource = &A2UIFormDataSource{}

func NewA2UIFormDataSource() datasource.DataSource {
	return &A2UIFormDataSource{
		BaseDataSource: NewBaseDataSource("a2ui_form"),
	}
}

// A2UIFormDataSource builds the definition of an A2UI task form from
// components given as their type and properties, from any catalog.
type A2UIFormDataSource struct {
	BaseDataSource
}

type A2UIFormDataSourceModel struct {
	Catalog    jsontypes.Normalized  `tfsdk:"catalog"`
	Open       *a2uiFormSurfaceModel `tfsdk:"open"`
	Completed  *a2uiFormSurfaceModel `tfsdk:"completed"`
	Definition types.String          `tfsdk:"definition"`
}

type a2uiFormSurfaceModel struct {
	Components []a2uiFormComponentModel `tfsdk:"component"`
}

type a2uiFormComponentModel struct {
	Id         types.String         `tfsdk:"id"`
	Type       types.String         `tfsdk:"type"`
	Properties jsontypes.Normalized `tfsdk:"properties"`
	Overlays   []a2uiOverlayModel   `tfsdk:"overlay"`
}

type a2uiOverlayModel struct {
	Pointer    types.String `tfsdk:"pointer"`
	Expression types.String `tfsdk:"expression"`
}

func (d *A2UIFormDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	surface := func(description string, validators []validator.Object) schema.SingleNestedBlock {
		return schema.SingleNestedBlock{
			MarkdownDescription: description,
			Validators:          validators,
			Blocks: map[string]schema.Block{
				"component": schema.ListNestedBlock{
					MarkdownDescription: "A component of the surface. One of them must have the ID `root`.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								MarkdownDescription: "The component's ID, unique in the surface.",
								Required:            true,
							},
							"type": schema.StringAttribute{
								MarkdownDescription: "The component type, as the catalog names it, such as `Text` or `Button`.",
								Required:            true,
							},
							"properties": schema.StringAttribute{
								CustomType: jsontypes.NormalizedType{},
								MarkdownDescription: "The component's other properties, as a JSON object in the shape the " +
									"catalog gives them, such as `jsonencode({ text = { path = \"/question\" } })`.",
								Optional: true,
							},
						},
						Blocks: map[string]schema.Block{"overlay": a2uiOverlayBlock},
					},
				},
			},
		}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Builds the definition of an A2UI task form, for the `A2UI` entry of a task definition's " +
			"`ui_forms`, from components given as a type and a JSON object of properties. It works with any catalog. " +
			"The form is checked against the catalog when it is read. Checks that need the task's schemas, such as " +
			"whether an action's context fits the output, happen when the task definition is saved. " +
			"For the basic catalog, `partiteai_a2ui_basic_form` has a block for each component type.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				MarkdownDescription: "A full A2UI catalog definition, as JSON. Its `catalogId` is the catalog clients render " +
					"the form with, and every `$ref` in it must point into it. Without it, the components come from the " +
					"A2UI v0.9 basic catalog.",
				Optional: true,
			},
			"definition": schema.StringAttribute{
				MarkdownDescription: "The form's definition, as JSON. Use it as the `A2UI` entry of a task definition's `ui_forms`.",
				Computed:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"open": surface("The surface shown while the task is open. Its data model starts as the task input, "+
				"and every event action completes the task with its context as the output. Required.",
				[]validator.Object{objectvalidator.IsRequired()}),
			"completed": surface("The surface that replaces the open one once the task is completed, with the task "+
				"output as its data model. It can't have event actions. Without it, the surface is removed once the "+
				"task is completed.", nil),
		},
	}
}

func (d *A2UIFormDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data A2UIFormDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	form := &a2ui.Form{}
	if !data.Catalog.IsNull() {
		form.Catalog = json.RawMessage(data.Catalog.ValueString())
	}
	if data.Open != nil {
		form.Open = a2uiFormSurface(&resp.Diagnostics, data.Open, path.Root("open"))
	}
	if data.Completed != nil {
		form.Completed = a2uiFormSurface(&resp.Diagnostics, data.Completed, path.Root("completed"))
	}
	if resp.Diagnostics.HasError() {
		return
	}

	definition, ok := checkedA2UIDefinition(&resp.Diagnostics, form)
	if !ok {
		return
	}
	data.Definition = types.StringValue(definition)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func a2uiFormSurface(diags *diag.Diagnostics, m *a2uiFormSurfaceModel, p path.Path) *a2ui.Surface {
	s := &a2ui.Surface{}
	for i, c := range m.Components {
		cp := p.AtName("component").AtListIndex(i)
		component := map[string]any{}
		if !c.Properties.IsNull() {
			if err := json.Unmarshal([]byte(c.Properties.ValueString()), &component); err != nil {
				diags.AddAttributeError(cp.AtName("properties"), "Invalid A2UI component", "properties must be a JSON object: "+err.Error())
				continue
			}
		}
		for _, key := range []string{"id", "component"} {
			if _, ok := component[key]; ok {
				diags.AddAttributeError(cp.AtName("properties"), "Invalid A2UI component",
					fmt.Sprintf("properties can't have %q; it comes from the component block.", key))
			}
		}
		id := c.Id.ValueString()
		component["id"] = id
		component["component"] = c.Type.ValueString()
		for j, o := range c.Overlays {
			addA2UIOverlay(diags, s, id, o.Pointer.ValueString(), o.Expression.ValueString(), cp.AtName("overlay").AtListIndex(j))
		}
		s.Components = append(s.Components, component)
	}
	return s
}

// addA2UIOverlay adds an overlay of a component to its surface.
func addA2UIOverlay(diags *diag.Diagnostics, s *a2ui.Surface, id, pointer, expression string, p path.Path) {
	if s.Overlays == nil {
		s.Overlays = map[string]map[string]string{}
	}
	if s.Overlays[id] == nil {
		s.Overlays[id] = map[string]string{}
	}
	if _, ok := s.Overlays[id][pointer]; ok {
		diags.AddAttributeError(p, "Invalid A2UI overlay", fmt.Sprintf("Component %q has more than one overlay at %q.", id, pointer))
		return
	}
	s.Overlays[id][pointer] = expression
}
