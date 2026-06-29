package objects

import (
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-platform/graphql"
)

func BuildFilterArgument(localEnum *graphql.Enum, name string, connectionArgs *graphql.InputObject, whereArgs graphql.InputObjectConfigFieldMap, whereConnectionArgs *graphql.InputObject, sortArgs graphql.InputObjectConfigFieldMap) graphql.FieldConfigArgument {

	_whereArgs := make(graphql.InputObjectConfigFieldMap)
	if len(whereArgs) > 0 {
		_whereArgs = make(graphql.InputObjectConfigFieldMap)
		for k, v := range whereArgs {
			_whereArgs[k] = v
		}

		_whereArgs["OR"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   strings.ToUpper(name + "_Or_Condition"),
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
		"_key": &graphql.ArgumentConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:        strings.ToUpper(name + "_Key_Condition"),
				Description: "Filter by document _key ids. If this is used, the where filter will be ignored.",
				Fields: graphql.InputObjectConfigFieldMap{
					"eq": &graphql.InputObjectFieldConfig{
						Type: graphql.String,
					},
					"ne": &graphql.InputObjectFieldConfig{
						Type: graphql.String,
					},
					"in": &graphql.InputObjectFieldConfig{
						Type: graphql.NewList(graphql.String),
					},
				},
			}),
		},
	}

	if connectionArgs != nil {
		filterArgConfig["connection"] = &graphql.ArgumentConfig{
			Type: connectionArgs,
		}
	}

	// Add group argument always, not just when whereConnectionArgs is present
	filterArgConfig["groupBy"] = &graphql.ArgumentConfig{
		Type: graphql.NewList(graphql.NewInputObject(graphql.InputObjectConfig{
			Name:        strings.ToUpper(name + "_GroupBy_Input"),
			Description: "Group by fields with key-value pairs for both user input and editable fields",
			Fields: graphql.InputObjectConfigFieldMap{
				"key": &graphql.InputObjectFieldConfig{
					Type:        graphql.String,
					Description: "Field name to group by",
				},
				"value": &graphql.InputObjectFieldConfig{
					Type:        graphql.String,
					Description: "Value to group by",
				},
			},
		})),
	}

	if whereConnectionArgs != nil {
		filterArgConfig["relation"] = &graphql.ArgumentConfig{
			Type: whereConnectionArgs,
		}
	}

	if len(_whereArgs) > 0 {
		filterArgConfig["where"] = &graphql.ArgumentConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   strings.ToUpper(name + "_Input_Where_Payload"),
				Fields: _whereArgs,
			}),
		}
	}

	if len(sortArgs) > 0 {
		filterArgConfig["sort"] = &graphql.ArgumentConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   strings.ToUpper(name + "_Input_Sort_Payload"),
				Fields: sortArgs,
			}),
		}
	}

	return filterArgConfig
}

func BuildConnectionArguments(name string, connections []*models.ConnectionType) *graphql.InputObject {

	fields := graphql.InputObjectConfigFieldMap{
		"connection_type": &graphql.InputObjectFieldConfig{
			Type: enums.ConnectionTypeEnum,
		},
		"_id": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"to_model": &graphql.InputObjectFieldConfig{
			Type: enums.BuildModelEnum(name, connections),
		},
		"relation_type": &graphql.InputObjectFieldConfig{
			Type: enums.RelationTypeEnum,
		},
	}

	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   strings.ToUpper(name + "_Connection_Filter_Condition"),
		Fields: fields,
	})
}

func BuildWhereRelationConditionArgument(name string, connections []*models.ConnectionType, whereArgs map[string]graphql.InputObjectConfigFieldMap) *graphql.InputObject {
	fields := graphql.InputObjectConfigFieldMap{}

	for _, connection := range connections {
		// only do it for has_one, has_many, many_to_many relation
		if len(whereArgs[connection.Model]) > 0 {
			relationFields := graphql.InputObjectConfigFieldMap{}
			for k, v := range whereArgs[connection.Model] {
				relationFields[k] = v
			}
			relationFields["_id"] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name:        strings.ToUpper(name + "_" + connection.Model + "_Relation_ID_Filter_Condition"),
					Description: "Filter related documents by id.",
					Fields: graphql.InputObjectConfigFieldMap{
						"eq": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"ne": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"in": &graphql.InputObjectFieldConfig{
							Type: graphql.NewList(graphql.String),
						},
						"not_in": &graphql.InputObjectFieldConfig{
							Type: graphql.NewList(graphql.String),
						},
					},
				}),
			}
			fields[connection.Model] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name:   strings.ToUpper(name + "_" + connection.Model + "_Where_Relation_Filter_Condition"),
					Fields: relationFields,
				}),
			}
		}
	}

	if len(fields) == 0 {
		return nil
	}

	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   strings.ToUpper(name + "_Where_Relation_Filter_Condition"),
		Fields: fields,
	})
}

