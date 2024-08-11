package scaler

import (
	"github.com/tailor-inc/graphql"
	"github.com/tailor-inc/graphql/language/ast"
	"strconv"
)

// ScalarMap
var ScalarMap = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "Map",
	Description: "খাটি javascript object, key:value pair",
	Serialize: func(value interface{}) interface{} {
		return value
	},
	ParseValue: func(value interface{}) interface{} {
		return value
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.ObjectValue:
			data := map[string]interface{}{}
			for i := range valueAST.Fields {
				field := valueAST.Fields[i]
				key := field.Name.Value
				switch field.Value.(type) {
				case *ast.IntValue:
					val, _ := strconv.Atoi(field.Value.GetValue().(string))
					data[key] = val
					break
				case *ast.FloatValue:
					val, _ := strconv.ParseFloat(field.Value.GetValue().(string), 64)
					data[key] = val
					break
				case *ast.BooleanValue:
					data[key] = field.Value.GetValue().(bool)
					break
				case *ast.StringValue:
					data[key] = field.Value.GetValue().(string)
					break
				case *ast.ListValue:
					var datas []interface{}
					for _, v := range field.Value.GetValue().([]ast.Value) {
						switch v.(type) {
						case *ast.IntValue:
							val, _ := strconv.Atoi(v.GetValue().(string))
							datas = append(datas, val)
							break
						case *ast.FloatValue:
							val, _ := strconv.ParseFloat(v.GetValue().(string), 64)
							datas = append(datas, val)
							break
						case *ast.StringValue:
							datas = append(datas, v.GetValue().(string))
							break
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
	},
})
