package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// publicSchemaBuildState holds shared inputs and intermediate maps for publicSchemaBuilder passes.
type publicSchemaBuildState struct {
	g *GraphCtrl

	ctx   context.Context
	span  trace.Span
	cache *models.ApplicationCache

	project *models.Project

	schemaRole *models.Role

	permissions       map[string]*models.APIPermission
	filteredModels    []*models.PublicSchemaModelFilter
	filteredFunctions []*models.ApitoFunction
	operationType     string

	preConnKey string
	skipPre    bool

	localEnum  *graphql.Enum
	metaObject *graphql.Field

	allLoaders map[string]*dataloader.Loader

	commonFields                   map[string]graphql.Fields
	aggregateFields                map[string]graphql.Fields
	commonMutationFieldsConfigArgs map[string]graphql.FieldConfigArgument
	connectionParamArgs            map[string]*graphql.InputObject
	whereArgs                      map[string]graphql.InputObjectConfigFieldMap
	whereRelationArgs              map[string]*graphql.InputObject
	sortParam                      map[string]graphql.InputObjectConfigFieldMap
	createMutationFieldsArguments  map[string]graphql.InputObjectConfigFieldMap
	updateMutationFieldsArguments  map[string]graphql.InputObjectConfigFieldMap
	connectionFields               map[string]graphql.InputObjectConfigFieldMap

	queryTypes    graphql.Fields
	mutationTypes graphql.Fields
}

// expandModelsForPreConnectionMaps returns filteredModels plus any schema models reachable as
// direct connection targets (transitive BFS). Pre-connection maps (commonFields, whereArgs, …)
// must exist for those targets or relation fields on a single filtered model get empty relFields.
func expandModelsForPreConnectionMaps(project *models.Project, filteredModels []*models.PublicSchemaModelFilter) []*models.PublicSchemaModelFilter {
	if project == nil || project.Schema == nil || len(filteredModels) == 0 {
		return filteredModels
	}
	byName := make(map[string]*models.ModelType)
	for _, m := range project.Schema.Models {
		if m != nil && m.Name != "" {
			byName[m.Name] = m
		}
	}
	seen := make(map[string]bool)
	queue := make([]string, 0)
	for _, mf := range filteredModels {
		if mf == nil || mf.Model == nil {
			continue
		}
		n := mf.Model.Name
		if !seen[n] {
			seen[n] = true
			queue = append(queue, n)
		}
	}
	for i := 0; i < len(queue); i++ {
		name := queue[i]
		m := byName[name]
		if m == nil {
			continue
		}
		for _, c := range m.Connections {
			if c == nil || c.Model == "" {
				continue
			}
			t := c.Model
			if byName[t] == nil {
				continue
			}
			if !seen[t] {
				seen[t] = true
				queue = append(queue, t)
			}
		}
	}
	orig := make(map[string]*models.PublicSchemaModelFilter)
	for _, mf := range filteredModels {
		if mf != nil && mf.Model != nil {
			orig[mf.Model.Name] = mf
		}
	}
	var extras []string
	for n := range seen {
		if _, ok := orig[n]; !ok {
			extras = append(extras, n)
		}
	}
	sort.Strings(extras)
	out := make([]*models.PublicSchemaModelFilter, 0, len(seen))
	for _, mf := range filteredModels {
		if mf != nil && mf.Model != nil && seen[mf.Model.Name] {
			out = append(out, mf)
		}
	}
	for _, n := range extras {
		out = append(out, &models.PublicSchemaModelFilter{Model: byName[n], Filter: nil})
	}
	return out
}

