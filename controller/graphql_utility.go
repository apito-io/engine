package controller

import (
	"fmt"
	"strings"

	"github.com/TruthHun/html2md"
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/k3a/html2text"
	"github.com/tailor-inc/graphql"
)

func (g *GraphCtrl) GetGraphQLAggregateField(name string, field *models.FieldInfo, update bool) *graphql.Field {
	switch field.InputType {
	case _const.IntInput, _const.DoubleInput:
		return GetFieldByType(name+"_"+field.Identifier, field.InputType, field.FieldType, field.Validation, "field", update).(*graphql.Field)
	default:
		// do nothing
	}
	return nil
}

func (g *GraphCtrl) GetGraphQLField(name string, field *models.FieldInfo, update bool) *graphql.Field {
	switch field.InputType {
	case _const.StringInput, _const.IntInput, _const.DoubleInput, _const.BoolInput:
		return GetFieldByType(name+"_"+field.Identifier, field.InputType, field.FieldType, field.Validation, "field", update).(*graphql.Field)
	case _const.GeoInput:
		return &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: strings.ToUpper(name) + "_GEO_NEAR_INPUT",
				Fields: graphql.Fields{
					"lat": &graphql.Field{
						Type: graphql.Float,
					},
					"lon": &graphql.Field{
						Type: graphql.Float,
					},
					"coordinates": &graphql.Field{
						Type: graphql.NewList(graphql.Float),
					},
					"type": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
		}
	case _const.RepeatedInput:
		objModel := graphql.Fields{
			"_id": &graphql.Field{Type: graphql.String},
		}
		for _, subfield := range field.SubFieldInfo {
			if subfield.SubFieldInfo != nil {
				objModel[subfield.Identifier] = g.GetGraphQLField(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield, update)
			} else {
				if val := GetFieldByType(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield.InputType, subfield.FieldType, subfield.Validation, "field", update); val != nil {
					objModel[subfield.Identifier] = val.(*graphql.Field)
				}
			}
		}
		return &graphql.Field{
			Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name:   fmt.Sprintf(`%s_%s`, name, field.Identifier),
				Fields: objModel,
			})),
		}
	case _const.ObjectInput:
		objModel := graphql.Fields{}
		for _, subfield := range field.SubFieldInfo {
			if val := GetFieldByType(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield.InputType, subfield.FieldType, subfield.Validation, "field", update); val != nil {
				objModel[subfield.Identifier] = val.(*graphql.Field)
			}
			if subfield.SubFieldInfo != nil {
				objModel[subfield.Identifier] = g.GetGraphQLField(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield, update)
			}
		}
		return &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name:   fmt.Sprintf(`%s_%s`, name, field.Identifier),
				Fields: objModel,
			}),
		}
	}
	return nil
}

