package controller

import (
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// collectFilteredModelsForPublicSchema applies role permissions and optional incoming request filters
// to produce the model list used for the rest of publicSchemaBuilder.
func collectFilteredModelsForPublicSchema(
	project *models.Project,
	cache *models.ApplicationCache,
	role *models.Role,
) (
	permissions map[string]*models.APIPermission,
	filteredModels []*models.PublicSchemaModelFilter,
	filteredFunctions []*models.ApitoFunction,
	operationType string,
	err error,
) {
	permissions = make(map[string]*models.APIPermission)

	if cache.IncomingRequest == nil {
		for _, model := range project.Schema.Models {
			if models.ModelIsProjectAuthUserModel(model) || models.ModelIsSaaSTenantControlPlaneModel(model) {
				continue
			}
			modelName := model.Name
			givenPermissions, e := utility.BuildCRUDPermissions(modelName, role)
			if e != nil {
				return nil, nil, nil, "", e
			}
			if givenPermissions != nil {
				permissions[modelName] = givenPermissions
				filteredModels = append(filteredModels, &models.PublicSchemaModelFilter{Model: model, Filter: nil})
			}
		}
		filteredFunctions = append(filteredFunctions, project.Schema.Functions...)
	} else {
		for _, filter := range cache.IncomingRequest {
			operationType = filter.OperationType
			for _, _fm := range filter.FilteredModels {
				for _, model := range project.Schema.Models {
					if models.ModelIsProjectAuthUserModel(model) || models.ModelIsSaaSTenantControlPlaneModel(model) {
						continue
					}
					if _fm.Name == model.Name {
						modelName := model.Name
						givenPermissions, e := utility.BuildCRUDPermissions(modelName, role)
						if e != nil {
							return nil, nil, nil, "", e
						}
						if givenPermissions != nil {
							permissions[modelName] = givenPermissions
							filteredModels = append(filteredModels, &models.PublicSchemaModelFilter{
								Model:             model,
								Filter:            _fm,
								HasMetaQuery:      _fm.HasMetaQuery,
								KnownAs:           _fm.KnownAs,
								IsDataloaderModel: _fm.IsDataloaderModel,
							})
						}
					}
				}
			}
			filteredFunctions = append(filteredFunctions, filter.FilteredFunctions...)
		}
	}

	return permissions, filteredModels, filteredFunctions, operationType, nil
}
