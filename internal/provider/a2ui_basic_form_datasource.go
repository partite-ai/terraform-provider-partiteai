package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/partite-ai/terraform-provider-partiteai/internal/a2ui"
)

var _ datasource.DataSource = &A2UIBasicFormDataSource{}

func NewA2UIBasicFormDataSource() datasource.DataSource {
	return &A2UIBasicFormDataSource{
		BaseDataSource: NewBaseDataSource("a2ui_basic_form"),
	}
}

// A2UIBasicFormDataSource builds the definition of an A2UI task form from
// components of the basic catalog, with a block for each component type.
type A2UIBasicFormDataSource struct {
	BaseDataSource
}

// A2UIBasicFormDataSourceModel holds the surfaces as objects, since their
// shape comes from the catalog.
type A2UIBasicFormDataSourceModel struct {
	Open       types.Object `tfsdk:"open"`
	Completed  types.Object `tfsdk:"completed"`
	Definition types.String `tfsdk:"definition"`
}

func (d *A2UIBasicFormDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	components, err := a2ui.BasicCatalogComponents()
	if err != nil {
		resp.Diagnostics.AddError("Invalid A2UI basic catalog", err.Error())
		return
	}
	componentBlock := a2uiBasicComponentBlock(components)
	resp.Schema = schema.Schema{
		MarkdownDescription: "Builds the definition of an A2UI task form from components of the A2UI v0.9 basic catalog, " +
			"for the `A2UI` entry of a task definition's `ui_forms`. Each component is a `component` block with one " +
			"block for its type, such as `text` or `button`, whose attributes are the type's properties in snake case. " +
			"A property that can be bound to the data model or come from a function call has `_path` and `_call` " +
			"forms beside it: set `text = \"Hi\"`, `text_path = \"/greeting\"`, or " +
			"`text_call = jsonencode({ call = \"formatString\", args = { value = \"Hi $${/name}\" } })`. " +
			"The form is checked against the catalog when it is read. Checks that need the task's schemas, such as " +
			"whether an action's context fits the output, happen when the task definition is saved. " +
			"For a custom catalog, use `partiteai_a2ui_form`.",
		Attributes: map[string]schema.Attribute{
			"definition": schema.StringAttribute{
				MarkdownDescription: "The form's definition, as JSON. Use it as the `A2UI` entry of a task definition's `ui_forms`.",
				Computed:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"open": schema.SingleNestedBlock{
				MarkdownDescription: "The surface shown while the task is open. Its data model starts as the task input, " +
					"and every event action completes the task with its context as the output. Required.",
				Blocks:     map[string]schema.Block{"component": componentBlock},
				Validators: []validator.Object{objectvalidator.IsRequired()},
			},
			"completed": schema.SingleNestedBlock{
				MarkdownDescription: "The surface that replaces the open one once the task is completed, with the task " +
					"output as its data model. It can't have event actions. Without it, the surface is removed once the " +
					"task is completed.",
				Blocks: map[string]schema.Block{"component": componentBlock},
			},
		},
	}
}

func (d *A2UIBasicFormDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data A2UIBasicFormDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	components, err := a2ui.BasicCatalogComponents()
	if err != nil {
		resp.Diagnostics.AddError("Invalid A2UI basic catalog", err.Error())
		return
	}

	b := &a2uiBasicFormBuilder{components: components}
	form := &a2ui.Form{Open: b.surface(data.Open, path.Root("open"))}
	if !data.Completed.IsNull() {
		form.Completed = b.surface(data.Completed, path.Root("completed"))
	}
	resp.Diagnostics.Append(b.diags...)
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

// checkedA2UIDefinition checks a form and returns its definition.
func checkedA2UIDefinition(diags *diag.Diagnostics, form *a2ui.Form) (string, bool) {
	if err := form.Check(); err != nil {
		diags.AddError("Invalid A2UI form", err.Error())
		return "", false
	}
	b, err := json.Marshal(form)
	if err != nil {
		diags.AddError("Invalid A2UI form", err.Error())
		return "", false
	}
	return string(b), true
}

var a2uiPointerPattern = regexp.MustCompile(`^/`)

// a2uiOverlayBlock is the overlay block of a component, in both A2UI form data
// sources.
var a2uiOverlayBlock = schema.ListNestedBlock{
	MarkdownDescription: "Sets a value of the component from the task input when the task starts.",
	NestedObject: schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			"pointer": schema.StringAttribute{
				MarkdownDescription: "A JSON pointer to the value in the component as A2UI writes it, with property " +
					"names in camel case, such as `/text` or `/action/event/context/answer`.",
				Required:   true,
				Validators: []validator.String{stringvalidator.RegexMatches(a2uiPointerPattern, "must start with /")},
			},
			"expression": schema.StringAttribute{
				MarkdownDescription: "A CEL expression that sees the task input as `input`, such as " +
					"`input.options.map(o, {'label': o, 'value': o})`. An empty optional, such as `input.?subtitle`, " +
					"removes the value.",
				Required: true,
			},
		},
	},
}