func BuildWhereConditionArgument(modelName, fieldPath string, fieldInfo *models.FieldInfo) *graphql.InputObject {

	fields := graphql.InputObjectConfigFieldMap{}

	if fieldInfo == nil {
		return graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   utility.WhereFilterConditionGraphQLTypeName(modelName, fieldPath),
			Fields: fields,
		})
	}

	// input type special filter
	switch fieldInfo.InputType {
	case "string":
		switch fieldInfo.FieldType {
		case "list":
			v := fieldInfo.Validation
			isDropdown := v != nil && !v.IsMultiChoice && len(v.FixedListElements) > 0
			if isDropdown { // for dropdown
				fields["eq"] = &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				}
				fields["ne"] = &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				}
				fields["in"] = &graphql.InputObjectFieldConfig{
					Type: graphql.NewList(graphql.String),
				}
			} else {
				fields["in"] = &graphql.InputObjectFieldConfig{
					Type: graphql.NewList(graphql.String),
				}
				fields["not_in"] = &graphql.InputObjectFieldConfig{
					Type: graphql.NewList(graphql.String),
				}
			}
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
			fields["between"] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewList(graphql.String),
			}
		case "multiline":
			fields["contains"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
		default:
			fields["eq"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			fields["ne"] = &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			}
			fields["in"] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewList(graphql.String),
			}
			fields["not_in"] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewList(graphql.String),
			}
			if !fieldInfo.SystemGenerated {
				fields["contains"] = &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				}
			}
		}
	case "int":
		fields["eq"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
		}
		fields["ne"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Int,
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
		fields["between"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewList(graphql.Int),
		}
		fields["in"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewList(graphql.Int),
		}
		fields["not_in"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewList(graphql.Int),
		}
	case "double":
		fields["eq"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Float,
		}
		fields["ne"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Float,
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
	case "bool":
		fields["eq"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Boolean,
		}
		fields["ne"] = &graphql.InputObjectFieldConfig{
			Type: graphql.Boolean,
		}
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
				Name: strings.ToUpper(modelName + "__FIELD__" + fieldPath + "__GEO_WITHIN_INPUT"),
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
	case "repeated":
		for _, sf := range fieldInfo.SubFieldInfo {
			fields[sf.Identifier] = &graphql.InputObjectFieldConfig{
				Type: BuildWhereConditionArgument(modelName, fieldPath+"__"+sf.Identifier+"__repeated", &models.FieldInfo{
					Identifier:      sf.Identifier,
					Description:     sf.Description,
					InputType:       sf.InputType,
					FieldType:       sf.FieldType,
					Validation:      sf.Validation,
					Serial:          sf.Serial,
					Label:           sf.Label,
					SystemGenerated: sf.SystemGenerated,
					SubFieldInfo:    sf.SubFieldInfo,
				}),
			}
		}
	case "object":
		for _, sf := range fieldInfo.SubFieldInfo {
			fields[sf.Identifier] = &graphql.InputObjectFieldConfig{
				Type: BuildWhereConditionArgument(modelName, fieldPath+"__"+sf.Identifier+"__object", &models.FieldInfo{
					Identifier:      sf.Identifier,
					Description:     sf.Description,
					InputType:       sf.InputType,
					FieldType:       sf.FieldType,
					Validation:      sf.Validation,
					Serial:          sf.Serial,
					Label:           sf.Label,
					SystemGenerated: sf.SystemGenerated,
					SubFieldInfo:    sf.SubFieldInfo,
				}),
			}
		}
	}

	if len(fields) == 0 {
		fmt.Println("Field is empty")
	}

	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   utility.WhereFilterConditionGraphQLTypeName(modelName, fieldPath),
		Fields: fields,
	})
}

func BuildSortConditionArgument(name string, fieldInfo *models.FieldInfo) *graphql.Enum {
	return enums.SortEnum
}