func (st *publicSchemaBuildState) loadOrBuildPreConnectionMaps() error {
	g := st.g
	filteredModels := expandModelsForPreConnectionMaps(st.project, st.filteredModels)
	permissions := st.permissions

	if g.cfg != nil && g.cfg.EnableCompiledSchemaCache {
		if hit, ok := getPreConnectionFromCache(st.preConnKey); ok {
			pre := clonePreConnectionShape(hit)
			st.commonFields = pre.commonFields
			st.aggregateFields = pre.aggregateFields
			st.commonMutationFieldsConfigArgs = pre.commonMutationFieldsConfigArgs
			st.connectionParamArgs = pre.connectionParamArgs
			st.whereArgs = pre.whereArgs
			st.whereRelationArgs = pre.whereRelationArgs
			st.sortParam = pre.sortParam
			st.createMutationFieldsArguments = pre.createMutationFieldsArguments
			st.updateMutationFieldsArguments = pre.updateMutationFieldsArguments
			st.connectionFields = pre.connectionFields
			st.skipPre = true
			if st.span != nil {
				st.span.SetAttributes(attribute.Bool("schema_cache_hit", true))
			}
		}
	}
	if !st.skipPre {
		st.commonFields = make(map[string]graphql.Fields)
		st.aggregateFields = make(map[string]graphql.Fields)
		st.commonMutationFieldsConfigArgs = make(map[string]graphql.FieldConfigArgument)
		st.connectionParamArgs = make(map[string]*graphql.InputObject)
		st.whereArgs = make(map[string]graphql.InputObjectConfigFieldMap)
		st.whereRelationArgs = make(map[string]*graphql.InputObject)
		st.sortParam = make(map[string]graphql.InputObjectConfigFieldMap)
		st.createMutationFieldsArguments = make(map[string]graphql.InputObjectConfigFieldMap)
		st.updateMutationFieldsArguments = make(map[string]graphql.InputObjectConfigFieldMap)
		st.connectionFields = make(map[string]graphql.InputObjectConfigFieldMap)

		for _, _definedModel := range filteredModels {

			definedModel := _definedModel.Model

			permission, ok := permissions[definedModel.Name]
			if !ok {
				var err error
				permission, err = utility.BuildCRUDPermissions(definedModel.Name, st.schemaRole)
				if err != nil {
					continue
				}
				ok = true
			}
			if (ok && permission != nil) || st.schemaRole.IsAdmin {

				queryBuilderInformation := resolver.QueryBuilderInformation{
					DataObjects:       make(graphql.Fields),
					AggregateObjects:  make(graphql.Fields),
					WhereParamObjects: make(graphql.InputObjectConfigFieldMap),
					SortParamObjects:  make(graphql.InputObjectConfigFieldMap),
				}

				for _, f := range definedModel.Fields {

					if !strings.HasPrefix(f.Identifier, "system_") {
						queryBuilderInformation.DataObjects[f.Identifier] = g.GetGraphQLField(definedModel.Name+"_query", f, false)
						if agv := g.GetGraphQLAggregateField(definedModel.Name+"_aggregate", f, false); agv != nil {
							queryBuilderInformation.AggregateObjects[f.Identifier] = agv
						}
					}

					if f.FieldType == "media" {
						continue
					}

					whereSortKey := f.Identifier
					if strings.HasPrefix(f.Identifier, "system_") {
						whereSortKey = utility.CanonicalSystemRelationFieldIdentifier(f.Identifier)
					}
					queryBuilderInformation.WhereParamObjects[whereSortKey] = &graphql.InputObjectFieldConfig{
						Type: objects.BuildWhereConditionArgument(definedModel.Name, f.Identifier, f),
					}

					if f.FieldType == "boolean" || f.FieldType == "list" {
						continue
					}
					queryBuilderInformation.SortParamObjects[whereSortKey] = &graphql.InputObjectFieldConfig{
						Type: objects.BuildSortConditionArgument(definedModel.Name+"_"+f.Identifier, f),
					}
				}

				if len(definedModel.Connections) > 0 {
					st.connectionParamArgs[definedModel.Name] = objects.BuildConnectionArguments(definedModel.Name, definedModel.Connections)
				}
				st.whereArgs[definedModel.Name] = queryBuilderInformation.WhereParamObjects
				st.sortParam[definedModel.Name] = queryBuilderInformation.SortParamObjects

				responseFields := graphql.Fields{
					"id": &graphql.Field{
						Type: graphql.String,
					},
					"meta": st.metaObject,
					"data": &graphql.Field{
						Type: graphql.NewObject(graphql.ObjectConfig{
							Name:   utility.GraphQLComposedTypeName(definedModel.Name, "RawModel"),
							Fields: queryBuilderInformation.DataObjects,
						}),
						Description: "Field Details",
					},
					"relation_doc_id": &graphql.Field{
						Type: graphql.String,
					},
				}

				st.commonFields[definedModel.Name] = responseFields
				if len(queryBuilderInformation.AggregateObjects) > 0 {
					st.aggregateFields[definedModel.Name] = queryBuilderInformation.AggregateObjects
				}

				cdata := graphql.InputObjectConfigFieldMap{}
				udata := graphql.InputObjectConfigFieldMap{}
				for _, f := range definedModel.Fields {

					if definedModel.Name == "user" && f.Identifier == "role" {
						continue
					}

					cdata[f.Identifier] = g.GetGraphQLArgumentObjectField(definedModel.Name, f, false)
					udata[f.Identifier] = g.GetGraphQLArgumentObjectField("update_"+definedModel.Name, f, true)
				}

				st.createMutationFieldsArguments[definedModel.Name] = cdata
				st.updateMutationFieldsArguments[definedModel.Name] = udata

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
						case "has_many":
							idFieldName = modelName + "_ids"
							idField = graphql.NewList(graphql.String)
						}

						if connPermission, ok := permissions[conn.Model]; ok && connPermission != nil {
							if connPermission.Create == "own" {
								idField = graphql.NewNonNull(idField)
							}
						}

						mutationFieldsConnectionArgument[idFieldName] = &graphql.InputObjectFieldConfig{
							Type: idField,
						}

					}

					st.connectionFields[definedModel.Name] = mutationFieldsConnectionArgument

					st.commonMutationFieldsConfigArgs[definedModel.Name] = graphql.FieldConfigArgument{
						"connect": &graphql.ArgumentConfig{
							Type: graphql.NewInputObject(graphql.InputObjectConfig{
								Name:   utility.GraphQLComposedTypeName(definedModel.Name, "Relation_Connect_Payload"),
								Fields: mutationFieldsConnectionArgument,
							}),
						},
						"disconnect": &graphql.ArgumentConfig{
							Type: graphql.NewInputObject(graphql.InputObjectConfig{
								Name:   utility.GraphQLComposedTypeName(definedModel.Name, "Relation_Disconnect_Payload"),
								Fields: mutationFieldsConnectionArgument,
							}),
						},
						"local": &graphql.ArgumentConfig{
							Type: st.localEnum,
						},
						"status": &graphql.ArgumentConfig{
							Type: enums.PublishStatusEnums,
						},
					}
				} else {
					st.commonMutationFieldsConfigArgs[definedModel.Name] = graphql.FieldConfigArgument{
						"local": &graphql.ArgumentConfig{
							Type: st.localEnum,
						},
						"status": &graphql.ArgumentConfig{
							Type: enums.PublishStatusEnums,
						},
					}
				}
			}
		}

		st.buildWhereRelationArgs()

		if g.cfg != nil && g.cfg.EnableCompiledSchemaCache {
			putPreConnectionInCache(st.preConnKey, &preConnectionShape{
				commonFields:                   st.commonFields,
				aggregateFields:                st.aggregateFields,
				whereArgs:                      st.whereArgs,
				sortParam:                      st.sortParam,
				connectionParamArgs:            st.connectionParamArgs,
				whereRelationArgs:              st.whereRelationArgs,
				createMutationFieldsArguments:  st.createMutationFieldsArguments,
				updateMutationFieldsArguments:  st.updateMutationFieldsArguments,
				connectionFields:               st.connectionFields,
				commonMutationFieldsConfigArgs: st.commonMutationFieldsConfigArgs,
			})
		}
	}
	return nil
}

