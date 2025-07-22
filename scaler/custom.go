package scaler

import (
	"fmt"
	"strconv"

	"github.com/apito-io/engine/models"
	"github.com/tailor-inc/graphql"
	"github.com/tailor-inc/graphql/language/ast"
)

type CustomParseLiteral func(valueAST ast.Value) interface{}

// ScalarJSONWithRequest is a custom scalar type for JSON object
func ScalarJSONWithRequest(name string, incomingReq *models.GraphQLIncomingRequest) *graphql.Scalar {
	return graphql.NewScalar(graphql.ScalarConfig{
		Name:        name + "_JSONArgument",
		Description: "খাটি JSON object, key:value pair",
		Serialize: func(value interface{}) interface{} {
			return value
		},
		ParseValue: func(value interface{}) interface{} {
			return value
		},
		ParseLiteral: graphql.ParseLiteralFn(customParseLiteral(incomingReq)),
	})
}

// ScalarJSON is a custom scalar type for JSON object
var ScalarJSON = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "JSON",
	Description: "খাটি JSON object, key:value pair",
	Serialize: func(value interface{}) interface{} {
		return value
	},
	ParseValue: func(value interface{}) interface{} {
		return value
	},
	ParseLiteral: graphql.ParseLiteralFn(customParseLiteral(nil)),
})

// ScalarJSONArray is a custom scalar type for JSON array with mixed types
var ScalarJSONArray = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "JSONArray",
	Description: "JSON array that can contain mixed types (strings, numbers, booleans)",
	Serialize: func(value interface{}) interface{} {
		return value
	},
	ParseValue: func(value interface{}) interface{} {
		return value
	},
	ParseLiteral: graphql.ParseLiteralFn(customParseArrayLiteral),
})

func customParseLiteral(req *models.GraphQLIncomingRequest) CustomParseLiteral {
	return func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.ObjectValue:
			data := map[string]interface{}{}
			for i := range valueAST.Fields {
				field := valueAST.Fields[i]
				key := field.Name.Value
				_valueType := field.Value
				fmt.Println(_valueType)
				switch field.Value.(type) {
				case *ast.Variable:
					varName := field.Value.GetValue().(*ast.Name).Value
					data[key] = req.Variables[varName]
				//return variableValues[varName]
				case *ast.IntValue:
					val, _ := strconv.Atoi(field.Value.GetValue().(string))
					data[key] = val
				case *ast.FloatValue:
					val, _ := strconv.ParseFloat(field.Value.GetValue().(string), 64)
					data[key] = val
				case *ast.BooleanValue:
					data[key] = field.Value.GetValue().(bool)
				case *ast.StringValue:
					data[key] = field.Value.GetValue().(string)
				case *ast.ListValue:
					var datas []interface{}
					for _, v := range field.Value.GetValue().([]ast.Value) {
						switch v.(type) {
						case *ast.IntValue:
							val, _ := strconv.Atoi(v.GetValue().(string))
							datas = append(datas, val)
						case *ast.FloatValue:
							val, _ := strconv.ParseFloat(v.GetValue().(string), 64)
							datas = append(datas, val)
						case *ast.StringValue:
							datas = append(datas, v.GetValue().(string))
						default:
							datas = append(datas, v.GetValue())
						}
					}
					data[key] = datas
				}
			}
			return data
		}
		return nil
	}
}

func customParseArrayLiteral(valueAST ast.Value) interface{} {
	switch valueAST := valueAST.(type) {
	case *ast.ListValue:
		var result []interface{}
		for _, item := range valueAST.Values {
			switch item := item.(type) {
			case *ast.StringValue:
				result = append(result, item.Value)
			case *ast.IntValue:
				val, _ := strconv.Atoi(item.Value)
				result = append(result, val)
			case *ast.FloatValue:
				val, _ := strconv.ParseFloat(item.Value, 64)
				result = append(result, val)
			case *ast.BooleanValue:
				result = append(result, item.Value)
			case *ast.Variable:
				// Handle variables if needed
				result = append(result, item.Name.Value)
			default:
				result = append(result, item.GetValue())
			}
		}
		return result
	case *ast.StringValue:
		// Handle single string value as array with one element
		return []interface{}{valueAST.Value}
	case *ast.IntValue:
		// Handle single int value as array with one element
		val, _ := strconv.Atoi(valueAST.Value)
		return []interface{}{val}
	case *ast.FloatValue:
		// Handle single float value as array with one element
		val, _ := strconv.ParseFloat(valueAST.Value, 64)
		return []interface{}{val}
	case *ast.BooleanValue:
		// Handle single boolean value as array with one element
		return []interface{}{valueAST.Value}
	}
	return nil
}
