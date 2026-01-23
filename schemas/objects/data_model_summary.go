package objects

import "github.com/tailor-platform/graphql"

var DatModelObject = graphql.NewObject(graphql.ObjectConfig{
	Name: "ListModelData_preview_fields",
	Fields: graphql.Fields{
		"id": &graphql.Field{
			Type: graphql.String,
		},
		"title": &graphql.Field{
			Type: graphql.String,
		},
		"icon": &graphql.Field{
			Type: graphql.String,
		},
		"status": &graphql.Field{
			Type: graphql.String,
		},
	},
})