func (st *publicSchemaBuildState) buildWhereRelationArgs() {
	for _, _definedModel := range st.filteredModels {
		definedModel := _definedModel.Model
		st.whereRelationArgs[definedModel.Name] = objects.BuildWhereRelationConditionArgument(definedModel.Name, definedModel.Connections, st.whereArgs)
	}
}

// resolveRelatedModelStoredKey returns the key used in commonFields / connectionParamArgs / whereArgs
// maps for a connection target. Edge connection.Model may use legacy camelCase while maps use
// canonical snake_case (or vice versa); mismatches yield nil maps and graphql.NewObject fails with
// "fields must be an object..." when the inner type has zero fields.
func (st *publicSchemaBuildState) resolveRelatedModelStoredKey(targetModelID string) string {
	if targetModelID == "" {
		return targetModelID
	}
	if f := st.commonFields[targetModelID]; len(f) > 0 {
		return targetModelID
	}
	targetCamel := utility.CamelFromAny(targetModelID)
	for storedName, f := range st.commonFields {
		if len(f) == 0 {
			continue
		}
		if utility.CamelFromAny(storedName) == targetCamel {
			return storedName
		}
	}
	return targetModelID
}

func (st *publicSchemaBuildState) attachConnectionFields() {
	g := st.g
	project := st.project
	cache := st.cache
	filteredModels := st.filteredModels
	permissions := st.permissions
	ctx := st.ctx

	for _, _definedModel := range filteredModels {

		definedModel := _definedModel.Model

		fields := st.commonFields[definedModel.Name]
		for _, connection := range definedModel.Connections {

			relKey := st.resolveRelatedModelStoredKey(connection.Model)
			relFields := st.commonFields[relKey]

			// Admins see all connection fields. Staff roles may grant known_as keys (chef) while
			// connection.Model is the base model (employee). When parent has read access, expose
			// known_as relation fields so queries on the parent type can traverse those edges.
			permission, ok := resolveConnectionPermission(permissions, connection, st.schemaRole)
			parentCanRead := modelReadAllowed(permissions, definedModel.Name, st.schemaRole)
			knownAsParentTraverse := connection.KnownAs != "" && parentCanRead
			allowConn := st.schemaRole.IsAdmin || knownAsParentTraverse || (ok && permission != nil)
			skipReadNone := ok && permission != nil && permission.Read == "none" && !st.schemaRole.IsAdmin && !knownAsParentTraverse
			if allowConn {
				if skipReadNone {
					continue
				}
				if len(relFields) == 0 {
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

					st.allLoaders[modelName] = dataloader.NewBatchedLoader(g.gqlServer.DataLoaderFn)

					// Public GraphQL field name: canonical snake model id (or known_as alias).
					// Root queries stay lowerCamel (productList); nested relation nodes match model ids.
					graphQLFieldName := utility.RelationFilterGraphQLKey(connection.Model, connection.KnownAs)
					if graphQLFieldName != modelName {
						st.allLoaders[graphQLFieldName] = st.allLoaders[modelName]
					}

					connRef := connection
					_field := &graphql.Field{
						Name: graphQLFieldName,
						Type: graphql.NewObject(graphql.ObjectConfig{
							Name:        definedModel.Name + "_has_one_" + modelName + "_connection",
							Fields:      relFields,
							Description: "has_one",
						}),
						Description: fmt.Sprintf("Has one %v", modelName),
					}

					_field.Resolve = func(p graphql.ResolveParams) (interface{}, error) {
						source, ok := p.Source.(*types.DefaultDocumentStructure)
						if !ok || source == nil {
							return func() (interface{}, error) {
								return nil, nil
							}, nil
						}

						// Loaders are registered as st.allLoaders[modelName] (KnownAs or connection.Model).
						// p.Info.Path.Key is often the parent field (e.g. "food"), not the target model key — using it
						// as the loader map key misses the batch loader and returns nil without hitting the driver.
						lidPreferred := modelName
						if pk, ok := p.Info.Path.Key.(string); ok && pk != "" {
							lidPreferred = pk
						}

						appCache, ok := utility.LegacyApplicationCache(p.Context)
						if !ok {
							return func() (interface{}, error) {
								return nil, errors.New("application cache missing in context")
							}, nil
						}
						loaders := appCache.Dataloaders
						rootSelectionSet, ok := utility.LegacySelectionSet(p.Context)
						if !ok {
							return func() (interface{}, error) {
								return nil, errors.New("selection set missing in context")
							}, nil
						}
						key := models.NewResolverKey(source.ID, nil)
						knownAs := _definedModel.KnownAs

						// Match has_many: find the root field for this document (source.Type), then the nested field.
						var selectionSet *ast.SelectionSet
						for _, sel := range rootSelectionSet {
							if val := sel.(*ast.Field); utility.SingularResourceName(val.Name) == source.Type {
								for _, inner := range val.SelectionSet {
									if _s := inner.(*ast.Field); utility.SingularResourceName(_s.Name) == utility.SingularResourceName(graphQLFieldName) {
										selectionSet = &_s.SelectionSet
										break
									}
								}
								break
							}
						}

						typeRelation := utility.WithRelationMeta(ctx, map[string]interface{}{
							"project_id":    project.ID,
							"relation_type": "has_one",
							"resolveParam":  &p,
							"connection":    connRef,
							"selectionSet":  selectionSet,
							"knownAs":       knownAs,
							"parentModel":   definedModel.Name,
						})
						tx, closeContext := onecontext.Merge(p.Context, typeRelation)
						defer closeContext()
						var batch *dataloader.Loader
						if v, ok := loaders[lidPreferred]; ok {
							batch = v
						} else if v, ok := loaders[modelName]; ok {
							batch = v
						}
						if batch != nil {
							thunk := batch.Load(tx, key)
							return func() (interface{}, error) {
								return thunk()
							}, nil
						}

						return func() (interface{}, error) {
							return nil, nil
						}, nil
					}

					fields[graphQLFieldName] = _field
				case "has_many":

					var modelName string
					if connection.KnownAs != "" {
						modelName = connection.KnownAs
					} else {
						modelName = connection.Model
					}

					hmName := utility.RelationNestedListGraphQLKey(connection.Model, connection.KnownAs)
					// Loaders stay keyed by MultipleResourceName for dataloader identity; field name is snake `_list`.
					loaderKey := utility.MultipleResourceName(modelName)
					st.allLoaders[loaderKey] = dataloader.NewBatchedLoader(g.gqlServer.DataLoaderFn)
					st.allLoaders[hmName] = st.allLoaders[loaderKey]

					fields[hmName] = &graphql.Field{
						Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
							Name:        definedModel.Name + "_has_many_" + utility.MultipleResourceName(definedModel.Name+"_"+modelName) + "_connections",
							Fields:      relFields,
							Description: "has_many",
						})),
						Args:        objects.BuildFilterArgument(st.localEnum, utility.MultipleResourceName(definedModel.Name+"_"+modelName), st.connectionParamArgs[relKey], st.whereArgs[relKey], st.whereRelationArgs[relKey], st.sortParam[relKey]),
						Description: fmt.Sprintf("Has many %v", connection.Model),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							source, ok := p.Source.(*types.DefaultDocumentStructure)
							if !ok || source == nil {
								return func() (interface{}, error) {
									return nil, nil
								}, nil
							}
							appCache, ok := utility.LegacyApplicationCache(p.Context)
							if !ok {
								return func() (interface{}, error) {
									return nil, errors.New("application cache missing in context")
								}, nil
							}
							loaders := appCache.Dataloaders
							rootSelectionSet, ok := utility.LegacySelectionSet(p.Context)
							if !ok {
								return func() (interface{}, error) {
									return nil, errors.New("selection set missing in context")
								}, nil
							}
							key := models.NewResolverKey(source.ID, nil)
							fieldName := p.Info.FieldName
							lid := utility.MultipleResourceName(fieldName)
							knownAs := _definedModel.KnownAs

							var selectionSet *ast.SelectionSet
							for _, sel := range rootSelectionSet {
								if val := sel.(*ast.Field); utility.SingularResourceName(val.Name) == source.Type {
									for _, inner := range val.SelectionSet {
										if _s := inner.(*ast.Field); _s.Name == fieldName || _s.Name == lid || _s.Name == hmName {
											selectionSet = &_s.SelectionSet
											break
										}
									}
									break
								}
							}

							typeRelation := utility.WithRelationMeta(ctx, map[string]interface{}{
								"project_id":    project.ID,
								"relation_type": "has_many",
								"resolveParam":  &p,
								"connection":    connection,
								"selectionSet":  selectionSet,
								"knownAs":       knownAs,
								"parentModel":   definedModel.Name,
							})
							tx, closeContext := onecontext.Merge(p.Context, typeRelation)
							defer closeContext()
							if ld, ok := loaders[fieldName]; ok {
								thunk := ld.Load(tx, key)
								return func() (interface{}, error) {
									return thunk()
								}, nil
							}
							if ld, ok := loaders[lid]; ok {
								thunk := ld.Load(tx, key)
								return func() (interface{}, error) {
									return thunk()
								}, nil
							}
							if ld, ok := loaders[loaderKey]; ok {
								thunk := ld.Load(tx, key)
								return func() (interface{}, error) {
									return thunk()
								}, nil
							}
							return func() (interface{}, error) {
								return nil, nil
							}, nil
						},
					}
				}
			}
		}
		st.commonFields[definedModel.Name] = fields
	}
	_ = cache
}

