package enums

import "github.com/tailor-inc/graphql"

func BuildLocalEnum(locals graphql.EnumValueConfigMap) *graphql.Enum {
	return graphql.NewEnum(graphql.EnumConfig{
		Name:   "LOCAL_TYPE_EMUN",
		Values: locals,
	})
}

// global filter
var SortEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "SORT_EMUN",
	Values: graphql.EnumValueConfigMap{
		"ASC": &graphql.EnumValueConfig{
			Value:       "ASC",
			Description: "SORT BY ASC",
		},
		"DESC": &graphql.EnumValueConfig{
			Value:       "DESC",
			Description: "SORT BY DESC",
		},
	},
})
