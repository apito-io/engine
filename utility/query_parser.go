package utility

import (
	"errors"
	"strings"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	"github.com/jinzhu/inflection"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func ExtractGraphQLOperationName(query string, schema *protobuff.ProjectSchema, isSystemQuery bool) ([]*shared.IncomingRequest, error) {
	qq, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		return nil, err
	}

	if len(qq.Operations) > 1 {
		return nil, errors.New("running multiple queries in a single request is not allowed at this moment")
	}

	xx := qq.Operations.ForName("")
	//name := xx.Operation

	var resp []*shared.IncomingRequest
	if isSystemQuery {
		resp = loopOnSystemNestedSets(xx.Operation, xx.SelectionSet)
	} else {
		resp = loopOnNestedSets(schema, xx.Operation, xx.SelectionSet)
	}

	return resp, nil
}

func loopOnSystemNestedSets(operationName ast.Operation, sets ast.SelectionSet) []*shared.IncomingRequest {
	var incomingRequest []*shared.IncomingRequest
	for _, yy := range sets {
		var _models []*shared.FilteredModel

		zz := yy.(*ast.Field)

		operation := zz.Name
		var operationType string

		switch operationName {
		case "query":
			if operation == "__schema" { // skip for introspect query for editor & client
				return nil
			}
			_models = append(_models, &shared.FilteredModel{
				Name:        operation,
				WhereFilter: nil,
			})
			operationType = "query"

			// search for sub query
		case "mutation":
			_models = append(_models, &shared.FilteredModel{
				Name:        operation,
				WhereFilter: nil,
			})
			operationType = "mutation"
		}

		incomingRequest = append(incomingRequest, &shared.IncomingRequest{
			OperationType:  operationType,
			FilteredModels: _models,
		})
	}

	return incomingRequest
}

func loopOnNestedSets(schema *protobuff.ProjectSchema, operationName ast.Operation, sets ast.SelectionSet) []*shared.IncomingRequest {
	var incomingRequest []*shared.IncomingRequest

	for _, yy := range sets {
		var _models []*shared.FilteredModel
		var isFunction bool

		zz := yy.(*ast.Field)

		operation := zz.Name
		var operationType string

		var _modelData *shared.FilteredModel
		switch operationName {
		case "query":
			if operation == "__schema" || operation == "__typename" { // skip for introspect query for editor & client
				return nil
			}
			var isConnectionQuery bool
			if strings.HasSuffix(operation, "Connection") {
				operation = strings.TrimSuffix(operation, "Connection")
				isConnectionQuery = true
			}

			_modelData = &shared.FilteredModel{
				Name:              inflection.Singular(operation),
				IsConnectionQuery: isConnectionQuery,
			}

			if len(zz.Arguments) > 0 {
				_modelData.WhereFilter = extractArguments(zz.Arguments)
			}

			_models = append(_models, _modelData)
			operationType = "query"

			// search for sub query
		case "mutation":
			var _function *protobuff.CloudFunction
			for _, function := range schema.Functions {
				if function.Name == operation {
					_function = function
					_models = append(_models, []*shared.FilteredModel{
						{
							Name: _function.Request.Model,
						},
						{
							Name: _function.Response.Model,
						},
					}...)
					isFunction = true
					break
				}
			}
			if !isFunction {
				if strings.HasPrefix(operation, "create") {
					_model := strings.TrimPrefix(operation, "create")
					_models = append(_models, &shared.FilteredModel{
						Name:        strings.ToLower(_model),
						WhereFilter: nil,
					})
				} else if strings.HasPrefix(operation, "update") {
					_model := strings.TrimPrefix(operation, "update")
					_models = append(_models, &shared.FilteredModel{
						Name:        strings.ToLower(_model),
						WhereFilter: nil,
					})
				} else if strings.HasPrefix(operation, "delete") {
					_model := strings.TrimPrefix(operation, "delete")
					_models = append(_models, &shared.FilteredModel{
						Name:        strings.ToLower(_model),
						WhereFilter: nil,
					})
				} else {
					_models = append(_models, &shared.FilteredModel{
						Name:        inflection.Singular(operation),
						WhereFilter: nil,
					})
				}
			}
			operationType = "mutation"
		}

		/*if len(zz.SelectionSet) > 0 {
			_res := loopOnNestedSets(schema, operationName, zz.SelectionSet)
			incomingRequest = append(incomingRequest, _res...)
		}*/

		if len(zz.SelectionSet) > 0 {
			// find if there is any meta-data available or not
			for _, _yy := range zz.SelectionSet {
				_zz := _yy.(*ast.Field)
				switch _zz.Name {
				case "meta":
					_modelData.HasMetaQuery = true
				case "id", "data":
					// for now we are not doing anything
				default:
					_models = append(_models, &shared.FilteredModel{
						Name: inflection.Singular(_zz.Name),
					})
				}
			}
		}

		incomingRequest = append(incomingRequest, &shared.IncomingRequest{
			OperationType:  operationType,
			FilteredModels: _models,
			IsFunction:     isFunction,
		})

		/*for _, ww := range zz.SelectionSet {
			tt := ww.(*ast.Field)
			fmt.Println(tt.Name)
		}*/
	}

	return incomingRequest
}

func extractArguments(args ast.ArgumentList) []string {
	var _args []string

	for _, y := range args {
		if y.Value != nil && len(y.Value.Children) > 0 {
			_childs := y.Value.Children
			for _, v := range _childs {
				_args = append(_args, v.Name)
			}
		}
	}

	return _args
}