func (st *publicSchemaBuildState) buildQueryAndMutationTypes() {
	g := st.g
	filteredModels := st.filteredModels
	permissions := st.permissions
	cache := st.cache
	operationType := st.operationType

	st.queryTypes = graphql.Fields{}
	st.mutationTypes = graphql.Fields{}

	for _, _definedModel := range filteredModels {

		definedModel := _definedModel.Model

		if permission, ok := permissions[definedModel.Name]; (ok && permission != nil) || st.schemaRole.IsAdmin {

			if permission.Read == "none" {
				continue
			}

			arg := graphql.FieldConfigArgument{
				"local": &graphql.ArgumentConfig{
					Type: st.localEnum,
				}}

			if definedModel.SinglePage {
				arg["_id"] = &graphql.ArgumentConfig{
					Type: graphql.String,
				}
			} else {
				arg["_id"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				}
			}

			singleQueryType := graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name:   utility.PascalFromAnyModelID(definedModel.Name),
					Fields: st.commonFields[definedModel.Name],
				}),
				Args:    arg,
				Resolve: g.gqlServer.SingleResourceResolverFn,
			}

			// Root single-document field: camel(stored id), same as plan / list derivations (not raw snake_case).
			st.queryTypes[utility.SingularResourceName(definedModel.Name)] = &singleQueryType

			if !definedModel.SinglePage {
				pluralQueryType := graphql.Field{
					Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
						Name:   utility.ListGraphQLTypeName(definedModel.Name),
						Fields: st.commonFields[definedModel.Name],
					})),
					Args:    objects.BuildFilterArgument(st.localEnum, utility.GraphQLTypeNameForFilterArg(definedModel.Name), st.connectionParamArgs[definedModel.Name], st.whereArgs[definedModel.Name], st.whereRelationArgs[definedModel.Name], st.sortParam[definedModel.Name]),
					Resolve: g.gqlServer.MultiResourceResolverFn,
				}
				st.queryTypes[utility.MultipleResourceName(definedModel.Name)] = &pluralQueryType

				countFields := graphql.Fields{
					"total": &graphql.Field{
						Type:    graphql.Int,
						Resolve: g.gqlServer.CountResolverFn,
					},
				}
				if len(st.aggregateFields[definedModel.Name]) > 0 {
					aggBase := st.aggregateFields[definedModel.Name]
					countFields["aggregate"] = &graphql.Field{
						Type: graphql.NewObject(graphql.ObjectConfig{
							Name:   utility.GraphQLComposedTypeName(definedModel.Name, "List_Aggregate"),
							Fields: aggBase,
						}),
						Resolve: g.gqlServer.AggregateResolverFn,
					}

					groupByFields := cloneGraphQLFields(aggBase)
					groupByFields["group_by_identifier_1"] = &graphql.Field{
						Type: graphql.String,
					}
					groupByFields["group_by_identifier_2"] = &graphql.Field{
						Type: graphql.String,
					}

					countFields["groupBy"] = &graphql.Field{
						Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
							Name:   utility.GraphQLComposedTypeName(definedModel.Name, "List_Aggregate_GroupBy"),
							Fields: groupByFields,
						})),
						Resolve: g.gqlServer.AggregateResolverFn,
					}
				}
				connectionQueryType := graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name:   utility.GraphQLComposedTypeName(definedModel.Name, "List_Connection"),
						Fields: countFields,
					}),
					Args: objects.BuildFilterArgument(st.localEnum, utility.GraphQLComposedTypeName(definedModel.Name, "List_Count"), st.connectionParamArgs[definedModel.Name], st.whereArgs[definedModel.Name], st.whereRelationArgs[definedModel.Name], st.sortParam[definedModel.Name]),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p, nil
					},
				}
				st.queryTypes[utility.MultipleResourceName(definedModel.Name)+"Count"] = &connectionQueryType
			}
		}

		if permission, ok := permissions[definedModel.Name]; (operationType == "" || operationType == "mutation") && ((ok && permission != nil) || st.schemaRole.IsAdmin) {

			if permission.Delete != "none" {
				deleteArgs := make(graphql.FieldConfigArgument)
				deleteArgs["_ids"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
				}

				DeleteMutationType := graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "Delete_" + utility.PascalFromAnyModelID(definedModel.Name),
						Fields: graphql.Fields{
							"response": &graphql.Field{
								Type: graphql.String,
							},
						},
					}),
					Args:    deleteArgs,
					Resolve: g.gqlServer.MutationResolverFn,
				}
				st.mutationTypes["delete"+utility.PascalFromAnyModelID(definedModel.Name)] = &DeleteMutationType
			}

			if permission.Update != "none" {

				st.commonMutationFieldsConfigArgs[definedModel.Name]["payload"] = &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   utility.GraphQLComposedTypeName(definedModel.Name, "Update_Payload"),
						Fields: st.updateMutationFieldsArguments[definedModel.Name],
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
				for k, v := range st.commonMutationFieldsConfigArgs[definedModel.Name] {
					updateArgs[k] = v
				}

				arg := updateArgs

				UpdateMutationType := graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name:   "Update_" + utility.PascalFromAnyModelID(definedModel.Name),
						Fields: st.commonFields[definedModel.Name],
					}),
					Args:    arg,
					Resolve: g.gqlServer.MutationResolverFn,
				}
				st.mutationTypes["update"+utility.PascalFromAnyModelID(definedModel.Name)] = &UpdateMutationType
			}

			if permission.Create != "none" && definedModel.Name != "user" {

				arg := make(graphql.FieldConfigArgument)
				for k, v := range st.commonMutationFieldsConfigArgs[definedModel.Name] {
					arg[k] = v
				}
				arg["payload"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   utility.GraphQLComposedTypeName(definedModel.Name, "Create_Payload"),
						Fields: st.createMutationFieldsArguments[definedModel.Name],
					})),
				}
				delete(arg, "disconnect")

				singleCreateMutationType := graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name:   "Create_" + utility.PascalFromAnyModelID(definedModel.Name),
						Fields: st.commonFields[definedModel.Name],
					}),
					Args:    arg,
					Resolve: g.gqlServer.MutationResolverFn,
				}

				pluralArgs := make(graphql.FieldConfigArgument)
				for k, v := range st.commonMutationFieldsConfigArgs[definedModel.Name] {
					pluralArgs[k] = v
				}

				upsertPayloadFields := st.createMutationFieldsArguments[definedModel.Name]
				upsertPayloadFields["_id"] = &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				}

				if connectionFieldsForModel, hasConnections := st.connectionFields[definedModel.Name]; hasConnections {
					upsertPayloadFields["_connect"] = &graphql.InputObjectFieldConfig{
						Type: graphql.NewInputObject(graphql.InputObjectConfig{
							Name:   utility.GraphQLComposedTypeName(definedModel.Name, "List_Connect"),
							Fields: connectionFieldsForModel,
						}),
					}
				}

				pluralArgs["payloads"] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
						Name:   utility.GraphQLComposedTypeName(definedModel.Name, "List_Upsert_Payload"),
						Fields: upsertPayloadFields,
					})))),
				}
				delete(pluralArgs, "payload")

				pluralCreateMutationType := graphql.Field{
					Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
						Name:   "Upsert_" + utility.ListGraphQLTypeName(definedModel.Name),
						Fields: st.commonFields[definedModel.Name],
					})),
					Args:    pluralArgs,
					Resolve: g.gqlServer.MutationResolverFn,
				}

				st.mutationTypes["create"+utility.PascalFromAnyModelID(definedModel.Name)] = &singleCreateMutationType
				st.mutationTypes["upsert"+utility.ListGraphQLTypeName(definedModel.Name)] = &pluralCreateMutationType
			}
		}

	}
	_ = cache
}

