package utility

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func ExtractGraphQLOperationName(query string, schema *models.ProjectSchema, isSystemQuery bool) ([]*models.IncomingRequest, error) {
	qq, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		return nil, err
	}

	if len(qq.Operations) > 1 {
		return nil, errors.New("running multiple queries in a single request is not allowed at this moment")
	}

	xx := qq.Operations.ForName("")
	//name := xx.Operation

	var resp []*models.IncomingRequest
	if isSystemQuery {
		resp, err = loopOnSystemNestedSets(xx.Operation, xx.SelectionSet)
	} else {
		resp, err = loopOnNestedSets(schema, xx.Operation, xx.SelectionSet)
	}
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func loopOnSystemNestedSets(operationName ast.Operation, sets ast.SelectionSet) ([]*models.IncomingRequest, error) {
	var incomingRequest []*models.IncomingRequest
	for _, yy := range sets {
		var _models []*models.FilteredModel

		zz := yy.(*ast.Field)

		operation := zz.Name
		var operationType string

		switch operationName {
		case "query":
			if operation == "__schema" { // skip for introspect query for editor & client
				return nil, nil
			}
			_models = append(_models, &models.FilteredModel{
				Name:        operation,
				WhereFilter: nil,
			})
			operationType = "query"

			// search for sub query
		case "mutation":
			_models = append(_models, &models.FilteredModel{
				Name:        operation,
				WhereFilter: nil,
			})
			operationType = "mutation"
		}

		incomingRequest = append(incomingRequest, &models.IncomingRequest{
			OperationType:  operationType,
			FilteredModels: _models,
		})
	}

	return incomingRequest, nil
}

func modelNameExtractor(models []*models.ModelType, _name string) (string, error) {

	if _name == "__schema" || _name == "__typename" { // skip for introspect query for editor & client
		return "", nil
	}

	var name string
	if strings.Contains(_name, "_") { // in case of connection model renaming like "teaches" as "headmaster"
		splits := strings.Split(_name, "_")
		name = splits[len(splits)-1]
	} else {
		name = _name
	}

	for _, model := range models {
		if model.Name == SingularResourceName(name) {
			name = model.Name // overwrite the name with the actual model name
			break
		}
	}

	if name == "" {
		return "", fmt.Errorf("%s does not match any model. invalid query", _name)
	}

	return name, nil
}

func loopOnNestedSets(schema *models.ProjectSchema, operationName ast.Operation, sets ast.SelectionSet) ([]*models.IncomingRequest, error) {
	var incomingRequest []*models.IncomingRequest

	for _, yy := range sets {
		var _models []*models.FilteredModel
		var isFunction bool

		zz := yy.(*ast.Field)
		operation := zz.Name

		var operationType string

		var _modelData *models.FilteredModel
		switch operationName {
		case "query":
			if zz.Name == "__schema" || zz.Name == "__typename" { // skip for introspect query for editor & client
				return nil, nil
			}

			var isConnectionQuery bool
			if strings.HasSuffix(operation, "Count") {
				operation = strings.TrimSuffix(operation, "Count")
				isConnectionQuery = true
			}

			_modelData = &models.FilteredModel{
				Name:              SingularResourceName(operation),
				IsConnectionQuery: isConnectionQuery,
			}

			if len(zz.Arguments) > 0 {
				_modelData.WhereFilter = extractArguments(zz.Arguments)
			}

			_models = append(_models, _modelData)
			operationType = "query"

			// search for sub query
		case "mutation":
			var _function *models.ApitoFunction
			for _, function := range schema.Functions {
				if function.Name == operation {
					_function = function
					_models = append(_models, []*models.FilteredModel{
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
					_models = append(_models, &models.FilteredModel{
						Name:        strings.ToLower(_model),
						WhereFilter: nil,
					})
				} else if strings.HasPrefix(operation, "update") {
					_model := strings.TrimPrefix(operation, "update")
					_models = append(_models, &models.FilteredModel{
						Name:        strings.ToLower(_model),
						WhereFilter: nil,
					})
				} else if strings.HasPrefix(operation, "delete") {
					_model := strings.TrimPrefix(operation, "delete")
					_models = append(_models, &models.FilteredModel{
						Name:        strings.ToLower(_model),
						WhereFilter: nil,
					})
				} else {
					_models = append(_models, &models.FilteredModel{
						Name:        SingularResourceName(operation),
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
					name, err := modelNameExtractor(schema.Models, _zz.Name)
					if err != nil {
						return nil, err
					}
					_models = append(_models, &models.FilteredModel{
						Name: name,
					})
				}
			}
		}

		incomingRequest = append(incomingRequest, &models.IncomingRequest{
			OperationType:     operationType,
			FilteredModels:    _models,
			FilteredFunctions: nil,
		})

		/*for _, ww := range zz.SelectionSet {
			tt := ww.(*ast.Field)
			fmt.Println(tt.Name)
		}*/
	}

	return incomingRequest, nil
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