func a2uiBasicComponentBlock(components *a2ui.BasicComponents) schema.ListNestedBlock {
	attributes, blocks := a2uiPropertySchema(components.Common, true)
	for _, c := range components.Components {
		typeAttributes, typeBlocks := a2uiPropertySchema(c.Properties, false)
		description := c.Description
		if description == "" {
			description = "The " + c.JSONName + " component."
		}
		blocks[c.Name] = schema.SingleNestedBlock{
			MarkdownDescription: description + " Set exactly one component type block.",
			Attributes:          typeAttributes,
			Blocks:              typeBlocks,
		}
	}
	blocks["overlay"] = a2uiOverlayBlock
	return schema.ListNestedBlock{
		MarkdownDescription: "A component of the surface. One of them must have the ID `root`.",
		NestedObject:        schema.NestedBlockObject{Attributes: attributes, Blocks: blocks},
	}
}

// a2uiPropertySchema returns the attributes and blocks that hold properties.
// Terraform requires a required attribute even when the single block it is in
// is left out, so only an attribute that is always present, such as one in a
// list block, can be required. The builder checks the others.
func a2uiPropertySchema(properties []a2ui.Property, alwaysPresent bool) (map[string]schema.Attribute, map[string]schema.Block) {
	attributes := map[string]schema.Attribute{}
	blocks := map[string]schema.Block{}
	for _, p := range properties {
		required := p.Required && alwaysPresent
		description := p.Description
		if p.Required && !alwaysPresent {
			description += " Required."
		}
		if description != "" && !strings.HasSuffix(description, ".") {
			description += "."
		}
		dynamic := func(literal schema.Attribute) {
			attributes[p.Name] = literal
			attributes[p.Name+"_path"] = schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("Binds `%s` to a JSON pointer in the data model.", p.Name),
				Optional:            true,
			}
			attributes[p.Name+"_call"] = schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: fmt.Sprintf("Sets `%s` to a function call, as JSON: `{\"call\": ..., \"args\": {...}}`.", p.Name),
				Optional:            true,
			}
		}
		oneOf := fmt.Sprintf(" Set at most one of `%[1]s`, `%[1]s_path` and `%[1]s_call`.", p.Name)
		if p.Required {
			oneOf = fmt.Sprintf(" Set exactly one of `%[1]s`, `%[1]s_path` and `%[1]s_call`.", p.Name)
		}

		switch p.Kind {
		case a2ui.KindString:
			a := schema.StringAttribute{MarkdownDescription: description, Required: required, Optional: !required}
			if len(p.Enum) > 0 {
				a.Validators = []validator.String{stringvalidator.OneOf(p.Enum...)}
			}
			attributes[p.Name] = a
		case a2ui.KindNumber:
			attributes[p.Name] = schema.Float64Attribute{MarkdownDescription: description, Required: required, Optional: !required}
		case a2ui.KindBool:
			attributes[p.Name] = schema.BoolAttribute{MarkdownDescription: description, Required: required, Optional: !required}
		case a2ui.KindStringList:
			attributes[p.Name] = schema.ListAttribute{ElementType: types.StringType, MarkdownDescription: description, Required: required, Optional: !required}
		case a2ui.KindDynamicString:
			dynamic(schema.StringAttribute{MarkdownDescription: description + oneOf, Optional: true})
		case a2ui.KindDynamicNumber:
			dynamic(schema.Float64Attribute{MarkdownDescription: description + oneOf, Optional: true})
		case a2ui.KindDynamicBool:
			dynamic(schema.BoolAttribute{MarkdownDescription: description + oneOf, Optional: true})
		case a2ui.KindDynamicStringList:
			dynamic(schema.ListAttribute{ElementType: types.StringType, MarkdownDescription: description + oneOf, Optional: true})
		case a2ui.KindChildList:
			attributes[p.Name] = schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "The IDs of the child components. Set this or a `" + p.Name + "_template` block.",
				Optional:            true,
			}
			blocks[p.Name+"_template"] = schema.SingleNestedBlock{
				MarkdownDescription: "Makes a child for each item of a list in the data model. Set this or `" + p.Name + "`.",
				Attributes: map[string]schema.Attribute{
					"component_id": schema.StringAttribute{MarkdownDescription: "The component to use as a template. Required.", Optional: true},
					"path":         schema.StringAttribute{MarkdownDescription: "The path to the list in the data model. Required.", Optional: true},
				},
			}
		case a2ui.KindIconName:
			attributes[p.Name] = schema.StringAttribute{
				MarkdownDescription: "The name of the icon. Set exactly one of `" + p.Name + "`, `svg_path` and `" + p.Name + "_path`.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf(p.Enum...)},
			}
			attributes["svg_path"] = schema.StringAttribute{MarkdownDescription: "An SVG path to draw as the icon.", Optional: true}
			attributes[p.Name+"_path"] = schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("Binds `%s` to a JSON pointer in the data model.", p.Name),
				Optional:            true,
			}
		case a2ui.KindAction:
			blocks[p.Name] = schema.SingleNestedBlock{
				MarkdownDescription: "What the component does. Set an `event` block or `function_call`.",
				Attributes: map[string]schema.Attribute{
					"function_call": schema.StringAttribute{
						CustomType:          jsontypes.NormalizedType{},
						MarkdownDescription: "Runs a function on the client, as JSON: `{\"call\": ..., \"args\": {...}}`.",
						Optional:            true,
					},
				},
				Blocks: map[string]schema.Block{
					"event": schema.SingleNestedBlock{
						MarkdownDescription: "Completes the task, with the context as its output.",
						Attributes: map[string]schema.Attribute{
							"name": schema.StringAttribute{MarkdownDescription: "The name of the event. Required.", Optional: true},
							"context": schema.StringAttribute{
								CustomType: jsontypes.NormalizedType{},
								MarkdownDescription: "The task output, as a JSON object. A value can be a literal or a binding, " +
									"such as `jsonencode({ confirmed = true, note = { path = \"/note\" } })`.",
								Optional: true,
							},
						},
					},
				},
			}
		case a2ui.KindObject:
			nestedAttributes, nestedBlocks := a2uiPropertySchema(p.Properties, false)
			blocks[p.Name] = schema.SingleNestedBlock{MarkdownDescription: description, Attributes: nestedAttributes, Blocks: nestedBlocks}
		case a2ui.KindObjectList:
			nestedAttributes, nestedBlocks := a2uiPropertySchema(p.Properties, true)
			blocks[p.BlockName()] = schema.ListNestedBlock{
				MarkdownDescription: description,
				NestedObject:        schema.NestedBlockObject{Attributes: nestedAttributes, Blocks: nestedBlocks},
			}
		}
	}
	return attributes, blocks
}

