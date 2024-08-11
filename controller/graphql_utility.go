package controller

import (
	"fmt"
	"strings"

	"github.com/TruthHun/html2md"
	"github.com/apito-io/buffers/protobuff"
	"github.com/k3a/html2text"
	"github.com/tailor-inc/graphql"
)

func (g *GraphCtrl) GetGraphQLField(name string, field *protobuff.FieldInfo, update bool) *graphql.Field {
	switch field.InputType {
	case "string", "int", "double", "bool":
		return GetFieldByType(name+"_"+field.Identifier, field.InputType, field.FieldType, field.Validation, "field", update).(*graphql.Field)
	case "geo":
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
	case "repeated":
		objmodel := graphql.Fields{
			"_id": &graphql.Field{Type: graphql.String},
		}
		for _, subfield := range field.SubFieldInfo {
			if val := GetFieldByType(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield.InputType, subfield.FieldType, subfield.Validation, "field", update); val != nil {
				objmodel[subfield.Identifier] = val.(*graphql.Field)
			}
		}
		return &graphql.Field{
			Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name:   fmt.Sprintf(`%s_%s`, name, field.Identifier),
				Fields: objmodel,
			})),
		}
	case "object":
		objmodel := graphql.Fields{}
		for _, subfield := range field.SubFieldInfo {
			if val := GetFieldByType(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield.InputType, subfield.FieldType, subfield.Validation, "field", update); val != nil {
				objmodel[subfield.Identifier] = val.(*graphql.Field)
			}
		}
		return &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name:   fmt.Sprintf(`%s_%s`, name, field.Identifier),
				Fields: objmodel,
			}),
		}
	}
	return nil
}

func GetFieldByType(name string, inputType string, fieldType string, validation *protobuff.Validation, argType string, update bool) interface{} {
	switch inputType {
	case "string":
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
			if !validation.IsMultiChoice && len(validation.FixedListElements) > 0 { // and multi-choice & dynamic list
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
			case "multiline":
				t = graphql.NewObject(graphql.ObjectConfig{
					Name: name + "_MultilineResponse",
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
								if val, ok := source["html"].(string); ok {
									text := html2text.HTML2Text(val)
									return text, nil
								}
								return nil, nil
							},
						},
					},
				})
				break
			case "media":
				tt := graphql.NewObject(graphql.ObjectConfig{
					Name: name + "MediaResponse",
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
				if validation.IsGallery {
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
			case "multiline":
				t = graphql.NewInputObject(graphql.InputObjectConfig{
					Name: name + "_MultilineInput",
					Fields: graphql.InputObjectConfigFieldMap{
						"html": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
					},
				})
			case "media":
				if validation.IsGallery {
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
	case "int":
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
	case "double":
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
			break
		}
		break
	case "bool":
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
			break
		}
		break
	case "geo":
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

func (g *GraphCtrl) GetGraphQLArgumentObjectField(name string, field *protobuff.FieldInfo, update bool) *graphql.InputObjectFieldConfig {
	switch field.InputType {
	case "string", "int", "double", "bool":
		return GetFieldByType(name+"_"+field.Identifier, field.InputType, field.FieldType, field.Validation, "input", update).(*graphql.InputObjectFieldConfig)
	case "geo":
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
	case "repeated":
		objmodel := graphql.InputObjectConfigFieldMap{
			"_id": &graphql.InputObjectFieldConfig{Type: graphql.String},
		}
		for _, subfield := range field.SubFieldInfo {
			// same object could not be used because one is *FieldInfo another *SubFieldInfo.. to avoid loopOnNestedSets/deadlock
			if val := GetFieldByType(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield.InputType, subfield.FieldType, subfield.Validation, "input", update); val != nil {
				objmodel[subfield.Identifier] = val.(*graphql.InputObjectFieldConfig)
			}
		}
		return &graphql.InputObjectFieldConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   name + "_" + field.Identifier,
				Fields: objmodel,
			}),
		}
	case "object":
		objmodel := graphql.InputObjectConfigFieldMap{}
		for _, subfield := range field.SubFieldInfo {
			// same object could not be used because one is *FieldInfo another *SubFieldInfo.. to avoid loopOnNestedSets/deadlock
			if val := GetFieldByType(name+"_"+field.Identifier+"_"+subfield.Identifier, subfield.InputType, subfield.FieldType, subfield.Validation, "input", update); val != nil {
				objmodel[subfield.Identifier] = val.(*graphql.InputObjectFieldConfig)
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