func (st *publicSchemaBuildState) mergeFunctionAndPluginFields() (*models.ApplicationCache, error) {
	g := st.g
	cache := st.cache
	filteredFunctions := st.filteredFunctions
	schemaRole := st.schemaRole

	queryTypes := st.queryTypes
	mutationTypes := st.mutationTypes

	if len(filteredFunctions) > 0 ||
		(cache.GraphqlRequest != nil && cache.GraphqlRequest.OperationName == "IntrospectionQuery") ||
		len(schemaRole.LogicExecutions) > 0 {
		for _, fn := range filteredFunctions {
			// Skip HashiCorp/provider-linked functions unless they are plugin schema fields (plg_*)
			// or Apito Functions platform runtimes (deno/wasm) which are first-class public fields.
			if fn.FunctionProviderID != "" && !strings.HasPrefix(fn.Name, "plg_") && !fn.IsApitoFunctionsRuntime() {
				continue
			}

			if utility.ArrayContains(schemaRole.LogicExecutions, fn.Name) || schemaRole.IsAdmin {
				if fn.Request != nil && fn.Response != nil {
					var _argType graphql.Input
					switch fn.Request.Model {
					case "CUSTOM":
						return nil, errors.New("not Supported Yet")
					case "JSON":
						_argType = scaler.ScalarJSONWithRequest(fn.Name, cache.GraphqlRequest)
					default:
						_argType = graphql.NewInputObject(graphql.InputObjectConfig{
							Name:   fn.Name + "_Input_Payload",
							Fields: st.updateMutationFieldsArguments[fn.Request.Model],
						})
					}

					var _arg graphql.FieldConfigArgument
					if fn.Request.OptionalPayload {
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
					switch fn.Response.Model {
					case "CUSTOM":
						return nil, errors.New("not Supported Yet")
					case "JSON":
						_type = graphql.NewObject(graphql.ObjectConfig{
							Name: fn.Name,
							Fields: graphql.Fields{
								"JSON": &graphql.Field{
									Type: scaler.ScalarJSON,
								},
							},
						})
					default:
						if fn.Response.IsArray {
							_type = graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
								Name:   fn.Name,
								Fields: st.commonFields[utility.SingularResourceName(fn.Response.Model)],
							}))
						} else {
							_type = graphql.NewObject(graphql.ObjectConfig{
								Name:   fn.Name,
								Fields: st.commonFields[utility.SingularResourceName(fn.Response.Model)],
							})
						}
					}
					switch fn.GraphQLSchemaType {
					case "Query":
						queryTypes[fn.Name] = &graphql.Field{
							Type:    _type,
							Args:    _arg,
							Resolve: g.gqlServer.ApitoFunctionResolverFn,
						}
					case "Mutation":
						mutationTypes[fn.Name] = &graphql.Field{
							Type:    _type,
							Args:    _arg,
							Resolve: g.gqlServer.ApitoFunctionResolverFn,
						}
					default:
						queryTypes[fn.Name] = &graphql.Field{
							Type:    _type,
							Args:    _arg,
							Resolve: g.gqlServer.ApitoFunctionResolverFn,
						}
					}
				}
			}
		}
	}

	if cache.RawSchemas != nil {
		for k, v := range cache.RawSchemas.Queries {
			if _, ok := queryTypes[k]; !ok && v != nil {
				queryTypes[k] = v
			} else {
				log.Printf("[apito] public schema: duplicate extension query name %q ignored", k)
			}
		}

		for k, v := range cache.RawSchemas.Mutations {
			mutationTypes[k] = v
		}
	}

	for name, field := range g.gqlServer.PublicAuthQueryFields() {
		if field == nil {
			continue
		}
		if _, exists := queryTypes[name]; exists {
			log.Printf("[apito] public schema: duplicate auth query name %q ignored", name)
			continue
		}
		queryTypes[name] = field
	}

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
		Dataloaders: st.allLoaders,
	}, nil
}