// a2uiBasicFormBuilder turns the configuration of a basic form into the form.
type a2uiBasicFormBuilder struct {
	components *a2ui.BasicComponents
	diags      diag.Diagnostics
}

func (b *a2uiBasicFormBuilder) surface(surface types.Object, p path.Path) *a2ui.Surface {
	s := &a2ui.Surface{}
	list, _ := surface.Attributes()["component"].(types.List)
	for i, element := range list.Elements() {
		cp := p.AtName("component").AtListIndex(i)
		attributes := a2uiAttributes(element)
		component := map[string]any{}
		b.properties(b.components.Common, attributes, cp, component)

		var typeNames []string
		for _, c := range b.components.Components {
			typeBlock, _ := attributes[c.Name].(types.Object)
			if typeBlock.IsNull() {
				continue
			}
			typeNames = append(typeNames, c.Name)
			component["component"] = c.JSONName
			b.properties(c.Properties, typeBlock.Attributes(), cp.AtName(c.Name), component)
		}
		if len(typeNames) != 1 {
			b.diags.AddAttributeError(cp, "Invalid A2UI component",
				fmt.Sprintf("Set exactly one component type block, such as text or button; this one has %d: %s.", len(typeNames), strings.Join(typeNames, ", ")))
			continue
		}

		b.overlays(s, a2uiString(attributes["id"]), attributes["overlay"], cp)
		s.Components = append(s.Components, component)
	}
	return s
}

