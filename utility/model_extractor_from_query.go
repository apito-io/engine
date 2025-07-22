package utility

import (
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/vektah/gqlparser/v2/ast"
)

func ExtractModelNames(schema *models.ProjectSchema, queryDoc *ast.QueryDocument) ([]*models.IncomingRequest, bool, error) {

	_modelNames := make(map[string]bool)
	for _, model := range schema.Models {
		_modelNames[model.Name] = true
	}

	var request []*models.IncomingRequest
	var isPluginRequest bool
	for _, op := range queryDoc.Operations {
		if op.Name == "IntrospectionQuery" { // skip for introspect query for editor & client
			break
		}

		modelNames := findModelNames(_modelNames, op.SelectionSet, []*models.FilteredModel{})
		var _functions []*models.ApitoFunction
		for _, root := range op.SelectionSet {
			_field := root.(*ast.Field)
			if len(_field.Arguments) > 0 {
				for _, arg := range _field.Arguments {
					if arg.Name == "relation" {
						for _, child := range arg.Value.Children {
							if _modelNames[child.Name] {
								modelNames = append(modelNames, &models.FilteredModel{
									Name:         child.Name,
									HasMetaQuery: false,
								})
							}
							break
						}
					}
				}
			}
			if strings.HasPrefix(_field.Name, "plg_") {
				isPluginRequest = true
			} else {
				_name := SingularResourceName(_field.Name)
				if !_modelNames[_name] && _name != "__typename" {
					for _, f := range schema.Functions {
						if f.Name == _name {
							if f.Request != nil && f.Request.Model != "JSON" {
								modelNames = append(modelNames, &models.FilteredModel{
									Name:         f.Request.Model,
									HasMetaQuery: false,
								})
							}
							if f.Response != nil && f.Response.Model != "JSON" {
								modelNames = append(modelNames, &models.FilteredModel{
									Name:         f.Response.Model,
									HasMetaQuery: false,
								})
							}
							_functions = append(_functions, f)
							break
						}
					}
				}
			}
		}
		request = append(request, &models.IncomingRequest{
			OperationType:     string(op.Operation),
			FilteredModels:    models.FilterUniqueStrings(modelNames),
			FilteredFunctions: _functions,
			IsPluginRequest:   isPluginRequest,
		})
	}
	return request, isPluginRequest, nil
}

func findModelNames(modelNames map[string]bool, selectionSet ast.SelectionSet, foundModelNames []*models.FilteredModel) []*models.FilteredModel {

	for _, selection := range selectionSet {
		switch _selection := selection.(type) {
		case *ast.Field:
			name := _selection.Name

			if name == "__schema" || name == "__type" || name == "__typename" || name == "data" || name == "meta" {
				continue
			}

			// Check for "meta" node presence
			hasMeta := false
			for _, nestedSelection := range _selection.SelectionSet {
				if field, ok := nestedSelection.(*ast.Field); ok && field.Name == "meta" {
					hasMeta = true
					break
				}
			}

			name = SingularResourceName(name)
			name = ExtractRelationName(name) // for known_as relation node
			if strings.HasPrefix(name, "create") || strings.HasPrefix(name, "delete") || strings.HasPrefix(name, "update") || strings.HasPrefix(name, "upsert") {
				modelName := SingularResourceName(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(name, "create"), "delete"), "update"), "upsert"))
				if modelNames[modelName] {
					foundModelNames = append(foundModelNames, &models.FilteredModel{
						Name:         modelName,
						HasMetaQuery: hasMeta,
					})
				}
			} else if modelNames[name] {
				filteredModel := &models.FilteredModel{
					Name:         name,
					HasMetaQuery: hasMeta,
				}
				if _selection.Alias != _selection.Name { // this means we have a different alias for the model
					filteredModel.KnownAs = _selection.Alias
				}
				// assume the model is a dataloader model if it's not a root model
				if len(foundModelNames) > 0 {
					filteredModel.IsDataloaderModel = true
				}
				foundModelNames = append(foundModelNames, filteredModel)
			}

			// Always check nested selections, even for root-level fields
			nestedModels := findModelNames(modelNames, _selection.SelectionSet, foundModelNames)
			foundModelNames = append(foundModelNames, nestedModels...)
		}
	}

	return foundModelNames
}

func HasMetaQuery(name string) bool {
	return name == "meta"
}