func GetFieldByType(name string, inputType string, fieldType string, validation *models.Validation, argType string, update bool) interface{} {
	switch inputType {
	case _const.StringInput:
		var t graphql.Type
		switch update {
		case true:
			if validation != nil && validation.IsGallery {
				t = graphql.NewList(graphql.String)
			} else {
				t = graphql.String
			}
			break
		case false:
			if validation != nil && validation.Required {
				t = graphql.NewNonNull(graphql.String)
			} else if validation != nil && validation.Required && validation.IsGallery {
				t = graphql.NewNonNull(graphql.NewList(graphql.String))
			} else if validation != nil && validation.IsGallery {
				t = graphql.NewList(graphql.String)
			} else {
				t = graphql.String
			}
			break
		}
		switch fieldType {
		case "list":
			if validation != nil && !validation.IsMultiChoice && len(validation.FixedListElements) > 0 { // and multi-choice & dynamic list
				t = graphql.String
			} else {
				t = graphql.NewList(graphql.String)
			}
			break
		default:
			t = graphql.String
		}
		switch argType {
		case "field":
			switch fieldType {
			case _const.MultilineField:
				t = graphql.NewObject(graphql.ObjectConfig{
					Name: name + "_MultilineField",
					Fields: graphql.Fields{
						"html": &graphql.Field{
							Type: graphql.String,
						},
						"markdown": &graphql.Field{
							Type: graphql.String,
							Resolve: func(p graphql.ResolveParams) (interface{}, error) {
								source := p.Source.(map[string]interface{})
								if val, ok := source["html"].(string); ok {
									markdown := html2md.Convert(val)
									return markdown, nil
								}
								return nil, nil
							},
						},
						"text": &graphql.Field{
							Type: graphql.String,
							Resolve: func(p graphql.ResolveParams) (interface{}, error) {
								source := p.Source.(map[string]interface{})
								if val, ok := source["text"].(string); ok && val != "" {
									return val, nil
								} else if val, ok := source["html"].(string); ok && val != "" {
									text := html2text.HTML2Text(val)
									return text, nil
								} else if val, ok := source["markdown"].(string); ok && val != "" {
									return val, nil
								} else {
									return nil, nil
								}
							},
						},
					},
				})
				break
			case _const.MediaField:
				tt := graphql.NewObject(graphql.ObjectConfig{
					Name: name + "MediaField",
					Fields: graphql.Fields{
						"file_name": &graphql.Field{
							Type: graphql.String,
						},
						"url": &graphql.Field{
							Type: graphql.String,
						},
						"id": &graphql.Field{
							Type: graphql.String,
						},
					},
				})
				if validation != nil && validation.IsGallery {
					t = graphql.NewList(tt)
				} else {
					t = tt
				}
				break
				/*default:
				t = graphql.String // for field type overwrite the required type*/
			}
			return &graphql.Field{
				Type: t,
			}
		case "arg":
			return &graphql.ArgumentConfig{
				Type: t,
			}
		case "input":
			switch fieldType {
			case _const.MultilineField:
				t = graphql.NewInputObject(graphql.InputObjectConfig{
					Name: name + "_MultilineInput",
					Fields: graphql.InputObjectConfigFieldMap{
						"html": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"markdown": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"text": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
					},
				})
			case _const.MediaField:
				if validation != nil && validation.IsGallery {
					t = graphql.NewInputObject(graphql.InputObjectConfig{
						Name: name + "_MultipleMediaInput",
						Fields: graphql.InputObjectConfigFieldMap{
							"urls": &graphql.InputObjectFieldConfig{
								Type: graphql.NewList(graphql.String),
							},
						},
					})
				} else {
					t = graphql.NewInputObject(graphql.InputObjectConfig{
						Name: name + "_SingleMediaInput",
						Fields: graphql.InputObjectConfigFieldMap{
							"url": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
						},
					})
				}
				break
			}
			return &graphql.InputObjectFieldConfig{
				Type: t,
			}
		default:
			fmt.Println("err")
		}
		break
	case _const.IntInput:
		var t graphql.Type
		switch update {
		case true:
			t = graphql.Int
			break
		case false:
			if validation != nil && validation.Required {
				t = graphql.NewNonNull(graphql.Int)
			} else {
				t = graphql.Int
			}
			break
		}

		switch argType {
		case "field":
			t = graphql.Int // if its a field then overwrite the required
			return &graphql.Field{
				Type: t,
			}
		case "arg":
			return &graphql.ArgumentConfig{
				Type: t,
			}
		case "input":
			return &graphql.InputObjectFieldConfig{
				Type: t,
			}
		}
		break
	case _const.DoubleInput:
		var t graphql.Type
		switch update {
		case true:
			t = graphql.Float
			break
		case false:
			if validation != nil && validation.Required {
				t = graphql.NewNonNull(graphql.Float)
			} else {
				t = graphql.Float
			}
			break
		}

		switch argType {
		case "field":
			return &graphql.Field{
				Type: t,
			}
		case "arg":
			return &graphql.ArgumentConfig{
				Type: t,
			}
		case "input":
			return &graphql.InputObjectFieldConfig{
				Type: t,
			}
		}
		break
	case _const.BoolInput:
		var t graphql.Type
		switch update {
		case true:
			t = graphql.Boolean
			break
		case false:
			if validation != nil && validation.Required {
				t = graphql.NewNonNull(graphql.Boolean)
			} else {
				t = graphql.Boolean
			}
			break
		}

		switch argType {
		case "field":
			return &graphql.Field{
				Type: t,
			}
		case "arg":
			return &graphql.ArgumentConfig{
				Type: t,
			}
		case "input":
			return &graphql.InputObjectFieldConfig{
				Type: t,
			}
		}
		break
	case _const.GeoInput:
		switch argType {
		case "field":
			return &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: strings.ToUpper(name) + "_GEO_NEAR_INPUT",
					Fields: graphql.Fields{
						"lat": &graphql.Field{
							Type: graphql.Float,
						},
						"lon": &graphql.Field{
							Type: graphql.Float,
						},
						"coordinates": &graphql.Field{
							Type: graphql.NewList(graphql.Float),
						},
						"type": &graphql.Field{
							Type: graphql.String,
						},
					},
				}),
			}
		case "arg":
			return &graphql.ArgumentConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name: name + "_repeated_geo",
					Fields: graphql.InputObjectConfigFieldMap{
						"lat": &graphql.InputObjectFieldConfig{
							Type: graphql.Float,
						},
						"lon": &graphql.InputObjectFieldConfig{
							Type: graphql.Float,
						},
					},
				}),
			}
		case "input":
			return &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name: name + "_repeated_geo",
					Fields: graphql.InputObjectConfigFieldMap{
						"lat": &graphql.InputObjectFieldConfig{
							Type: graphql.Float,
						},
						"lon": &graphql.InputObjectFieldConfig{
							Type: graphql.Float,
						},
					},
				}),
			}
		default:
			fmt.Println("err")
		}
		break
	}
	return nil
}