// overlays adds a component's overlay blocks to its surface.
func (b *a2uiBasicFormBuilder) overlays(s *a2ui.Surface, id string, value attr.Value, p path.Path) {
	list, _ := value.(types.List)
	for i, element := range list.Elements() {
		attributes := a2uiAttributes(element)
		addA2UIOverlay(&b.diags, s, id,
			a2uiString(attributes["pointer"]),
			a2uiString(attributes["expression"]),
			p.AtName("overlay").AtListIndex(i))
	}
}

// properties writes the configured properties into out, by their JSON names.
func (b *a2uiBasicFormBuilder) properties(properties []a2ui.Property, attributes map[string]attr.Value, p path.Path, out map[string]any) {
	for _, prop := range properties {
		switch prop.Kind {
		case a2ui.KindString, a2ui.KindNumber, a2ui.KindBool, a2ui.KindStringList:
			if v, ok := a2uiLiteral(attributes[prop.Name]); ok {
				out[prop.JSONName] = v
			} else if prop.Required {
				b.diags.AddAttributeError(p.AtName(prop.Name), "Missing A2UI property", fmt.Sprintf("Set %s.", prop.Name))
			}
		case a2ui.KindDynamicString, a2ui.KindDynamicNumber, a2ui.KindDynamicBool, a2ui.KindDynamicStringList:
			b.oneOf(prop, p, out, map[string]func() (any, bool){
				prop.Name:           func() (any, bool) { return a2uiLiteral(attributes[prop.Name]) },
				prop.Name + "_path": func() (any, bool) { return a2uiBinding(attributes[prop.Name+"_path"]) },
				prop.Name + "_call": func() (any, bool) { return b.json(attributes[prop.Name+"_call"], p.AtName(prop.Name+"_call")) },
			})
		case a2ui.KindChildList:
			b.oneOf(prop, p, out, map[string]func() (any, bool){
				prop.Name: func() (any, bool) { return a2uiLiteral(attributes[prop.Name]) },
				prop.Name + "_template": func() (any, bool) {
					o, _ := attributes[prop.Name+"_template"].(types.Object)
					if o.IsNull() {
						return nil, false
					}
					template := map[string]any{}
					tp := p.AtName(prop.Name + "_template")
					b.properties([]a2ui.Property{
						{JSONName: "componentId", Name: "component_id", Kind: a2ui.KindString, Required: true},
						{JSONName: "path", Name: "path", Kind: a2ui.KindString, Required: true},
					}, o.Attributes(), tp, template)
					return template, true
				},
			})
		case a2ui.KindIconName:
			b.oneOf(prop, p, out, map[string]func() (any, bool){
				prop.Name: func() (any, bool) { return a2uiLiteral(attributes[prop.Name]) },
				"svg_path": func() (any, bool) {
					v, ok := a2uiLiteral(attributes["svg_path"])
					return map[string]any{"svgPath": v}, ok
				},
				prop.Name + "_path": func() (any, bool) { return a2uiBinding(attributes[prop.Name+"_path"]) },
			})
		case a2ui.KindAction:
			b.action(prop, attributes[prop.Name], p.AtName(prop.Name), out)
		case a2ui.KindObject:
			o, _ := attributes[prop.Name].(types.Object)
			if o.IsNull() {
				if prop.Required {
					b.diags.AddAttributeError(p.AtName(prop.Name), "Missing A2UI property", fmt.Sprintf("Add a %s block.", prop.Name))
				}
				continue
			}
			nested := map[string]any{}
			b.properties(prop.Properties, o.Attributes(), p.AtName(prop.Name), nested)
			out[prop.JSONName] = nested
		case a2ui.KindObjectList:
			list, _ := attributes[prop.BlockName()].(types.List)
			// A required list with no blocks is empty, such as options an
			// overlay sets.
			if len(list.Elements()) == 0 {
				if prop.Required {
					out[prop.JSONName] = []any{}
				}
				continue
			}
			items := make([]any, 0, len(list.Elements()))
			for i, element := range list.Elements() {
				nested := map[string]any{}
				b.properties(prop.Properties, a2uiAttributes(element), p.AtName(prop.BlockName()).AtListIndex(i), nested)
				items = append(items, nested)
			}
			out[prop.JSONName] = items
		}
	}
}

