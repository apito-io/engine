package controller

import (
	"context"
	"errors"
	"fmt"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/scaler"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/graph-gophers/dataloader"
	"github.com/jinzhu/inflection"
	"github.com/tailor-inc/graphql"
	"github.com/teivah/onecontext"
	"github.com/vektah/gqlparser/v2/ast"
	"strings"
)

func remove(s []*protobuff.ModelType, i int) []*protobuff.ModelType {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

type ModelWithFilter struct {
	Model        *protobuff.ModelType
	Filter       *shared.FilteredModel
	HasMetaQuery bool
}

func (g *GraphCtrl) publicSchemaBuilder(ctx context.Context, cache *shared.ApplicationCache) (*shared.ApplicationCache, error) {

	project := cache.Project
	if project.Schema == nil {
		return nil, errors.New("user Defined Schema Not Found")
	}

	var hasFunctionInQuery bool

	var definedModels []*ModelWithFilter
	// filter those who doesn't model have defined schema and not in the permissions
	for _, model := range project.Schema.Models {
		if model.Fields != nil && cache.IncomingRequest == nil { // just filter the type
			definedModels = append(definedModels, &ModelWithFilter{Model: model, Filter: nil})
		} else {
			if model.Fields != nil && len(cache.IncomingRequest) > 0 {
				for _, filter := range cache.IncomingRequest {
					if filter.IsFunction {
						hasFunctionInQuery = true
					}
					for _, _fm := range filter.FilteredModels {
						if _fm.Name == model.Name {
							// just filter the type
							definedModels = append(definedModels, &ModelWithFilter{
								Model:        model,
								Filter:       _fm,
								HasMetaQuery: _fm.HasMetaQuery,
							})
						}
					}
				}
			}
		}
	}

	if len(definedModels) == 0 {
		return nil, errors.New("query not found in schema. please re-check")
	}

	var allLoaders = make(map[string]*dataloader.Loader)

	// global meta cache
	allLoaders["meta_loader"] = dataloader.NewBatchedLoader(g.gqlServer.MetaLoaderFn)

	// build all fields without connection
	commonFields := make(map[string]graphql.Fields)
	commonMutationFieldsConfigArgs := make(map[string]graphql.FieldConfigArgument)

	var locals = make(graphql.EnumValueConfigMap)
	for _, local := range project.Locals {
		locals[local] = &graphql.EnumValueConfig{
			Value:       local,
			Description: fmt.Sprintf(`%s local support`, local),
		}
	}

	localEnum := enums.BuildLocalEnum(locals)

	metaObject := objects.BuildMetaObject(ctx, project.Id, false)
	whereArgs := make(map[string]graphql.InputObjectConfigFieldMap)
	sortParam := make(map[string]graphql.InputObjectConfigFieldMap)

	createMutationFieldsArguments := make(map[string]graphql.InputObjectConfigFieldMap)
	updateMutationFieldsArguments := make(map[string]graphql.InputObjectConfigFieldMap)

	// generates common fields
	for _, _definedModel := range definedModels {

		definedModel := _definedModel.Model

		// build fields
		// fields
		queryBuilderInformation := resolver.QueryBuilderInformation{
			DataObjects:       make(graphql.Fields),
			WhereParamObjects: make(graphql.InputObjectConfigFieldMap),
			SortParamObjects:  make(graphql.InputObjectConfigFieldMap),
		}
		// #todo caching field for extra performance
		// but got a problem in cacheing
		// unsupported type: graphql.IsTypeOfFn
		/*if val, err := g.gqlServer.GetCacheGraphQLFieldsGeneration(ctx, project.Id, definedModel.Name); val != nil && err == nil {
			queryBuilderInformation = *val
		} else {*/
		for _, f := range definedModel.Fields {
			if f.Validation != nil && f.Validation.Hide { // skip hide and media field from filter
				continue
			}

			queryBuilderInformation.DataObjects[f.Identifier] = g.GetGraphQLField(definedModel.Name+"_query", f, false)

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
			err = g.gqlServer.CacheGraphQLFieldsGeneration(ctx, project.Id, definedModel.Name, &queryBuilderInformation)
			if err != nil {
				return nil, err
			}
		}*/

		whereArgs[definedModel.Name] = queryBuilderInformation.WhereParamObjects
		sortParam[definedModel.Name] = queryBuilderInformation.SortParamObjects

		// overwrite the meta if user queries it
		if _definedModel.HasMetaQuery {
			metaObject = objects.BuildMetaObject(ctx, project.Id, true)
		}

		responseFields := graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"meta": metaObject,
			"data": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name:   definedModel.Name + "_Common_DataFields",
					Fields: queryBuilderInformation.DataObjects,
				}),
				Description: fmt.Sprintf("Meta Fields"),
			},
		}

		commonFields[definedModel.Name] = responseFields

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
					break
				case "has_many":
					idFieldName = modelName + "_ids"
					idField = graphql.NewList(graphql.String)
					break
				}

				mutationFieldsConnectionArgument[idFieldName] = &graphql.InputObjectFieldConfig{
					Type: idField,
				}
			}

			commonMutationFieldsConfigArgs[definedModel.Name] = graphql.FieldConfigArgument{
				"connect": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   definedModel.Name + "_input_connection_payload",
						Fields: mutationFieldsConnectionArgument,
					}),
				},
				"disconnect": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   definedModel.Name + "_input_disconnection_payload",
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

	// add the connection fields
	for _, _definedModel := range definedModels {

		definedModel := _definedModel.Model

		fields := commonFields[definedModel.Name]
		for _, connection := range definedModel.Connections {

			var modelName string
			if connection.KnownAs != "" && (connection.Type == "forward" || connection.Type == "backward") {
				modelName = fmt.Sprintf(`%s_%s`, connection.KnownAs, connection.Model)
			} else {
				modelName = connection.Model
			}

			switch connection.Relation {
			case "has_one":

				// add a dataloader
				allLoaders[modelName] = dataloader.NewBatchedLoader(g.gqlServer.DataLoaderFn)

				fields[modelName] = &graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name:        definedModel.Name + "_has_one_" + modelName + "_connection",
						Fields:      commonFields[connection.Model],
						Description: "has_one",
					}),
					Description: fmt.Sprintf("Has one %v", connection.Model),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// get title from source
						source := p.Source.(*shared.DefaultDocumentStructure)
						//fmt.Println(source)
						var (
							v                = p.Context.Value
							loaders          = v("loaders").(map[string]*dataloader.Loader)
							rootSelectionSet = v("selectionSet").(ast.SelectionSet)
							key              = models.NewResolverKey(source.Id, nil)
							lid              = p.Info.FieldName
						)

						var selectionSet *ast.SelectionSet
						for _, s := range rootSelectionSet {
							if val := s.(*ast.Field); inflection.Singular(val.Name) == lid {
								selectionSet = &val.SelectionSet
								break
							}
						}

						typeRelation := context.WithValue(ctx, "relation_meta", map[string]interface{}{
							"project_id":    project.Id,
							"relation_type": "has_one", // need this info in the dataloader
							"resolveParam":  &p,
							"connection":    connection,
							"selectionSet":  selectionSet,
						})
						tx, closeContext := onecontext.Merge(p.Context, typeRelation)
						defer closeContext()
						// #todo we did a temp fix
						// #todo need to avoid unnecessary resolver to add to data cache Ex. posts'definedModel user query also adding comment in resolvers !!
						if val, ok := loaders[lid]; ok {
							thunk := val.Load(tx, key)
							return func() (interface{}, error) {
								return thunk()
							}, nil
						}

						return func() (interface{}, error) {
							return nil, nil
						}, nil
					},
				}

				break
			case "has_many":

				// add a dataloader
				allLoaders[inflection.Plural(modelName)] = dataloader.NewBatchedLoader(g.gqlServer.DataLoaderFn)

				fields[inflection.Plural(modelName)] = &graphql.Field{
					//Name: inflection.Plural(definedModel.Name + "_" + connection.Name),
					Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
						Name:        definedModel.Name + "_has_many_" + inflection.Plural(definedModel.Name+"_"+modelName) + "_connections",
						Fields:      commonFields[inflection.Singular(connection.Model)],
						Description: "has_many",
					})),
					Args:        objects.BuildFilterArgument(localEnum, inflection.Plural(definedModel.Name+"_"+modelName), whereArgs[connection.Model], sortParam[connection.Model]),
					Description: fmt.Sprintf("Has many %v", connection.Model),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// get title from source
						source := p.Source.(*shared.DefaultDocumentStructure)
						//fmt.Println(source)
						var (
							v                = p.Context.Value
							loaders          = v("cache").(*shared.ApplicationCache).Dataloaders
							rootSelectionSet = v("selectionSet").(ast.SelectionSet)
							key              = models.NewResolverKey(source.Id, nil)
							lid              = inflection.Plural(p.Info.FieldName)
						)

						var selectionSet *ast.SelectionSet
						for _, s := range rootSelectionSet {
							if val := s.(*ast.Field); inflection.Singular(val.Name) == source.Type {
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
							"project_id":    project.Id,
							"relation_type": "has_many",
							"resolveParam":  &p,
							"connection":    connection,
							"selectionSet":  selectionSet,
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
		// save it back
		commonFields[definedModel.Name] = fields
	}

	// build mutation objects
	queryTypes := graphql.Fields{}
	mutationTypes := graphql.Fields{}
	for _, _definedModel := range definedModels {

		definedModel := _definedModel.Model

		// build query

		arg := graphql.FieldConfigArgument{
			"local": &graphql.ArgumentConfig{
				Type: localEnum,
			}}

		if !definedModel.SinglePage {
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
			Resolve: g.gqlServer.RootResolverFn,
		}

		queryTypes[definedModel.Name] = &singleQueryType

		if !definedModel.SinglePage {
			// add the plural query type
			pluralQueryType := graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name:   strings.Title(inflection.Plural(definedModel.Name)),
					Fields: commonFields[definedModel.Name],
				})),
				Args:    objects.BuildFilterArgument(localEnum, strings.Title(inflection.Plural(definedModel.Name)), whereArgs[definedModel.Name], sortParam[definedModel.Name]),
				Resolve: g.gqlServer.RootResolverFn,
			}
			queryTypes[inflection.Plural(definedModel.Name)] = &pluralQueryType

			if _definedModel.Filter != nil && _definedModel.Filter.IsConnectionQuery {
				// filter connection
			}
			connectionQueryType := graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: strings.Title(inflection.Plural(definedModel.Name) + "_Connection"),
					Fields: graphql.Fields{
						"total": &graphql.Field{
							Type: graphql.Int,
						},
					},
				}),
				Args:    objects.BuildFilterArgument(localEnum, strings.Title(inflection.Plural(definedModel.Name)+"_Connection"), whereArgs[definedModel.Name], sortParam[definedModel.Name]),
				Resolve: g.gqlServer.CountResolverFn,
			}
			queryTypes[inflection.Plural(definedModel.Name)+"Connection"] = &connectionQueryType
		}

		// build mutation
		// delete mutation
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

		// update

		commonMutationFieldsConfigArgs[definedModel.Name]["payload"] = &graphql.ArgumentConfig{
			Type: graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   definedModel.Name + "_update_payload",
				Fields: updateMutationFieldsArguments[definedModel.Name],
			}),
		}

		updateArgs := make(graphql.FieldConfigArgument)
		updateArgs["_id"] = &graphql.ArgumentConfig{
			Type: graphql.NewNonNull(graphql.String),
		}
		// copy the common with it
		for k, v := range commonMutationFieldsConfigArgs[definedModel.Name] {
			updateArgs[k] = v
		}

		arg = updateArgs

		UpdateMutationType := graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name:   "Update_" + strings.Title(definedModel.Name),
				Fields: commonFields[definedModel.Name],
			}),
			Args:    arg,
			Resolve: g.gqlServer.MutationResolverFn,
		}
		mutationTypes["update"+strings.Title(definedModel.Name)] = &UpdateMutationType

		// create
		commonMutationFieldsConfigArgs[definedModel.Name]["payload"] = &graphql.ArgumentConfig{
			Type: graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   definedModel.Name + "_create_payload",
				Fields: createMutationFieldsArguments[definedModel.Name],
			})),
		}

		delete(commonMutationFieldsConfigArgs[definedModel.Name], "disconnect") // no disconnect field necessary during create operationType
		arg = commonMutationFieldsConfigArgs[definedModel.Name]

		CreateMutationType := graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name:   "Create_" + strings.Title(definedModel.Name),
				Fields: commonFields[definedModel.Name],
			}),
			Args:    arg,
			Resolve: g.gqlServer.MutationResolverFn,
		}

		mutationTypes["create"+strings.Title(definedModel.Name)] = &CreateMutationType

	}

	// build function query
	if hasFunctionInQuery {
		// logic for custom resolver / functions
		for _, s := range project.Schema.Functions {
			// if permitted in the role then add to query
			if s.Request != nil && s.Response != nil {
				var arg graphql.FieldConfigArgument
				switch s.Request.Model {
				case "CUSTOM":
					/*inputs := make(graphql.InputObjectConfigFieldMap)
					for _, f := range definedModel.Request.Params {
						inputs[f.Identifier] = utility.GetGraphQLArgumentObjectField(definedModel.Name, f, false)
					}*/
					return nil, errors.New("not Supported Yet")
				case "JSON":
					arg = graphql.FieldConfigArgument{
						"payload": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(scaler.ScalarMap),
						},
					}
					break
				default:
					arg = graphql.FieldConfigArgument{
						"payload": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
								Name:   s.Name + "_input_payload",
								Fields: updateMutationFieldsArguments[s.Request.Model],
							})),
						},
					}
					break
				}

				var _type *graphql.Field
				switch s.Response.Model {
				case "CUSTOM":
					return nil, errors.New("Not Supported Yet")
				case "JSON":
					_type = &graphql.Field{
						Type: scaler.ScalarMap,
					}
					break
				default:
					_type = &graphql.Field{
						Type: graphql.NewObject(graphql.ObjectConfig{
							Name:   s.Name,
							Fields: commonFields[inflection.Singular(s.Response.Model)],
						}),
					}
					break
				}

				mutationTypes[s.Name] = &graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "Custom_Resolver_" + s.Name,
						Fields: graphql.Fields{
							s.Response.Model: _type,
						},
					}),
					Args:    arg,
					Resolve: g.gqlServer.ApitoFunctionResolverFn,
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

	return &shared.ApplicationCache{
		Project: cache.Project,
		RawSchemas: &shared.RawSchema{
			Queries:   queryTypes,
			Mutations: mutationTypes,
		},
		Dataloaders: allLoaders,
	}, nil
}