func (g *GraphCtrl) GetGraphQLArgumentObjectField(name string, field *models.FieldInfo, update bool) *graphql.InputObjectFieldConfig {
	switch field.InputType {
	case _const.StringInput, _const.IntInput, _const.DoubleInput, _const.BoolInput:
		return GetFieldByType(name+"_"+field.Identifier, field.InputType, field.FieldType, field.Validation, "input", update).(*graphql.InputObjectFieldConfig)
	case _const.GeoInput:
		return &graphql.InputObjectFieldConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name: name + "_" + field.Identifier,
				Fields: graphql.InputObjectConfigFieldMap{
					"lat": &graphql.InputObjectFieldConfig{
						Type: graphql.Float,
					},
					"lon": &graphql.InputObjectFieldConfig{
						Type: graphql.Float,
					},
				},
			}),
		}
	case _const.RepeatedInput:
		objmodel := graphql.InputObjectConfigFieldMap{
			"_id": &graphql.InputObjectFieldConfig{Type: graphql.String},
		}

		for _, subfield := range field.SubFieldInfo {
			if subfield.SubFieldInfo != nil {
				objmodel[subfield.Identifier] = g.GetGraphQLArgumentObjectField(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield, update)
			} else {
				// same object could not be used because one is *FieldInfo another *SubFieldInfo.. to avoid loopOnNestedSets/deadlock
				if val := GetFieldByType(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield.InputType, subfield.FieldType, subfield.Validation, "input", update); val != nil {
					objmodel[subfield.Identifier] = val.(*graphql.InputObjectFieldConfig)
				}
			}
		}
		return &graphql.InputObjectFieldConfig{
			Type: graphql.NewList(graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   name + "_" + field.Identifier,
				Fields: objmodel,
			})),
		}
	case _const.ObjectInput:
		objmodel := graphql.InputObjectConfigFieldMap{}
		for _, subfield := range field.SubFieldInfo {
			// same object could not be used because one is *FieldInfo another *SubFieldInfo.. to avoid loopOnNestedSets/deadlock
			if val := GetFieldByType(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield.InputType, subfield.FieldType, subfield.Validation, "input", update); val != nil {
				objmodel[subfield.Identifier] = val.(*graphql.InputObjectFieldConfig)
			}
			if subfield.SubFieldInfo != nil {
				objmodel[subfield.Identifier] = g.GetGraphQLArgumentObjectField(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield, update)
			}
		}
		return &graphql.InputObjectFieldConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   name + "_" + field.Identifier,
				Fields: objmodel,
			}),
		}
	}
	return nil
}