// oneOf sets a property from whichever of its forms is set, and requires
// exactly one when the property is required.
func (b *a2uiBasicFormBuilder) oneOf(prop a2ui.Property, p path.Path, out map[string]any, forms map[string]func() (any, bool)) {
	var set []string
	for _, name := range sortedA2UIKeys(forms) {
		if v, ok := forms[name](); ok {
			set = append(set, name)
			out[prop.JSONName] = v
		}
	}
	names := strings.Join(sortedA2UIKeys(forms), ", ")
	switch {
	case len(set) > 1:
		b.diags.AddAttributeError(p.AtName(set[0]), "Conflicting A2UI property", fmt.Sprintf("Set only one of %s.", names))
	case len(set) == 0 && prop.Required:
		b.diags.AddAttributeError(p.AtName(prop.Name), "Missing A2UI property", fmt.Sprintf("Set one of %s.", names))
	}
}

func (b *a2uiBasicFormBuilder) action(prop a2ui.Property, value attr.Value, p path.Path, out map[string]any) {
	o, _ := value.(types.Object)
	if o.IsNull() {
		if prop.Required {
			b.diags.AddAttributeError(p, "Missing A2UI property", fmt.Sprintf("Add an %s block.", prop.Name))
		}
		return
	}
	attributes := o.Attributes()
	event, _ := attributes["event"].(types.Object)
	call, hasCall := b.json(attributes["function_call"], p.AtName("function_call"))
	switch {
	case !event.IsNull() && hasCall:
		b.diags.AddAttributeError(p, "Conflicting A2UI property", "Set an event block or function_call, not both.")
	case hasCall:
		out[prop.JSONName] = map[string]any{"functionCall": call}
	case !event.IsNull():
		e := event.Attributes()
		name, _ := e["name"].(types.String)
		if name.IsNull() {
			b.diags.AddAttributeError(p.AtName("event").AtName("name"), "Missing A2UI property", "Set the event's name.")
			return
		}
		ev := map[string]any{"name": name.ValueString()}
		if context, ok := b.json(e["context"], p.AtName("event").AtName("context")); ok {
			ev["context"] = context
		}
		out[prop.JSONName] = map[string]any{"event": ev}
	default:
		b.diags.AddAttributeError(p, "Missing A2UI property", "Set an event block or function_call.")
	}
}

// json decodes a JSON attribute.
func (b *a2uiBasicFormBuilder) json(value attr.Value, p path.Path) (any, bool) {
	s, ok := value.(jsontypes.Normalized)
	if !ok || s.IsNull() {
		return nil, false
	}
	var v any
	if err := json.Unmarshal([]byte(s.ValueString()), &v); err != nil {
		b.diags.AddAttributeError(p, "Invalid JSON", err.Error())
		return nil, false
	}
	return v, true
}

// a2uiLiteral returns the JSON value of a string, number, bool or string list
// attribute that is set.
func a2uiLiteral(value attr.Value) (any, bool) {
	if value == nil || value.IsNull() {
		return nil, false
	}
	switch v := value.(type) {
	case types.String:
		return v.ValueString(), true
	case types.Float64:
		return v.ValueFloat64(), true
	case types.Bool:
		return v.ValueBool(), true
	case types.List:
		items := []any{}
		for _, element := range v.Elements() {
			items = append(items, a2uiString(element))
		}
		return items, true
	}
	return nil, false
}

// a2uiBinding returns a binding to the path in a string attribute that is set.
func a2uiBinding(value attr.Value) (any, bool) {
	s, ok := value.(types.String)
	if !ok || s.IsNull() {
		return nil, false
	}
	return map[string]any{"path": s.ValueString()}, true
}

// a2uiAttributes returns the attributes of an object value.
func a2uiAttributes(value attr.Value) map[string]attr.Value {
	o, _ := value.(types.Object)
	return o.Attributes()
}

// a2uiString returns the value of a string attribute.
func a2uiString(value attr.Value) string {
	s, _ := value.(types.String)
	return s.ValueString()
}

func sortedA2UIKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
