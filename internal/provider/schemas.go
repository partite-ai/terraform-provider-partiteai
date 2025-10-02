package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var tagObjectType = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Tag name",
			Required:            true,
		},
		"value": schema.StringAttribute{
			MarkdownDescription: "Tag value",
			Required:            true,
		},
	},
}

// commonSchemas provides reusable schema components
var commonSchemas = struct {
	id          schema.StringAttribute
	name        schema.StringAttribute
	description schema.StringAttribute
	createdAt   schema.StringAttribute
	updatedAt   schema.StringAttribute
	tagsField   schema.ListNestedAttribute
}{
	id: schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Resource identifier",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	},
	name: schema.StringAttribute{
		MarkdownDescription: "Resource name",
		Required:            true,
	},
	description: schema.StringAttribute{
		MarkdownDescription: "Resource description",
		Optional:            true,
	},
	createdAt: schema.StringAttribute{
		CustomType:          timetypes.RFC3339Type{},
		Computed:            true,
		MarkdownDescription: "Timestamp when the resource was created",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	},
	updatedAt: schema.StringAttribute{
		CustomType:          timetypes.RFC3339Type{},
		Computed:            true,
		MarkdownDescription: "Timestamp when the resource was last updated",
	},
	tagsField: schema.ListNestedAttribute{
		Optional:            true,
		Computed:            true,
		Default:             listdefault.StaticValue(types.ListValueMust(tagObjectType.Type(), nil)),
		NestedObject:        tagObjectType,
		MarkdownDescription: "Tags for the resource",
	},
}

// standardSchemaAttributes returns a schema with the common attributes merged with the provided ones
func standardSchemaAttributes(attrs map[string]schema.Attribute) map[string]schema.Attribute {
	mergedAttrs := map[string]schema.Attribute{
		"id":          commonSchemas.id,
		"name":        commonSchemas.name,
		"description": commonSchemas.description,
		"tags":        commonSchemas.tagsField,
		"created_at":  commonSchemas.createdAt,
		"updated_at":  commonSchemas.updatedAt,
	}

	for k, v := range attrs {
		mergedAttrs[k] = v
	}

	return mergedAttrs
}
