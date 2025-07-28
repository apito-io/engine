package scaler

import "github.com/tailor-inc/graphql"

var UpdateModelTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "UpdateModelTypeEnum",
	Values: graphql.EnumValueConfigMap{
		"rename": &graphql.EnumValueConfig{
			Value:       "rename",
			Description: "",
		},
		"duplicate": &graphql.EnumValueConfig{
			Value:       "duplicate",
			Description: "",
		},
		"convert": &graphql.EnumValueConfig{
			Value:       "convert",
			Description: "",
		},
		"delete": &graphql.EnumValueConfig{
			Value:       "delete",
			Description: "",
		},
	},
})
