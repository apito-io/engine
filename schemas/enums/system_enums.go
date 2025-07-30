package enums

import (
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/types/protobuff"
	"github.com/tailor-inc/graphql"
)

type FieldOperation string

const (
	FieldOperation_Rename    FieldOperation = "rename"
	FieldOperation_Duplicate FieldOperation = "duplicate"
	FieldOperation_Delete    FieldOperation = "delete"
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
		"delete": &graphql.EnumValueConfig{
			Value:       FieldOperation_Delete,
			Description: "Delete a Field",
		},
	},
})

var PluginTypeEnums = graphql.NewEnum(graphql.EnumConfig{
	Name: "PLUGIN_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"system": &graphql.EnumValueConfig{
			Value:       protobuff.PluginType_PLUGIN_TYPE_SYSTEM,
			Description: "Extension Type Plugin",
		},
		"project": &graphql.EnumValueConfig{
			Value:       protobuff.PluginType_PLUGIN_TYPE_PROJECT,
			Description: "Function Type Plugin",
		},
	},
})

var PluginLoadTypeEnums = graphql.NewEnum(graphql.EnumConfig{
	Name: "PLUGIN_LOAD_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"not_installed": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_PLUGIN_LOAD_STATUS_NOT_INSTALLED,
			Description: "Plugin is not Installed",
		},
		"installed": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_PLUGIN_LOAD_STATUS_INSTALLED,
			Description: "Plugin is is Building",
		},
		"re_install": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_PLUGIN_LOAD_STATUS_REINSTALL,
			Description: "Plugin is Built",
		},
		"loading": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_PLUGIN_LOAD_STATUS_LOADING,
			Description: "Plugin is Loading",
		},
		"loaded": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_PLUGIN_LOAD_STATUS_LOADED,
			Description: "Plugin is Loaded",
		},
		"load_failed": &graphql.EnumValueConfig{
			Value:       protobuff.PluginLoadStatus_PLUGIN_LOAD_STATUS_LOAD_FAILED,
			Description: "Plugin load failed",
		},
	},
})

var PluginActivationType = graphql.NewEnum(graphql.EnumConfig{
	Name: "PLUGIN_ACTIVATION_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"deactivated": &graphql.EnumValueConfig{
			Value:       protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_DEACTIVATED,
			Description: "Local Plugin Gets Build and Installed at Run Time",
		},
		"activated": &graphql.EnumValueConfig{
			Value:       protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED,
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
			Value:       _const.IntInput,
			Description: "Integer/Number Type",
		},
		"bool": &graphql.EnumValueConfig{
			Value:       _const.BoolInput,
			Description: "True/ False Value",
		},
		"double": &graphql.EnumValueConfig{
			Value:       _const.DoubleInput,
			Description: "Floating Point Number",
		},
		"string": &graphql.EnumValueConfig{
			Value:       _const.StringInput,
			Description: "String Type",
		},
		"geo": &graphql.EnumValueConfig{
			Value:       _const.GeoInput,
			Description: "GEO Location Type",
		},
		"object": &graphql.EnumValueConfig{
			Value:       _const.ObjectInput,
			Description: "Object Input Type",
		},
		"repeated": &graphql.EnumValueConfig{
			Value:       _const.RepeatedInput,
			Description: "Array Input Type",
		},
	},
})

var FieldTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "FIELD_TYPE_ENUM",
	Values: graphql.EnumValueConfigMap{
		"text": &graphql.EnumValueConfig{
			Value:       _const.TextField,
			Description: "Text Field",
		},
		"media": &graphql.EnumValueConfig{
			Value:       _const.MediaField,
			Description: "Media Module Type",
		},
		"multiline": &graphql.EnumValueConfig{
			Value:       _const.MultilineField,
			Description: "Multi Line Text",
		},
		"date": &graphql.EnumValueConfig{
			Value:       _const.DateField,
			Description: "Floating Point Number",
		},
		"number": &graphql.EnumValueConfig{
			Value:       _const.NumberField,
			Description: "Number Field",
		},
		"boolean": &graphql.EnumValueConfig{
			Value:       _const.BooleanField,
			Description: "Module Type",
		},
		"list": &graphql.EnumValueConfig{
			Value:       _const.ListField,
			Description: "Module Type",
		},
		"geo": &graphql.EnumValueConfig{
			Value:       _const.GeoField,
			Description: "Module Type",
		},
		"object": &graphql.EnumValueConfig{
			Value:       _const.ObjectField,
			Description: "Single Object Field Type",
		},
		"repeated": &graphql.EnumValueConfig{
			Value:       _const.RepeatedField,
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
		"forward": &graphql.EnumValueConfig{
			Value:       "forward",
			Description: "forward connection like store has many products",
		},
		"backward": &graphql.EnumValueConfig{
			Value:       "backward",
			Description: "backward connection like product belongs to store",
		},
	},
})

var RelationTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "RELATION_TYPE_ENUM",
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

var RolesEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "ROLES_ENUM",
	Values: graphql.EnumValueConfigMap{
		"developer": &graphql.EnumValueConfig{
			Value:       "developer",
			Description: "For Developer",
		},
		"editor": &graphql.EnumValueConfig{
			Value:       "editor",
			Description: "For Editor",
		},
		"admin": &graphql.EnumValueConfig{
			Value:       "admin",
			Description: "For Admin",
		},
	},
})
