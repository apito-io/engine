package args

import (
	"github.com/apito-io/engine/schemas/enums"
	"github.com/tailor-inc/graphql"
)

var FilterArg = &graphql.ArgumentConfig{
	Type: graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "filterArgumentObject",
		Fields: graphql.InputObjectConfigFieldMap{
			"page": &graphql.InputObjectFieldConfig{
				Type: graphql.Int,
			},
			"limit": &graphql.InputObjectFieldConfig{
				Type: graphql.Int,
			},
		},
	}),
}

var CommonFilter = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "CommonFilter",
	Fields: graphql.InputObjectConfigFieldMap{
		"contains": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"eq": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"ne": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"in": &graphql.InputObjectFieldConfig{
			Type:        graphql.NewList(graphql.String),
			Description: "When the field & value both of them are array",
		},
		"in_index": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "When the field is string & value is array",
		},
		"in_r": &graphql.InputObjectFieldConfig{
			Type:        graphql.NewList(graphql.String),
			Description: "In in reverse when the value is array but the field is not array",
		},
		"not_in": &graphql.InputObjectFieldConfig{
			Type: graphql.NewList(graphql.String),
		},
		"only_contains": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "Field should only contains the value",
		},
	},
})

var BooleanFilter = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BooleanFilter",
	Fields: graphql.InputObjectConfigFieldMap{
		"eq": &graphql.InputObjectFieldConfig{
			Type: graphql.Boolean,
		},
		"ne": &graphql.InputObjectFieldConfig{
			Type: graphql.Boolean,
		},
	},
})

var IntegerFilter = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "IntegerFilter",
	Fields: graphql.InputObjectConfigFieldMap{
		"eq": &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
		},
		"lt": &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
		},
		"gt": &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
		},
		"between": &graphql.InputObjectFieldConfig{
			Type: graphql.NewList(graphql.Int),
		},
	},
})

var DateFilter = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "DateFilterFilter",
	Fields: graphql.InputObjectConfigFieldMap{
		"at_date": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"between_date": &graphql.InputObjectFieldConfig{
			Type: graphql.NewList(graphql.String),
		},
	},
})

var TimeFilter = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "TimeFilter",
	Fields: graphql.InputObjectConfigFieldMap{
		"exact": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"before": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"after": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
	},
})

var SortField = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SortingField",
	Fields: graphql.InputObjectConfigFieldMap{
		"col": &graphql.InputObjectFieldConfig{
			Type: graphql.NewNonNull(graphql.String),
		},
		"type": &graphql.InputObjectFieldConfig{
			Type: enums.SortEnum,
		},
	},
})
