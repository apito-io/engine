package enums

import (
	"github.com/apito-io/buffers/protobuff"
	"github.com/tailor-inc/graphql"
)

type FieldOperation string

const (
	FieldOperation_Rename    FieldOperation = "rename"
	FieldOperation_Duplicate FieldOperation = "duplicate"
)

var FieldOperationEnums = graphql.NewEnum(graphql.EnumConfig{
	Name: "FIELD_OPERATION_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"rename": &graphql.EnumValueConfig{
			Value:       FieldOperation_Rename,
			Description: "Rename a Field",
		},
		"duplicate": &graphql.EnumValueConfig{
			Value:       FieldOperation_Duplicate,
			Description: "Duplicate a Field",
		},
	},
})

var PluginTypeEnums = graphql.NewEnum(graphql.EnumConfig{
	Name: "PLUGIN_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"extension": &graphql.EnumValueConfig{
			Value:       protobuff.PluginType_NormalPlugin,
			Description: "Extension Type Plugin",
		},
		"function": &graphql.EnumValueConfig{
			Value:       protobuff.PluginType_Function,
			Description: "Function Type Plugin",
		},
		"storage": &graphql.EnumValueConfig{
			Value:       protobuff.PluginType_Storage,
			Description: "Media Type Plugin",
		},
	},
})

var PluginLoadTypeEnums = graphql.NewEnum(graphql.EnumConfig{
	Name: "PLUGIN_LOAD_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"not_installed": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_NotInstalled,
			Description: "Plugin is not Installed",
		},
		"installed": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_Installed,
			Description: "Plugin is is Building",
		},
		"re_install": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_ReInstall,
			Description: "Plugin is Built",
		},
		"loading": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_Loading,
			Description: "Plugin is Loading",
		},
		"loaded": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_Loaded,
			Description: "Plugin is Loaded",
		},
		"load_failed": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_LoadFailed,
			Description: "Plugin load failed",
		},
	},
})

var PluginActivationType = graphql.NewEnum(graphql.EnumConfig{
	Name: "PLUGIN_ACTIVATION_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"deactivated": &graphql.EnumValueConfig{
			Value:       protobuff.PluginActivateStatus_deactivated,
			Description: "Local Plugin Gets Build and Installed at Run Time",
		},
		"activated": &graphql.EnumValueConfig{
			Value:       protobuff.PluginActivateStatus_activated,
			Description: "Third Party Plugin Gets Installed by Github",
		},
	},
})

var PluginSystemType = graphql.NewEnum(graphql.EnumConfig{
	Name: "PLUGIN_SYSTEM_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"local": &graphql.EnumValueConfig{
			Value:       "local",
			Description: "Local Plugin Gets Build and Installed at Run Time",
		},
		"third_party": &graphql.EnumValueConfig{
			Value:       "third_party",
			Description: "Third Party Plugin Gets Installed by Github",
		},
	},
})

var PublishStatusEnums = graphql.NewEnum(graphql.EnumConfig{
	Name: "PUBLISH_STATUS_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"draft": &graphql.EnumValueConfig{
			Value:       "draft",
			Description: "When Document is in Draft",
		},
		"published": &graphql.EnumValueConfig{
			Value:       "published",
			Description: "When Document is published",
		},
	},
})

var FilterStatusEnums = graphql.NewEnum(graphql.EnumConfig{
	Name: "FILTER_STATUS_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"all": &graphql.EnumValueConfig{
			Value:       "all",
			Description: "Filter All Status Document",
		},
		"draft": &graphql.EnumValueConfig{
			Value:       "draft",
			Description: "When Document is in Draft",
		},
		"published": &graphql.EnumValueConfig{
			Value:       "published",
			Description: "When Document is published",
		},
	},
})

var InputTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "INPUT_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"int": &graphql.EnumValueConfig{
			Value:       "int",
			Description: "Integer/Number Type",
		},
		"bool": &graphql.EnumValueConfig{
			Value:       "bool",
			Description: "True/ False Value",
		},
		"double": &graphql.EnumValueConfig{
			Value:       "double",
			Description: "Floating Point Number",
		},
		"string": &graphql.EnumValueConfig{
			Value:       "string",
			Description: "String Type",
		},
		"geo": &graphql.EnumValueConfig{
			Value:       "geo",
			Description: "GEO Location Type",
		},
		"object": &graphql.EnumValueConfig{
			Value:       "object",
			Description: "Object Input Type",
		},
		"repeated": &graphql.EnumValueConfig{
			Value:       "repeated",
			Description: "Array Input Type",
		},
	},
})

var FieldTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "FIELD_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"text": &graphql.EnumValueConfig{
			Value:       "text",
			Description: "Text Field",
		},
		"media": &graphql.EnumValueConfig{
			Value:       "media",
			Description: "Media Module Type",
		},
		"multiline": &graphql.EnumValueConfig{
			Value:       "multiline",
			Description: "Multi Line Text",
		},
		"date": &graphql.EnumValueConfig{
			Value:       "date",
			Description: "Floating Point Number",
		},
		"number": &graphql.EnumValueConfig{
			Value:       "number",
			Description: "Number Field",
		},
		"boolean": &graphql.EnumValueConfig{
			Value:       "boolean",
			Description: "Module Type",
		},
		"list": &graphql.EnumValueConfig{
			Value:       "list",
			Description: "Module Type",
		},
		"geo": &graphql.EnumValueConfig{
			Value:       "geo",
			Description: "Module Type",
		},
		"object": &graphql.EnumValueConfig{
			Value:       "object",
			Description: "Single Object Field Type",
		},
		"repeated": &graphql.EnumValueConfig{
			Value:       "repeated",
			Description: "Array of Object Field Type",
		},
	},
})

var FieldSubTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "FIELD_SUB_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"dropdown": &graphql.EnumValueConfig{
			Value:       "dropdown",
			Description: "Dropdown Field",
		},
		"dynamicList": &graphql.EnumValueConfig{
			Value:       "dynamicList",
			Description: "Incremental Dynamic List of Item",
		},
		"multiSelect": &graphql.EnumValueConfig{
			Value:       "multiSelect",
			Description: "Multiple Select",
		},
	},
})

var ConnectionTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "CONNECTION_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"has_one": &graphql.EnumValueConfig{
			Value:       "has_one",
			Description: "One to One Relation",
		},
		"has_many": &graphql.EnumValueConfig{
			Value:       "has_many",
			Description: "Has Many Relations",
		},
	},
})
