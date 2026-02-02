package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/scaler"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-platform/graphql"
	"github.com/teivah/onecontext"
	"github.com/vektah/gqlparser/v2/ast"
)

func remove(s []*models.ModelType, i int) []*models.ModelType {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

type ModelWithFilter struct {
	Model             *models.ModelType
	Filter            *models.FilteredModel
	HasMetaQuery      bool
	IsDataloaderModel bool
	KnownAs           string
}

func (g *GraphCtrl) publicSchemaBuilder(ctx context.Context, cache *models.ApplicationCache) (*models.ApplicationCache, error) {

	project := cache.Project
	if project.Schema == nil {
		return nil, errors.New("user Defined Schema Not Found")
	}

	if cache.Param != nil && cache.Param.Role == nil {
		return nil, errors.New("cant Build Schema Without a Role")
	}

	role := cache.Param.Role

	permissions := make(map[string]*models.APIPermission)
	var filteredModels []*ModelWithFilter
	var dataloaderModels []*ModelWithFilter
	var filteredFunctions []*models.ApitoFunction
	var operationType string

	if cache.IncomingRequest == nil {
		for _, model := range project.Schema.Models {
			modelName := model.Name
			givenPermissions, err := utility.BuildCRUDPermissions(modelName, role)
			if err != nil {
				return nil, err
			}
			if givenPermissions != nil {
				permissions[modelName] = givenPermissions
				filteredModels = append(filteredModels, &ModelWithFilter{Model: model, Filter: nil})
			}
		}
		filteredFunctions = append(filteredFunctions, project.Schema.Functions...)
	} else {
		// filter those who doesn't model have defined schema and not in the permissions
		for _, filter := range cache.IncomingRequest {
			operationType = filter.OperationType
			for _, _fm := range filter.FilteredModels {
				for _, model := range project.Schema.Models {
					if _fm.Name == model.Name {
						// just filter the type
						modelName := model.Name
						givenPermissions, err := utility.BuildCRUDPermissions(modelName, role)
						if err != nil {
							return nil, err
						}
						if givenPermissions != nil {
							permissions[modelName] = givenPermissions
							filteredModels = append(filteredModels, &ModelWithFilter{
								Model:             model,
								Filter:            _fm,
								HasMetaQuery:      _fm.HasMetaQuery,
								KnownAs:           _fm.KnownAs,
								IsDataloaderModel: _fm.IsDataloaderModel,
							})
							if _fm.IsDataloaderModel {
								dataloaderModels = append(dataloaderModels, &ModelWithFilter{
									Model:        model,
									Filter:       _fm,
									HasMetaQuery: _fm.HasMetaQuery,
									KnownAs:      _fm.KnownAs,
								})
							}
						}
					}
				}
			}
			filteredFunctions = append(filteredFunctions, filter.FilteredFunctions...)
		}
	}

	if len(filteredFunctions) == 0 && len(filteredModels) == 0 { // if defined model is null and not function query then something is wrong
		return nil, errors.New("query not found in schema. please re-check")
	}

	var allLoaders = make(map[string]*dataloader.Loader)

	// global meta cache | used for created_by and last_modified_by meta loading information
	allLoaders["system_user_loader"] = dataloader.NewBatchedLoader(g.gqlServer.SystemUserMetaLoader)

	// build all fields without connection
	commonFields := make(map[string]graphql.Fields)
	aggregateFields := make(map[string]graphql.Fields)
	commonMutationFieldsConfigArgs := make(map[string]graphql.FieldConfigArgument)

	localEnum := enums.BuildLocalEnum(project.Settings.Locals)

	metaObject := objects.BuildMetaObject(ctx, project.ID)

	connectionParamArgs := make(map[string]*graphql.InputObject)
	whereArgs := make(map[string]graphql.InputObjectConfigFieldMap)
	whereRelationArgs := make(map[string]*graphql.InputObject)
	sortParam := make(map[string]graphql.InputObjectConfigFieldMap)

	createMutationFieldsArguments := make(map[string]graphql.InputObjectConfigFieldMap)
	updateMutationFieldsArguments := make(map[string]graphql.InputObjectConfigFieldMap)
	connectionFields := make(map[string]graphql.InputObjectConfigFieldMap)

	// generates common fields
	for _, _definedModel := range filteredModels {

		definedModel := _definedModel.Model

		// build fields
		// for permission based insert user_id , tenant_id if necessary
		if permission, ok := permissions[definedModel.Name]; (ok && permission != nil) || cache.Param.Role.IsAdmin {

			// #todo caching field for extra performance
			// but got a problem in cacheing
			// unsupported type: graphql.IsTypeOfFn
			/*if val, err := g.gqlServer.GetCacheGraphQLFieldsGeneration(ctx, project.ID, definedModel.Name); val != nil && err == nil {
				queryBuilderInformation = *val
			} else {*/

			// fields
			queryBuilderInformation := resolver.QueryBuilderInformation{
				DataObjects:      make(graphql.Fields),
				AggregateObjects: make(graphql.Fields),
				//ConnectionParamObjects: make(map[string]*graphql.InputObject),
				WhereParamObjects: make(graphql.InputObjectConfigFieldMap),
				SortParamObjects:  make(graphql.InputObjectConfigFieldMap),
			}

			for _, f := range definedModel.Fields {

				if !strings.HasPrefix(f.Identifier, "system_") { // ignore system_ fields for data and aggregate
					queryBuilderInformation.DataObjects[f.Identifier] = g.GetGraphQLField(definedModel.Name+"_query", f, false)
					if agv := g.GetGraphQLAggregateField(definedModel.Name+"_aggregate", f, false); agv != nil {
						queryBuilderInformation.AggregateObjects[f.Identifier] = agv
					}
				}

				// don't model include media type in where filter
				if f.FieldType == "media" { // skip hide and media field from where condition
					continue
				}

				//if _definedModel.Filter != nil && len(_definedModel.Filter.WhereFilter) > 0 {

				//if !utility.ArrayContains(_definedModel.Filter.WhereFilter, f.Identifier) {
				//	continue // skip if user is not demanding it
				//}

				queryBuilderInformation.WhereParamObjects[f.Identifier] = &graphql.InputObjectFieldConfig{
					Type: objects.BuildWhereConditionArgument(definedModel.Name+"_"+f.Identifier, f),
				}
				//}

				// build sort param object
				if f.FieldType == "boolean" || f.FieldType == "list" { // skip hide and media field from sort
					continue
				}
				queryBuilderInformation.SortParamObjects[f.Identifier] = &graphql.InputObjectFieldConfig{
					Type: objects.BuildSortConditionArgument(definedModel.Name+"_"+f.Identifier, f),
				}
			}

			/*	fmt.Println(fmt.Sprintf("caching fields %#v", queryBuilderInformation.DataObjects))
				err = g.gqlServer.CacheGraphQLFieldsGeneration(ctx, project.ID, definedModel.Name, &queryBuilderInformation)
				if err != nil {
					return nil, err
				}
			}*/

			if len(definedModel.Connections) > 0 {
				connectionParamArgs[definedModel.Name] = objects.BuildConnectionArguments(definedModel.Name, definedModel.Connections)
			}
			whereArgs[definedModel.Name] = queryBuilderInformation.WhereParamObjects
			sortParam[definedModel.Name] = queryBuilderInformation.SortParamObjects

			// overwrite the meta if user queries it
			if _definedModel.HasMetaQuery {
				metaObject = objects.BuildMetaObject(ctx, project.ID)
			}

			responseFields := graphql.Fields{
				"id": &graphql.Field{
					Type: graphql.String,
				},
				"meta": metaObject,
				"data": &graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name:   strings.Title(definedModel.Name) + "_RawModel",
						Fields: queryBuilderInformation.DataObjects,
					}),
					Description: fmt.Sprintf("Field Details"),
				},
				"relation_doc_id": &graphql.Field{
					Type: graphql.String,
				},
			}

			commonFields[definedModel.Name] = responseFields
			if len(queryBuilderInformation.AggregateObjects) > 0 {
				aggregateFields[definedModel.Name] = queryBuilderInformation.AggregateObjects
			}

			cdata := graphql.InputObjectConfigFieldMap{}
			udata := graphql.InputObjectConfigFieldMap{}
			for _, f := range definedModel.Fields {

				if definedModel.Name == "user" && f.Identifier == "role" { // don't model expose the role filed in the public api
					continue
				}

				cdata[f.Identifier] = g.GetGraphQLArgumentObjectField(definedModel.Name, f, false)
				udata[f.Identifier] = g.GetGraphQLArgumentObjectField("update_"+definedModel.Name, f, true)
				/*if f.Validation != nil && len(f.Validation.Locals) > 0 {
					for _, l := range f.Validation.Locals {
						createMutationFieldsArguments[fmt.Sprintf(`%s_%definedModel`, f.Name, l)] = utility.GetGraphQLArgumentObjectField(definedModel.Name, f)
					}
				}*/
			}

			createMutationFieldsArguments[definedModel.Name] = cdata
			updateMutationFieldsArguments[definedModel.Name] = udata

			if len(definedModel.Connections) > 0 {

				mutationFieldsConnectionArgument := graphql.InputObjectConfigFieldMap{}
				for _, conn := range definedModel.Connections {

					/*if conn.ProtectedRelation { // don'model add that to connect we will connect it from the token
						continue
					}*/
					var modelName string
					if conn.KnownAs != "" {
						modelName = conn.KnownAs
					} else {
						modelName = conn.Model
					}

					var idFieldName string
					var idField graphql.Type
					switch conn.Relation {
					case "has_one":
						idFieldName = modelName + "_id"
						idField = graphql.String
					case "has_many":
						idFieldName = modelName + "_ids"
						idField = graphql.NewList(graphql.String)
					}

					if connPermission, ok := permissions[conn.Model]; (ok && connPermission != nil) || cache.Param.Role.IsAdmin {
						switch permission.Create {
						case "tenant":
							idField = graphql.NewNonNull(idField)
							break
						case "own":
							idField = graphql.NewNonNull(idField)
							break
						}
					}

					mutationFieldsConnectionArgument[idFieldName] = &graphql.InputObjectFieldConfig{
						Type: idField,
					}

				}

				// Store the connection fields for later use in upsert
				connectionFields[definedModel.Name] = mutationFieldsConnectionArgument

				commonMutationFieldsConfigArgs[definedModel.Name] = graphql.FieldConfigArgument{
					"connect": &graphql.ArgumentConfig{
						Type: graphql.NewInputObject(graphql.InputObjectConfig{
							Name:   strings.Title(definedModel.Name + "_Relation_Connect_Payload"),
							Fields: mutationFieldsConnectionArgument,
						}),
					},
					"disconnect": &graphql.ArgumentConfig{
						Type: graphql.NewInputObject(graphql.InputObjectConfig{
							Name:   strings.Title(definedModel.Name + "_Relation_Disconnect_Payload"),
							Fields: mutationFieldsConnectionArgument,
						}),
					},
					"local": &graphql.ArgumentConfig{
						Type: localEnum,
					},
					"status": &graphql.ArgumentConfig{
						Type: enums.PublishStatusEnums,
					},
				}
			} else {
				commonMutationFieldsConfigArgs[definedModel.Name] = graphql.FieldConfigArgument{
					"local": &graphql.ArgumentConfig{
						Type: localEnum,
					},
				}
			}
		}
	}

	// build whereRelationArgs

	for _, _definedModel := range filteredModels {
		definedModel := _definedModel.Model
		whereRelationArgs[definedModel.Name] = objects.BuildWhereRelationConditionArgument(definedModel.Name, definedModel.Connections, whereArgs)
	}

	// this temp Connection is used to pass relevant connection data to the dataloader
	// specially used when known_as is used in the query
	tempConnection := make(map[string]*models.ConnectionType)

	// add the connection fields / dataloader fields
	for _, _definedModel := range filteredModels {

		definedModel := _definedModel.Model

		fields := commonFields[definedModel.Name]
		for _, connection := range definedModel.Connections {

			if permission, ok := permissions[connection.Model]; ok && permission != nil || (permission != nil && cache.Param.Role.IsAdmin) {
				if permission.Read == "none" {
					continue
				}
				switch connection.Relation {
				case "has_one":

					var modelName string
					if connection.KnownAs != "" {
						modelName = connection.KnownAs
					} else {
						modelName = connection.Model
					}

					// add a dataloader
					allLoaders[modelName] = dataloader.NewBatchedLoader(g.gqlServer.DataLoaderFn)

					tempConnection[modelName] = connection

					_field := &graphql.Field{
						Name: modelName,
						Type: graphql.NewObject(graphql.ObjectConfig{
							Name:        definedModel.Name + "_has_one_" + modelName + "_connection",
							Fields:      commonFields[connection.Model],
							Description: "has_one",
						}),
						Description: fmt.Sprintf("Has one %v", modelName),
					}

					_field.Resolve = func(p graphql.ResolveParams) (interface{}, error) {
						// get title from source
						source := p.Source.(*types.DefaultDocumentStructure)

						_path := p.Info.Path.Key
						modelName = _path.(string) // overwrite the model name

						//fmt.Println(source)
						var (
							v                = p.Context.Value
							loaders          = v("cache").(*models.ApplicationCache).Dataloaders
							rootSelectionSet = v("selectionSet").(ast.SelectionSet)
							key              = models.NewResolverKey(source.ID, nil)
							lid              = modelName
							knownAs          = _definedModel.KnownAs
						)

						var selectionSet *ast.SelectionSet
						for _, s := range rootSelectionSet {
							if val := s.(*ast.Field); utility.SingularResourceName(val.Name) == lid {
								selectionSet = &val.SelectionSet
								break
							}
						}

						typeRelation := context.WithValue(ctx, "relation_meta", map[string]interface{}{
							"project_id":    project.ID,
							"relation_type": "has_one", // need this info in the dataloader
							"resolveParam":  &p,
							"connection":    tempConnection[modelName],
							"selectionSet":  selectionSet,
							"knownAs":       knownAs,
						})
						tx, closeContext := onecontext.Merge(p.Context, typeRelation)
						defer closeContext()
						// #todo we did a temp fix
						// #todo need to avoid unnecessary resolver to add to data cache Ex. posts' defined Model user query also adding comment in resolvers !!
						if val, ok := loaders[lid]; ok {
							thunk := val.Load(tx, key)
							return func() (interface{}, error) {
								return thunk()
							}, nil
						}

						return func() (interface{}, error) {
							return nil, nil
						}, nil
					}

					fields[connection.Model] = _field
					break
				case "has_many":

					var modelName string
					if connection.KnownAs != "" {
						modelName = connection.KnownAs
					} else {
						modelName = connection.Model
					}

					// add a dataloader
					allLoaders[utility.MultipleResourceName(modelName)] = dataloader.NewBatchedLoader(g.gqlServer.DataLoaderFn)

					fields[utility.MultipleResourceName(modelName)] = &graphql.Field{
						//Name: utility.MultipleResourceName(definedModel.Name + "_" + connection.Name),
						Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
							Name:        definedModel.Name + "_has_many_" + utility.MultipleResourceName(definedModel.Name+"_"+modelName) + "_connections",
							Fields:      commonFields[utility.SingularResourceName(connection.Model)],
							Description: "has_many",
						})),
						Args:        objects.BuildFilterArgument(localEnum, utility.MultipleResourceName(definedModel.Name+"_"+modelName), connectionParamArgs[connection.Model], whereArgs[connection.Model], whereRelationArgs[connection.Model], sortParam[connection.Model]),
						Description: fmt.Sprintf("Has many %v", connection.Model),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							// get title from source
							source := p.Source.(*types.DefaultDocumentStructure)
							//fmt.Println(source)
							var (
								v                = p.Context.Value
								loaders          = v("cache").(*models.ApplicationCache).Dataloaders
								rootSelectionSet = v("selectionSet").(ast.SelectionSet)
								key              = models.NewResolverKey(source.ID, nil)
								lid              = utility.MultipleResourceName(p.Info.FieldName)
								knownAs          = _definedModel.KnownAs
							)

							var selectionSet *ast.SelectionSet
							for _, s := range rootSelectionSet {
								if val := s.(*ast.Field); utility.SingularResourceName(val.Name) == source.Type {
									for _, s := range val.SelectionSet {
										if _s := s.(*ast.Field); _s.Name == lid {
											selectionSet = &_s.SelectionSet
											break
										}
									}
									break
								}
							}

							typeRelation := context.WithValue(ctx, "relation_meta", map[string]interface{}{
								"project_id":    project.ID,
								"relation_type": "has_many",
								"resolveParam":  &p,
								"connection":    connection,
								"selectionSet":  selectionSet,
								"knownAs":       knownAs,
							})
							tx, closeContext := onecontext.Merge(p.Context, typeRelation)
							defer closeContext()
							thunk := loaders[lid].Load(tx, key)
							return func() (interface{}, error) {
								return thunk()
							}, nil
						},
					}
					break
				}
			}
		}
		// save it back
		commonFields[definedModel.Name] = fields
	}

	// build mutation objects
	queryTypes := graphql.Fields{}
	mutationTypes := graphql.Fields{}
	for _, _definedModel := range filteredModels {

		definedModel := _definedModel.Model

		// build query
		if permission, ok := permissions[definedModel.Name]; (ok && permission != nil) || cache.Param.Role.IsAdmin {

			if permission.Read == "none" {
				continue
			}

			arg := graphql.FieldConfigArgument{
				"local": &graphql.ArgumentConfig{
					Type: localEnum,
				}}

			if definedModel.SinglePage || definedModel.IsTenantModel {
				arg["_id"] = &graphql.ArgumentConfig{
					Type: graphql.String,
				}
			} else {
				arg["_id"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				}
			}

			// single query type
			singleQueryType := graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name:   strings.Title(definedModel.Name),
					Fields: commonFields[definedModel.Name],
				}),
				Args:    arg,
				Resolve: g.gqlServer.SingleResourceResolverFn,
			}

			queryTypes[definedModel.Name] = &singleQueryType

			if !definedModel.SinglePage {
				// add the plural query type
				pluralQueryType := graphql.Field{
					Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
						Name:   strings.Title(utility.MultipleResourceName(definedModel.Name)),
						Fields: commonFields[definedModel.Name],
					})),
					Args:    objects.BuildFilterArgument(localEnum, strings.Title(utility.MultipleResourceName(definedModel.Name)), connectionParamArgs[definedModel.Name], whereArgs[definedModel.Name], whereRelationArgs[definedModel.Name], sortParam[definedModel.Name]),
					Resolve: g.gqlServer.MultiResourceResolverFn,
				}
				queryTypes[utility.MultipleResourceName(definedModel.Name)] = &pluralQueryType

				if _definedModel.Filter != nil && _definedModel.Filter.IsConnectionQuery {
					// filter connection
				}
				countFields := graphql.Fields{
					"total": &graphql.Field{
						Type:    graphql.Int,
						Resolve: g.gqlServer.CountResolverFn,
					},
				}
				if len(aggregateFields[definedModel.Name]) > 0 {
					countFields["aggregate"] = &graphql.Field{
						Type: graphql.NewObject(graphql.ObjectConfig{
							Name:   strings.Title(utility.MultipleResourceName(definedModel.Name) + "_Aggregate"),
							Fields: aggregateFields[definedModel.Name],
						}),
						Resolve: g.gqlServer.AggregateResolverFn,
					}

					// for groupby add an extra key called group_by_identifier to aggregate fields

					aggregateFields[definedModel.Name]["group_by_identifier_1"] = &graphql.Field{
						Type: graphql.String,
					}
					aggregateFields[definedModel.Name]["group_by_identifier_2"] = &graphql.Field{
						Type: graphql.String,
					}

					countFields["groupBy"] = &graphql.Field{
						Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
							Name:   strings.Title(utility.MultipleResourceName(definedModel.Name) + "_Aggregate_GroupBy"),
							Fields: aggregateFields[definedModel.Name],
						})),
						Resolve: g.gqlServer.AggregateResolverFn,
					}
				}
				connectionQueryType := graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name:   strings.Title(utility.MultipleResourceName(definedModel.Name) + "_Connection"),
						Fields: countFields,
					}),
					Args: objects.BuildFilterArgument(localEnum, strings.Title(utility.MultipleResourceName(definedModel.Name)+"_Count"), connectionParamArgs[definedModel.Name], whereArgs[definedModel.Name], whereRelationArgs[definedModel.Name], sortParam[definedModel.Name]),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p, nil
					},
				}
				queryTypes[utility.MultipleResourceName(definedModel.Name)+"Count"] = &connectionQueryType
			}
		}

		// build mutation
		if permission, ok := permissions[definedModel.Name]; (operationType == "" || operationType == "mutation") && ((ok && permission != nil) || cache.Param.Role.IsAdmin) {

			// delete mutation
			if permission.Delete != "none" {
				deleteArgs := make(graphql.FieldConfigArgument)
				deleteArgs["_ids"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
				}

				DeleteMutationType := graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "Delete_" + strings.Title(definedModel.Name),
						Fields: graphql.Fields{
							"response": &graphql.Field{
								Type: graphql.String,
							},
						},
					}),
					Args:    deleteArgs,
					Resolve: g.gqlServer.MutationResolverFn,
				}
				mutationTypes["delete"+strings.Title(definedModel.Name)] = &DeleteMutationType
			}

			// update
			if permission.Update != "none" {

				commonMutationFieldsConfigArgs[definedModel.Name]["payload"] = &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   strings.Title(definedModel.Name + "_Update_Payload"),
						Fields: updateMutationFieldsArguments[definedModel.Name],
					}),
				}

				updateArgs := make(graphql.FieldConfigArgument)
				updateArgs["_id"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				}
				updateArgs["keepRevision"] = &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				}
				updateArgs["deltaUpdate"] = &graphql.ArgumentConfig{
					Type:        graphql.Boolean,
					Description: "Delta update mode. Use if to upate array with _id",
				}
				// copy the common with it
				for k, v := range commonMutationFieldsConfigArgs[definedModel.Name] {
					updateArgs[k] = v
				}

				arg := updateArgs

				UpdateMutationType := graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name:   "Update_" + strings.Title(definedModel.Name),
						Fields: commonFields[definedModel.Name],
					}),
					Args:    arg,
					Resolve: g.gqlServer.MutationResolverFn,
				}
				mutationTypes["update"+strings.Title(definedModel.Name)] = &UpdateMutationType
			}

			// create
			if permission.Create != "none" && definedModel.Name != "user" { // skip createUser because we have userRegister

				// Create args for singular mutation
				arg := make(graphql.FieldConfigArgument)
				for k, v := range commonMutationFieldsConfigArgs[definedModel.Name] {
					arg[k] = v
				}
				arg["payload"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   strings.Title(definedModel.Name + "_Create_Payload"),
						Fields: createMutationFieldsArguments[definedModel.Name],
					})),
				}
				delete(arg, "disconnect") // no disconnect field necessary during create operation

				singleCreateMutationType := graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name:   "Create_" + strings.Title(definedModel.Name),
						Fields: commonFields[definedModel.Name],
					}),
					Args:    arg,
					Resolve: g.gqlServer.MutationResolverFn,
				}

				// Create separate args for plural mutation
				pluralArgs := make(graphql.FieldConfigArgument)
				for k, v := range commonMutationFieldsConfigArgs[definedModel.Name] {
					pluralArgs[k] = v
				}

				// inject _id, _connect, _disconnect field to the payload
				upsertPayloadFields := createMutationFieldsArguments[definedModel.Name]
				upsertPayloadFields["_id"] = &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				}

				if connectionFieldsForModel, hasConnections := connectionFields[definedModel.Name]; hasConnections {
					upsertPayloadFields["_connect"] = &graphql.InputObjectFieldConfig{
						Type: graphql.NewInputObject(graphql.InputObjectConfig{
							Name:   strings.Title(utility.MultipleResourceName(definedModel.Name) + "_Connect"),
							Fields: connectionFieldsForModel,
						}),
					}
				}

				pluralArgs["payloads"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   strings.Title(utility.MultipleResourceName(definedModel.Name) + "_Upsert_Payload"),
						Fields: upsertPayloadFields,
					})))),
				}
				//delete(pluralArgs, "disconnect")
				delete(pluralArgs, "payload")
				//delete(pluralArgs, "connect") // keep this for now for backward compatibility
				//delete(pluralArgs, "disconnect") // keep this for now for backward compatibility

				pluralCreateMutationType := graphql.Field{
					Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
						Name:   "Upsert_" + strings.Title(utility.MultipleResourceName(definedModel.Name)),
						Fields: commonFields[definedModel.Name],
					})),
					Args:    pluralArgs,
					Resolve: g.gqlServer.MutationResolverFn,
				}

				mutationTypes["create"+strings.Title(definedModel.Name)] = &singleCreateMutationType
				mutationTypes["upsert"+strings.Title(utility.MultipleResourceName(definedModel.Name))] = &pluralCreateMutationType
			}
		}

	}

	// build function query
	if len(filteredFunctions) > 0 ||
		cache.GraphqlRequest.OperationName == "IntrospectionQuery" ||
		len(role.LogicExecutions) > 0 {
		// logic for custom resolver / functions
		for _, s := range filteredFunctions {
			if s.FunctionProviderID != "" && !strings.HasPrefix(s.Name, "plg_") {
				continue
			}

			// if permitted in the role then add to query
			if utility.ArrayContains(role.LogicExecutions, s.Name) || role.IsAdmin {
				if s.Request != nil && s.Response != nil {
					var _argType graphql.Input
					switch s.Request.Model {
					case "CUSTOM":
						/*inputs := make(graphql.InputObjectConfigFieldMap)
						for _, f := range definedModel.Request.Params {
							inputs[f.Identifier] = utility.GetGraphQLArgumentObjectField(definedModel.Name, f, false)
						}*/
						return nil, errors.New("not Supported Yet")
					case "JSON":
						_argType = scaler.ScalarJSONWithRequest(s.Name, cache.GraphqlRequest)
					default:
						_argType = graphql.NewInputObject(graphql.InputObjectConfig{
							Name:   s.Name + "_Input_Payload",
							Fields: updateMutationFieldsArguments[s.Request.Model],
						})
					}

					var _arg graphql.FieldConfigArgument
					if s.Request.OptionalPayload {
						_arg = graphql.FieldConfigArgument{
							"payload": &graphql.ArgumentConfig{
								Type: _argType,
							},
						}
					} else {
						_arg = graphql.FieldConfigArgument{
							"payload": &graphql.ArgumentConfig{
								Type: graphql.NewNonNull(_argType),
							},
						}
					}

					var _type graphql.Type
					switch s.Response.Model {
					case "CUSTOM":
						return nil, errors.New("not Supported Yet")
					case "JSON":
						_type = graphql.NewObject(graphql.ObjectConfig{
							Name: s.Name,
							Fields: graphql.Fields{
								"JSON": &graphql.Field{
									Type: scaler.ScalarJSON,
								},
							},
						})
					default:
						if s.Response.IsArray {
							_type = graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
								Name:   s.Name,
								Fields: commonFields[utility.SingularResourceName(s.Response.Model)],
							}))
						} else {
							_type = graphql.NewObject(graphql.ObjectConfig{
								Name:   s.Name,
								Fields: commonFields[utility.SingularResourceName(s.Response.Model)],
							})
						}
					}
					switch s.GraphQLSchemaType {
					case "Query":
						queryTypes[s.Name] = &graphql.Field{
							Type:    _type,
							Args:    _arg,
							Resolve: g.gqlServer.ApitoFunctionResolverFn,
						}
					case "Mutation":
						mutationTypes[s.Name] = &graphql.Field{
							Type:    _type,
							Args:    _arg,
							Resolve: g.gqlServer.ApitoFunctionResolverFn,
						}
					default:
						queryTypes[s.Name] = &graphql.Field{
							Type:    _type,
							Args:    _arg,
							Resolve: g.gqlServer.ApitoFunctionResolverFn,
						}
					}
				}
			}
		}
	}

	// Copy Third Party Plugin Queries & Mutations
	if cache.RawSchemas != nil {
		for k, v := range cache.RawSchemas.Queries {
			if _, ok := queryTypes[k]; !ok && v != nil {
				queryTypes[k] = v
			} else {
				fmt.Println(fmt.Sprintf(`the system already has a query named '%s'. please check/choose a different name for extension query. ignoring this one.`, k))
			}
		}

		for k, v := range cache.RawSchemas.Mutations {
			mutationTypes[k] = v
		}
	}

	// for error handing "Schema query must be Object Type but got: nil."
	// so if no query is found then add a dummy query
	if len(queryTypes) == 0 {
		queryTypes = graphql.Fields{
			"foo": &graphql.Field{
				Type: graphql.NewList(graphql.String),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "bar", nil
				},
			},
		}
	}

	return &models.ApplicationCache{
		Project: cache.Project,
		RawSchemas: &models.RawSchema{
			Queries:   queryTypes,
			Mutations: mutationTypes,
		},
		Dataloaders: allLoaders,
	}, nil
}
