package objects

import (
	"strings"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/tailor-inc/graphql"
)

func BuildFilterArgument(localEnum *graphql.Enum, name string, whereArgs graphql.InputObjectConfigFieldMap, sortArgs graphql.InputObjectConfigFieldMap) graphql.FieldConfigArgument {

	_whereArgs := make(graphql.InputObjectConfigFieldMap)
	if len(whereArgs) > 0 {
		_whereArgs = make(graphql.InputObjectConfigFieldMap)
		for k, v := range whereArgs {
			_whereArgs[k] = v
		}

		_whereArgs["OR"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   name + "_OR_CONDITIONS",
				Fields: whereArgs,
			}),
		}
	}

	/*	whereArgs["AND"] = &graphql.InputObjectFieldConfig{
		Type: graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   name + "_AND_CONDITIONS",
			Fields: whereArgs,
		}),
	}*/

	filterArgConfig := graphql.FieldConfigArgument{
		"page": &graphql.ArgumentConfig{
			Type: graphql.Int,
		},
		"limit": &graphql.ArgumentConfig{
			Type: graphql.Int,
		},
		"local": &graphql.ArgumentConfig{
			Type: localEnum,
		},
		"status": &graphql.ArgumentConfig{
			Type: enums.FilterStatusEnums,
		},
	}

	if len(_whereArgs) > 0 {
		filterArgConfig["where"] = &graphql.ArgumentConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   name + "_input_where_payload",
				Fields: _whereArgs,
			}),
		}
	}

	if len(sortArgs) > 0 {
		filterArgConfig["sort"] = &graphql.ArgumentConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   name + "_input_sort_payload",
				Fields: sortArgs,
			}),
		}
	}

	return filterArgConfig
}

func BuildWhereConditionArgument(name string, fieldInfo *protobuff.FieldInfo) *graphql.InputObject {

	fields := graphql.InputObjectConfigFieldMap{}

	// input type special filter
	switch fieldInfo.InputType {
	case "string":
		switch fieldInfo.FieldType {
		case "list":
			if !fieldInfo.Validation.IsMultiChoice && len(fieldInfo.Validation.FixedListElements) > 0 { // for dropdown
				fields["eq"] = &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				}
				fields["ne"] = &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				}
			} else {
				fields["in"] = &graphql.InputObjectFieldConfig{
					Type: graphql.NewList(graphql.String),
				}
				fields["not_in"] = &graphql.InputObjectFieldConfig{
					Type: graphql.NewList(graphql.String),
				}
			}
			break
		case "date":
			fields["eq"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			fields["ne"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			fields["before"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			fields["after"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			break
		case "multiline":
			fields["contains"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			break
		default:
			fields["eq"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			fields["ne"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			fields["contains"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
		}
		break
	case "int":
		fields["eq"] = &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		}
		fields["ne"] = &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		}
		fields["lt"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
		}
		fields["lte"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
		}
		fields["gt"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
		}
		fields["gte"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
		}
		break
	case "double":
		fields["eq"] = &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		}
		fields["ne"] = &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		}
		fields["lt"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Float,
		}
		fields["lte"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Float,
		}
		fields["gt"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Float,
		}
		fields["gte"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Float,
		}
		break
	case "bool":
		fields["eq"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Boolean,
		}
		fields["ne"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Boolean,
		}
		break
	case "geo":
		/*		fields["geo_near"] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name: strings.ToUpper(name) + "_GEO_NEAR_INPUT",
					Fields: graphql.InputObjectConfigFieldMap{
						"lat": &graphql.InputObjectFieldConfig{
							Type: graphql.Float,
						},
						"lon": &graphql.InputObjectFieldConfig{
							Type: graphql.Float,
						},
						"nth": &graphql.InputObjectFieldConfig{
							Description: " n closest coordinates to a reference point, and return the documents with the nearby locations. The default for n is 100, which means 100 documents are returned at most, the closest matches first. Default is 3",
							Type: graphql.Int,
						},
					},
				}),
			}*/
		fields["geo_within"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name: strings.ToUpper(name) + "_GEO_WITHIN_INPUT",
				Fields: graphql.InputObjectConfigFieldMap{
					"lat": &graphql.InputObjectFieldConfig{
						Type: graphql.Float,
					},
					"lon": &graphql.InputObjectFieldConfig{
						Type: graphql.Float,
					},
					"km_radius": &graphql.InputObjectFieldConfig{
						Description: "Radius in KM",
						Type:        graphql.NewNonNull(graphql.Float),
					},
				},
			}),
		}
		break
	case "repeated":
		for _, sf := range fieldInfo.SubFieldInfo {
			fields[sf.Identifier] = &graphql.InputObjectFieldConfig{
				Type: BuildWhereConditionArgument(name+"_"+fieldInfo.Identifier+"_"+sf.Identifier+"_repeated", &protobuff.FieldInfo{
					Identifier:      sf.Identifier,
					Description:     sf.Description,
					InputType:       sf.InputType,
					FieldType:       sf.FieldType,
					Validation:      sf.Validation,
					Serial:          sf.Serial,
					Label:           sf.Label,
					SystemGenerated: sf.SystemGenerated,
				}),
			}
		}
	case "object":
		for _, sf := range fieldInfo.SubFieldInfo {
			fields[sf.Identifier] = &graphql.InputObjectFieldConfig{
				Type: BuildWhereConditionArgument(name+"_"+fieldInfo.Identifier+"_"+sf.Identifier+"_object", &protobuff.FieldInfo{
					Identifier:      sf.Identifier,
					Description:     sf.Description,
					InputType:       sf.InputType,
					FieldType:       sf.FieldType,
					Validation:      sf.Validation,
					Serial:          sf.Serial,
					Label:           sf.Label,
					SystemGenerated: sf.SystemGenerated,
				}),
			}
		}
	}

	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   name + "_COMMON_FILTER_CONDITIONS",
		Fields: fields,
	})
}

func BuildSortConditionArgument(name string, fieldInfo *protobuff.FieldInfo) *graphql.Enum {
	return enums.SortEnum
}
