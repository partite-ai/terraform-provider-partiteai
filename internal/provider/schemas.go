package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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

// Data source nested attribute objects
var memorySlotDataSourceType = dsschema.NestedAttributeObject{
	Attributes: map[string]dsschema.Attribute{
		"name": dsschema.StringAttribute{
			MarkdownDescription: "Memory slot name",
			Computed:            true,
		},
		"description": dsschema.StringAttribute{
			MarkdownDescription: "Memory slot description",
			Computed:            true,
		},
		"memory_type": dsschema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: "Schema defining the structure of data stored in this memory slot",
			Computed:            true,
		},
		"memory_scope": dsschema.StringAttribute{
			MarkdownDescription: "Scope of the memory slot (e.g., 'conversation', 'user')",
			Computed:            true,
		},
		"can_read": dsschema.StringAttribute{
			MarkdownDescription: "Permission level for reading memory",
			Computed:            true,
		},
		"can_write": dsschema.StringAttribute{
			MarkdownDescription: "Permission level for writing memory",
			Computed:            true,
		},
	},
}

var mcpConnectionDataSourceType = dsschema.NestedAttributeObject{
	Attributes: map[string]dsschema.Attribute{
		"id": dsschema.StringAttribute{
			MarkdownDescription: "MCP connection identifier",
			Computed:            true,
		},
		"name": dsschema.StringAttribute{
			MarkdownDescription: "MCP connection name",
			Computed:            true,
		},
	},
}

// More data source nested attribute objects for MCP connections
var envVarDataSourceType = dsschema.NestedAttributeObject{
	Attributes: map[string]dsschema.Attribute{
		"name": dsschema.StringAttribute{
			MarkdownDescription: "The name of the environment variable.",
			Computed:            true,
		},
		"value": dsschema.StringAttribute{
			MarkdownDescription: "The value of the environment variable.",
			Computed:            true,
			Sensitive:           true,
		},
	},
}

var oauthClientCredentialsDataSourceType = dsschema.SingleNestedAttribute{
	MarkdownDescription: "OAuth client credentials for authentication.",
	Computed:            true,
	Attributes: map[string]dsschema.Attribute{
		"client_id": dsschema.StringAttribute{
			MarkdownDescription: "The client ID for OAuth authentication.",
			Computed:            true,
		},
		"client_secret": dsschema.StringAttribute{
			MarkdownDescription: "The client secret for OAuth authentication.",
			Computed:            true,
			Sensitive:           true,
		},
		"scopes": dsschema.ListAttribute{
			MarkdownDescription: "The scopes for OAuth authentication.",
			ElementType:         types.StringType,
			Computed:            true,
		},
	},
}

var basicAuthCredentialsDataSourceType = dsschema.SingleNestedAttribute{
	MarkdownDescription: "Basic authentication credentials.",
	Computed:            true,
	Attributes: map[string]dsschema.Attribute{
		"username": dsschema.StringAttribute{
			MarkdownDescription: "The username for basic authentication.",
			Computed:            true,
		},
		"password": dsschema.StringAttribute{
			MarkdownDescription: "The password for basic authentication.",
			Computed:            true,
			Sensitive:           true,
		},
	},
}

var headerAuthCredentialsDataSourceType = dsschema.SingleNestedAttribute{
	MarkdownDescription: "Header authentication credentials.",
	Computed:            true,
	Attributes: map[string]dsschema.Attribute{
		"name": dsschema.StringAttribute{
			MarkdownDescription: "The name of the header.",
			Computed:            true,
		},
		"value": dsschema.StringAttribute{
			MarkdownDescription: "The value of the header.",
			Computed:            true,
			Sensitive:           true,
		},
	},
}

// Data source schemas - similar to resource schemas but with required ID and all computed
var commonDataSourceSchemas = struct {
	id          dsschema.StringAttribute
	name        dsschema.StringAttribute
	description dsschema.StringAttribute
	createdAt   dsschema.StringAttribute
	updatedAt   dsschema.StringAttribute
	tagsField   dsschema.ListNestedAttribute
}{
	id: dsschema.StringAttribute{
		Required:            true,
		MarkdownDescription: "Resource identifier",
	},
	name: dsschema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Resource name",
	},
	description: dsschema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Resource description",
	},
	createdAt: dsschema.StringAttribute{
		CustomType:          timetypes.RFC3339Type{},
		Computed:            true,
		MarkdownDescription: "Timestamp when the resource was created",
	},
	updatedAt: dsschema.StringAttribute{
		CustomType:          timetypes.RFC3339Type{},
		Computed:            true,
		MarkdownDescription: "Timestamp when the resource was last updated",
	},
	tagsField: dsschema.ListNestedAttribute{
		Computed: true,
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"name": dsschema.StringAttribute{
					MarkdownDescription: "Tag name",
					Computed:            true,
				},
				"value": dsschema.StringAttribute{
					MarkdownDescription: "Tag value",
					Computed:            true,
				},
			},
		},
		MarkdownDescription: "Tags for the resource",
	},
}

// standardDataSourceSchemaAttributes returns a data source schema with the common attributes merged with the provided ones
func standardDataSourceSchemaAttributes(attrs map[string]dsschema.Attribute) map[string]dsschema.Attribute {
	mergedAttrs := map[string]dsschema.Attribute{
		"id":          commonDataSourceSchemas.id,
		"name":        commonDataSourceSchemas.name,
		"description": commonDataSourceSchemas.description,
		"tags":        commonDataSourceSchemas.tagsField,
		"created_at":  commonDataSourceSchemas.createdAt,
		"updated_at":  commonDataSourceSchemas.updatedAt,
	}

	for k, v := range attrs {
		mergedAttrs[k] = v
	}

	return mergedAttrs
}
