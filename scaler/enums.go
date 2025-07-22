package scaler

import "github.com/tailor-inc/graphql"

var UpdateModelTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "UpdateModelTypeEnum",
	Values: graphql.EnumValueConfigMap{
		"Rename": &graphql.EnumValueConfig{
			Value:       "rename",
			Description: "",
		},
		"Duplicate": &graphql.EnumValueConfig{
			Value:       "duplicate",
			Description: "",
		},
		"Convert": &graphql.EnumValueConfig{
			Value:       "convert",
			Description: "",
		},
		"Delete": &graphql.EnumValueConfig{
			Value:       "delete",
			Description: "",
		},
	},
})
