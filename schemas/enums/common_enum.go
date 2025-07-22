package enums

import (
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/tailor-inc/graphql"
)

func BuildModelEnum(name string, connections []*models.ConnectionType) *graphql.Enum {
	var models = make(graphql.EnumValueConfigMap)
	for _, local := range connections {
		models[local.Model] = &graphql.EnumValueConfig{
			Value: local.Model,
			//Description: fmt.Sprintf(`%s %s %s`, local.Model, local.Relation, local.),
		}
	}
	return graphql.NewEnum(graphql.EnumConfig{
		Name:   name + "_MODEL_TYPE_ENUM",
		Values: models,
	})
}

func BuildLocalEnum(_locals []string) *graphql.Enum {

	var locals = make(graphql.EnumValueConfigMap)
	for _, local := range _locals {
		locals[local] = &graphql.EnumValueConfig{
			Value:       local,
			Description: fmt.Sprintf(`%s local support`, local),
		}
	}

	return graphql.NewEnum(graphql.EnumConfig{
		Name:   "LOCAL_TYPE_ENUM",
		Values: locals,
	})
}

// global filter
var SortEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "SORT_ENUM",
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
