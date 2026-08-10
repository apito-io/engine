package resolver

import (
	"github.com/apito-io/engine/resolver/dataloader"
	"github.com/apito-io/engine/scaler"
	"github.com/apito-io/engine/schemas/args"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/tailor-platform/graphql"
)

func (s *GraphQLServer) BuildServerQueriesAndMutations() {

	s.SystemDataloaders, _ = dataloader.GetSystemDataloader(s.SystemDriver)

	privateSchemaObjects := objects.GetSchemaObjects(s.SystemDriver, s.SystemDataloaders)
	privateSchemaObjects.ObjectModels = privateSchemaObjects.InitPrivateObjects()
	s.registerProjectSettingsGraphQLFields(privateSchemaObjects.ObjectModels)
	s.PrivateSchemObjects = privateSchemaObjects

	if s.Cfg.SchemaObjectsExtensionHook != nil {
		s.Cfg.SchemaObjectsExtensionHook(privateSchemaObjects)
	}

	s.SystemQueriesChan <- &graphql.Fields{
		"currentProject": &graphql.Field{
			Name:    "GetCurrentProject",
			Type:    privateSchemaObjects.ProjectDetailsObject,
			Resolve: s.GetCurrentProjectResolverFn,
		},
		"auditLogs": &graphql.Field{
			Name: "GetAuditLogs",
			Type: graphql.NewList(privateSchemaObjects.AuditLogObject),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "if id is missing gets the current project",
				},
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "AuditWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"user_id": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"project_id": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"internal_function": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"response_code": &graphql.InputObjectFieldConfig{
								Type: args.IntegerFilter,
							},
							"graphql_operation_name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
						},
					}),
				},
			},
			Resolve: s.ListAuditLogsFn,
		},
		"searchSchemaOperations": &graphql.Field{
			Name: "SearchSchemaOperations",
			Type: graphql.NewList(schemaOperationGraphQLObject()),
			Args: graphql.FieldConfigArgument{
				"statuses": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
				"limit": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: s.SearchSchemaOperationsResolverFn,
		},
		"listProjects": &graphql.Field{
			Name: "ListAllProjects",
			Args: graphql.FieldConfigArgument{
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "ProjectWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"id": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"created_at": &graphql.InputObjectFieldConfig{
								Type: args.DateFilter,
							},
							"updated_at": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
						},
					}),
				},
			},
			Type:    graphql.NewList(privateSchemaObjects.ProjectDetailsObject),
			Resolve: s.ListProjectsResolverFn,
		},
		"listPermissionsAndScopes": &graphql.Field{
			Name: "ListPermissionsAndScopes",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "ListPermissionsAndScopesResponse",
				Fields: graphql.Fields{
					"permissions": &graphql.Field{
						Type: graphql.NewList(graphql.String),
					},
					"models": &graphql.Field{
						Type: graphql.NewList(graphql.String),
					},
					"functions": &graphql.Field{
						Type: graphql.NewList(graphql.String),
					},
				},
			}),
			Resolve: s.ListRoleScopesResolverFn,
		},
		"getProjectPlans": &graphql.Field{
			Name: "GetProjectPlans",
			Type: graphql.NewList(privateSchemaObjects.PlanObject),
			Resolve: s.GetProjectPlansResolverFn,
		},
		"modelDocumentCounts": &graphql.Field{
			Name: "ModelDocumentCounts",
			Args: graphql.FieldConfigArgument{
				"models": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
			},
			Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name: "ModelDocumentCountItem",
				Fields: graphql.Fields{
					"model": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
					},
					"count": &graphql.Field{
						Type: graphql.NewNonNull(graphql.Int),
					},
				},
			})),
			Resolve: s.ModelDocumentCountsResolverFn,
		},
		"getUser": &graphql.Field{
			Name: "GetLoggedInUser",
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "if id is missing gets the current project",
				},
			},
			Type:    privateSchemaObjects.SystemUserObject,
			Resolve: s.GetLoggedInUserFn,
		},
		"getProject": &graphql.Field{
			Name: "GetAProject",
			Type: privateSchemaObjects.ProjectDetailsObject,
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "if id is missing gets the current project",
				},
			},
			Resolve: s.GetProjectResolverFn,
		},
		"listWebHooks": &graphql.Field{
			Name: "ListWebHooks",
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Type:    graphql.NewList(privateSchemaObjects.WebHookModelObject),
			Resolve: s.ListWebHooksResolverFn,
		},
		"projectModelsInfo": &graphql.Field{
			Name: "ListAndFilterProjectModelsInfo",
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Type:    graphql.NewList(privateSchemaObjects.ModelTypeObject),
			Resolve: s.ListModelsInfoResolverFn,
		},
		"projectSchemaRelationGraph": &graphql.Field{
			Name: "ProjectSchemaRelationGraph",
			Type: scaler.ScalarJSON,
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "optional; when set must match the active project id (same as projectModelsInfo scope)",
				},
			},
			Resolve: s.ProjectSchemaRelationGraphResolverFn,
		},
		/*"projectModelInfoByName": &graphql.Field{
			Name:    "ProjectModelInfoByName",
			Type:    privateSchemaObjects.ModelTypeObject,
			Args:    graphql.FieldConfigArgument{},
			Resolve: s.ProjectModelInfoResolverFn,
		},*/
		"listPluginIds": &graphql.Field{
			Name: "GetListPluginIds",
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(enums.PluginTypeEnums),
				},
			},
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "ListPluginIDsResponse",
				Fields: graphql.Fields{
					"type": &graphql.Field{
						Type: graphql.NewNonNull(enums.PluginTypeEnums),
					},
					"plugins": &graphql.Field{
						Type: graphql.NewList(graphql.String),
					},
				},
			}),
			Resolve: s.LoadedFunctionProviderResolverFn,
		},
		"getSystemPlugins": &graphql.Field{
			Name: "GetSystemAndThirdPartyPlugins",
			Type: graphql.NewList(privateSchemaObjects.PluginDetailsObject),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "SystemPluginsWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
						},
					}),
				},
			},
			Resolve: s.SystemPlugins,
		},
		"getProjectPlugins": &graphql.Field{
			Name: "GetProjectSpecificInstalledPlugins",
			Type: graphql.NewList(privateSchemaObjects.PluginDetailsObject),
			Args: graphql.FieldConfigArgument{
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "ProjectPluginsWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
						},
					}),
				},
			},
			Resolve: s.ProjectSpecificInstalledPlugins,
		},
		/*		"listModelData": &graphql.Field{
				Name: "ListAllDataOfAModel",
				Type: graphql.NewList(objects.DatModelObject),
				Args: graphql.FieldConfigArgument{
					"model": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"search": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"where": &graphql.ArgumentConfig{
						Type: scaler.ScalarJSON,
					},
					"page": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"limit": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: server.ListModelsDataInfoResolverFn,
			},*/
		"getSingleData": &graphql.Field{
			Name: "GetSingleDataType",
			Type: privateSchemaObjects.DocModelObject,
			Args: graphql.FieldConfigArgument{
				"model": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"single_page_data": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"local": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"revision": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
			},
			Resolve: s.ListSingleModelDataInfoResolverFn,
		},
		"getModelData": &graphql.Field{
			Name: "GetModelDataType",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "GetModelDataResponse",
				Fields: graphql.Fields{
					"results": &graphql.Field{
						Type:    graphql.NewList(privateSchemaObjects.DocModelObject),
						Resolve: s.ListDetailedModelsDataInfoResolverFn,
					},
					"count": &graphql.Field{
						Type:    graphql.Int,
						Resolve: s.ListDetailedModelsDataCountResolverFn,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"_key": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
				"model": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"search": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"page": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
				"limit": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
				"where": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
				"status": &graphql.ArgumentConfig{
					Type: enums.FilterStatusEnums,
				},
				"connection": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "ListAllDataDetailedOfAModelConnectionPayload",
						Fields: graphql.InputObjectConfigFieldMap{
							"_id": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"to_model": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"relation_type": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"known_as": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"model": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"connection_type": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
						},
					}),
				},
				"intersect": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
			},
			Resolve: s.ListDetailedModelsDataProxyResolverFn,
		},
		"searchSystemUsers": &graphql.Field{
			Name: "SearchSystemUsersOfApito",
			Type: graphql.NewList(privateSchemaObjects.SystemUserObject),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "SearchSystemUsersWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"first_name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"last_name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"username": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"email": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"organization_id": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
						},
					}),
				},
			},
			Resolve: s.SearchSystemUsersResolverFn,
		},
		"teamMembers": &graphql.Field{
			Name: "GetProjectTeamMembers",
			Args: graphql.FieldConfigArgument{
				"organization_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "TeamMembersWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"first_name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"last_name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"username": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"email": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
							"organization_id": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
						},
					}),
				},
			},
			Type:    graphql.NewList(privateSchemaObjects.SystemUserObject),
			Resolve: s.ListProjectTeams,
		},
		"teams": &graphql.Field{
			Name: "GetTeamMembersOfAProject",
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "TeamsWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
						},
					}),
				},
			},
			Type:    graphql.NewList(privateSchemaObjects.TeamObject),
			Resolve: s.GetTeamsResolverFn,
		},
		"organizations": &graphql.Field{
			Name: "GetUserOrganizations",
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "OrganizationsWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"name": &graphql.InputObjectFieldConfig{
								Type: args.CommonFilter,
							},
						},
					}),
				},
			},
			Type:    graphql.NewList(privateSchemaObjects.OrganizationObject),
			Resolve: s.GetOrganizationsResolverFn,
		},
		/*		"getPhotos": &graphql.Field{
					Name: "ListAllDataOfAMedia",
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "GetAllPhotosResponse",
						Fields: graphql.Fields{
							"results": &graphql.Field{
								Type:    graphql.NewList(objects.FileDetailsType),
								Resolve: s.GetPhotosInfoResolverFn,
							},
							"count": &graphql.Field{
								Type:    graphql.Int,
								Resolve: s.CountPhotosInfoResolverFn,
							},
						},
					}),
					Args: graphql.FieldConfigArgument{
						"models": &graphql.ArgumentConfig{
							Type: graphql.NewList(graphql.String),
						},
						"search": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
						"page": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
						"limit": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
						"ids_in": &graphql.ArgumentConfig{
							Type: graphql.NewList(graphql.String),
						},
					},
					Resolve: s.GetPhotosAndCountInfoResolverFn,
				},
		*/
		"listSingleModelRevisionData": &graphql.Field{
			Name: "ListAllRevisionDataOfADocument",
			Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name: "RevisionHistoryResponse",
				Fields: graphql.Fields{
					"id":          &graphql.Field{Type: graphql.String},
					"revision_at": &graphql.Field{Type: graphql.String},
					"status":      &graphql.Field{Type: graphql.String},
				},
			})),
			Args: graphql.FieldConfigArgument{
				"model": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"single_page_data": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.ListDocumentRevisionInfoResolverFn,
		},
		/*"listSingleModelRelationData": &graphql.Field{
			Name: "ListSingleModelHasManyData",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "ListSingleModelHasManyDataObject",
				Fields: graphql.Fields{
					"data": &graphql.Field{
						Type: scaler.ScalarJSON,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"from_model": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"to_model": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"relation_type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"known_as": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: server.ListSingleModelHasManyResolverFn,
		},*/
		"projectFunctionsInfo": &graphql.Field{
			Name: "ProjectFunctionsInfo",
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Type:    graphql.NewList(privateSchemaObjects.CloudFunctionObject),
			Resolve: s.ProjectFunctionsInfoResolverFn,
		},
		"listFunctionRevisions": &graphql.Field{
			Name: "ListFunctionRevisions",
			Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name: "FunctionRevisionType",
				Fields: graphql.Fields{
					"id":            &graphql.Field{Type: graphql.String},
					"project_id":    &graphql.Field{Type: graphql.String},
					"name":          &graphql.Field{Type: graphql.String},
					"revision":      &graphql.Field{Type: graphql.Float},
					"runtime":       &graphql.Field{Type: graphql.String},
					"language":      &graphql.Field{Type: graphql.String},
					"source":        &graphql.Field{Type: graphql.String},
					"artifact_key":  &graphql.Field{Type: graphql.String},
					"artifact_hash": &graphql.Field{Type: graphql.String},
					"abi_version":   &graphql.Field{Type: graphql.String},
					"capabilities":  &graphql.Field{Type: graphql.NewList(graphql.String)},
					"created_by":    &graphql.Field{Type: graphql.String},
					"created_at":    &graphql.Field{Type: graphql.String},
				},
			})),
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"limit": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: s.ListFunctionRevisionsResolverFn,
		},
		"listFunctionDeployments": &graphql.Field{
			Name: "ListFunctionDeployments",
			Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name: "FunctionDeploymentType",
				Fields: graphql.Fields{
					"id":          &graphql.Field{Type: graphql.String},
					"project_id":  &graphql.Field{Type: graphql.String},
					"name":        &graphql.Field{Type: graphql.String},
					"revision_id": &graphql.Field{Type: graphql.String},
					"build_id":    &graphql.Field{Type: graphql.String},
					"environment": &graphql.Field{Type: graphql.String},
					"status":      &graphql.Field{Type: graphql.String},
					"deployed_by": &graphql.Field{Type: graphql.String},
					"rollback_of": &graphql.Field{Type: graphql.String},
					"created_at":  &graphql.Field{Type: graphql.String},
				},
			})),
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"limit": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: s.ListFunctionDeploymentsResolverFn,
		},
		"listExecutableFunctions": &graphql.Field{
			Name: "ListExecutableFunctions",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "ListExecutableFunctionsResponse",
				Fields: graphql.Fields{
					"functions": &graphql.Field{Type: graphql.NewList(graphql.String)},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.ListExecutableFunctionsResolverFn,
		},
		/*// this function is deprecated
		"thirdPartyFunctionQuery": &graphql.Field{
			Name: "ThirdPartyFunctionQuery",
			Args: graphql.FieldConfigArgument{
				"function_provider": &graphql.ArgumentConfig{
					Type: enums.FuncProviderEnum,
				},
				"region": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Type:    graphql.NewList(objects.FunctionProviderConfigType),
			Resolve: server.AWSLambdaFunctionInfoResolverFn,
		},*/
		"listAvailableFunctions": &graphql.Field{
			Name: "GetListAvailableFunctions",
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"function_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "ListAvailableFunctionsResponse",
				Fields: graphql.Fields{
					"functions": &graphql.Field{
						Type: graphql.NewList(graphql.String),
					},
				},
			}),
			Resolve: s.ListAvailableFunctionsResolverFn,
		},
		"connectSupport": &graphql.Field{
			Name: "ConnectSupport",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "ConnectSupportResponse",
				Fields: graphql.Fields{
					"id_token": &graphql.Field{Type: graphql.String},
				},
			}),
			Resolve: s.ConnectSupportResolverFn,
		},
		/*		"switchProjects": &graphql.Field{
				Name: "SwitchProjects",
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "SwitchProjectsResponse",
					Fields: graphql.Fields{
						"token": &graphql.Field{
							Type: graphql.String,
						},
						"role": &graphql.Field{
							Type: graphql.String,
						},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: server.SwitchProjectResolverFn,
			},*/
	}
	s.SystemMutationsChan <- &graphql.Fields{
		// This the mutation that sends messages to the server.
		"send": &graphql.Field{
			Args: graphql.FieldConfigArgument{
				"message": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "SendMutationResponse",
				Fields: graphql.Fields{
					"message": &graphql.Field{Type: graphql.String},
				},
			}),
			Resolve: s.SendEvent,
		},
		// plugin query
		/*"pluginBuildTrigger": &graphql.Field{
			Name: "PluginBuildTriggerResponse",
			Type: objects.PluginDetailsType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(enums.PluginSystemType),
				},
				"current_status": &graphql.ArgumentConfig{
					Type: enums.PluginLoadTypeEnums,
				},
				"build_command": &graphql.ArgumentConfig{
					Type: enums.PluginLoadTypeEnums,
				},
				"repository_url": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"branch": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.PluginBuildTriggerResolverFn,
		},*/
		"upsertPlugin": &graphql.Field{
			Name: "UpsetPluginResponse",
			Type: privateSchemaObjects.PluginDetailsObject,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				}, /*
					"type": &graphql.ArgumentConfig{
						Type: enums.PluginTypeEnums,
					},
					"title": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"icon": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"version": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"description": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"role": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"exported_variable": &graphql.ArgumentConfig{
						Type: graphql.String,
					}, */
				"env_vars": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "PluginConfigEnvVarsPayload",
						Fields: graphql.InputObjectConfigFieldMap{
							"key": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"value": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
						},
					})),
				},
				"enable": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"activate_status": &graphql.ArgumentConfig{
					Type: enums.PluginActivationType,
				},
			},
			Resolve: s.UpsertPluginResolverFn,
		},
		"removeProjectSpecificPlugin": &graphql.Field{
			Name: "RemoveProjectSpecificPlugin",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "RemoveProjectSpecificPluginResponse",
				Fields: graphql.Fields{
					"message": &graphql.Field{Type: graphql.String},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.RemoveProjectSpecificPluginResolverFn,
		},
		// Delete Media File
		/*"deleteMediaFile": &graphql.Field{
			Name: "DeleteMediaFile",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "DeleteMediaFileResponse",
				Fields: graphql.Fields{
					"message": &graphql.Field{Type: graphql.String},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"ids": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
				},
			},
			Resolve: s.DeleteMediaFileInfoResolverFn,
		},*/
		"generateProjectToken": &graphql.Field{
			Name: "GenerateProjectToken",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "GenerateProjectTokenResponse",
				Fields: graphql.Fields{
					"token": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"role": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"duration": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.GenerateProjectTokenResolverFn,
		},
		"deleteProjectToken": &graphql.Field{
			Name: "DeleteApiToken",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "DeleteApiTokenResponse",
				Fields: graphql.Fields{
					"msg": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"token": &graphql.ArgumentConfig{
					Type: graphql.String, // optional when token_id is provided (legacy full-secret revoke)
				},
				"token_id": &graphql.ArgumentConfig{
					Type: graphql.String, // preferred revoke key (matches ProjectToken.token_id)
				},
				"duration": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteProjectTokenResolverFn,
		},
		"createWebHook": &graphql.Field{
			Name: "CreateWebHook",
			Type: privateSchemaObjects.WebHookModelObject,
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"model": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"events": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
				},
				"url": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"logic_executions": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
			},
			Resolve: s.CreateWebHookResolverFn,
		},
		"deleteWebHook": &graphql.Field{
			Name: "DeleteWebHook",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "DeleteWebHookResponse",
				Fields: graphql.Fields{
					"msg": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteWebHookResolverFn,
		},
		/*"createProject": &graphql.Field{
			Name: "CreateProject",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "CreateProjectsResponse",
				Fields: graphql.Fields{
					"id": &graphql.Field{
						Type: graphql.String,
					},
					"name": &graphql.Field{
						Type: graphql.String,
					},
					"description": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"description": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.CreateProjectResolverFn,
		},*/
		"updateProject": &graphql.Field{
			Name: "UpdateProject",
			Type: privateSchemaObjects.ProjectDetailsObject,
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"description": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"project_secret_key": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"settings": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "UpdateSettingsPayload",
						Fields: graphql.InputObjectConfigFieldMap{
							"default_locale": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"locals": &graphql.InputObjectFieldConfig{
								Type: graphql.NewList(graphql.String),
							},
							"enable_revision_history": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"system_graphql_hooks": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"default_storage_plugin": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"default_function_plugin": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
						},
					}),
				},
				"add_team_member": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "AddTeamMemberPayload",
						Fields: graphql.InputObjectConfigFieldMap{
							"email": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.String),
							},
							"role": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"team_id": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"administrative_permissions": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
							},
						},
					}),
				},
				"remove_team_member": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "RemoveTeamMemberPayload",
						Fields: graphql.InputObjectConfigFieldMap{
							"member_id": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.String),
							},
						},
					}),
				},
				"driver": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "UpdateProjectDriverDetails",
						Fields: graphql.InputObjectConfigFieldMap{
							"host": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.String),
							},
							"port": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.String),
							},
							"db": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
							},
							"user": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
							},
							"password": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
							},
							"access_key": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
							},
							"secret_key": &graphql.InputObjectFieldConfig{
								Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
							},
						},
					}),
				},
			},
			Resolve: s.UpdateProjectResolverFn,
		},
		"updateProjectAuthenticationSettings": &graphql.Field{
			Type: projectAuthenticationSettingsPayload,
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(updateProjectAuthenticationInputType),
				},
			},
			Resolve: s.UpdateProjectAuthenticationSettingsResolverFn,
		},
		"updateProjectStorageSettings": &graphql.Field{
			Type: projectStorageSettingsPayload,
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(updateProjectStorageInputType),
				},
			},
			Resolve: s.UpdateProjectStorageSettingsResolverFn,
		},
		"updateProfile": &graphql.Field{
			Name: "UpdateProfile",
			Type: privateSchemaObjects.SystemUserObject,
			Args: graphql.FieldConfigArgument{
				"first_name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"last_name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"username": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"role": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"organization_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"old_pass": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"new_pass": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.UpdateProfileResolverFn,
		},
		"addModelToProject": &graphql.Field{
			Name: "CreateCustomTypes",
			Type: graphql.NewList(privateSchemaObjects.ModelTypeObject),
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"single_record": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"is_common_model": &graphql.ArgumentConfig{
					Type:        graphql.Boolean,
					Description: "SaaS: project-wide model without tenant_id scoping",
				},
			},
			Resolve: s.AddModelToProjectResolverFn,
		},
		"runModelMigrations": &graphql.Field{
			Name: "CreateCustomTypes",
			Type: graphql.NewList(privateSchemaObjects.ModelTypeObject),
			Args: graphql.FieldConfigArgument{
				"force_recreate": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
			},
			Resolve: s.RunModelMigrationsResolverFn,
		},
		"updateModel": &graphql.Field{
			Name: "UpdateModelTypes",
			Type: privateSchemaObjects.ModelTypeObject,
			Args: graphql.FieldConfigArgument{
				"type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(scaler.UpdateModelTypeEnum),
				},
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"new_name": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "used in duplicate type",
				},
				"single_page_model": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"is_common_model": &graphql.ArgumentConfig{
					Type:        graphql.Boolean,
					Description: "SaaS: project-wide model without tenant_id scoping",
				},
			},
			Resolve: s.UpdateModelResolverFn,
		},
		"upsertFunctionToProject": &graphql.Field{
			Name: "UpsertFunctionToProjectType",
			Type: privateSchemaObjects.CloudFunctionObject,
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"description": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"graphql_schema_type": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"function_provider_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"provider_exported_variable": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"function_exported_variable": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"function_path": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"update": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"request": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"request_payload_is_optional": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"response": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"response_is_array": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"function_connected": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"env_vars": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "Function_Provider_Env_Vars_Payload",
						Fields: graphql.InputObjectConfigFieldMap{
							"key": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"value": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
						},
					})),
				},
				/*"region": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"remote_function_name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},*/
				"runtime_config": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "Function_Provider_Config_Payload",
						Fields: graphql.InputObjectConfigFieldMap{
							"runtime": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"memory": &graphql.InputObjectFieldConfig{
								Type: graphql.Int,
							},
							"handler": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"time_out": &graphql.InputObjectFieldConfig{
								Type: graphql.Int,
							},
						},
					}),
				},
				"language": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"binary_url": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"source": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"trigger_type": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"capabilities": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
			},
			Resolve: s.UpsertFunctionToProjectResolverFn,
		},
		"upsertRoleToProject": &graphql.Field{
			Name: "UpsertRoleToProject",
			Type: privateSchemaObjects.RoleObject,
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"is_admin": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"api_permissions": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
				"logic_executions": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
				/*				"administrative_permissions": &graphql.ArgumentConfig{
								Type: graphql.NewList(graphql.String),
							},*/
			},
			Resolve: s.UpsertRoleToProjectResolverFn,
		},
		"upsertPlanToProject": &graphql.Field{
			Name: "UpsertPlanToProject",
			Type: privateSchemaObjects.PlanObject,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"description": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"api_permissions": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
				"logic_executions": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
				"quotas": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
			},
			Resolve: s.UpsertPlanToProjectResolverFn,
		},
		"duplicatePlanInProject": &graphql.Field{
			Name: "DuplicatePlanInProject",
			Type: privateSchemaObjects.PlanObject,
			Args: graphql.FieldConfigArgument{
				"source_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"new_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.DuplicatePlanInProjectResolverFn,
		},
		"duplicateRoleInProject": &graphql.Field{
			Name: "DuplicateRoleInProject",
			Type: privateSchemaObjects.RoleObject,
			Args: graphql.FieldConfigArgument{
				"source_role": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"new_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DuplicateRoleInProjectResolverFn,
		},
		/*"deleteModelFromProject": &graphql.Field{
			Name: "DeleteCustomTypes",
			Type: graphql.NewList(objects.ModelType),
			Args: graphql.FieldConfigArgument{
				"model": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteModelResolverFn,
		},*/
		"deleteFunctionFromProject": &graphql.Field{
			Name: "DeleteApitoFunction",
			Type: graphql.NewList(privateSchemaObjects.CloudFunctionObject),
			Args: graphql.FieldConfigArgument{
				"function": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteFunctionResolverFn,
		},
		"deployFunctionToProject": &graphql.Field{
			Name: "DeployFunctionToProject",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "DeployFunctionToProjectResponse",
				Fields: graphql.Fields{
					"function":   &graphql.Field{Type: privateSchemaObjects.CloudFunctionObject},
					"revision":   &graphql.Field{Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "DeployFunctionRevisionType",
						Fields: graphql.Fields{
							"id":            &graphql.Field{Type: graphql.String},
							"revision":      &graphql.Field{Type: graphql.Float},
							"artifact_key":  &graphql.Field{Type: graphql.String},
							"artifact_hash": &graphql.Field{Type: graphql.String},
							"created_at":    &graphql.Field{Type: graphql.String},
						},
					})},
					"deployment": &graphql.Field{Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "DeployFunctionDeploymentType",
						Fields: graphql.Fields{
							"id":          &graphql.Field{Type: graphql.String},
							"revision_id": &graphql.Field{Type: graphql.String},
							"status":      &graphql.Field{Type: graphql.String},
							"created_at":  &graphql.Field{Type: graphql.String},
						},
					})},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"source": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.DeployFunctionToProjectResolverFn,
		},
		"rollbackFunctionDeployment": &graphql.Field{
			Name: "RollbackFunctionDeployment",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "RollbackFunctionDeploymentResponse",
				Fields: graphql.Fields{
					"function": &graphql.Field{Type: privateSchemaObjects.CloudFunctionObject},
					"revision": &graphql.Field{Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "RollbackFunctionRevisionType",
						Fields: graphql.Fields{
							"id":           &graphql.Field{Type: graphql.String},
							"artifact_key": &graphql.Field{Type: graphql.String},
							"created_at":   &graphql.Field{Type: graphql.String},
						},
					})},
					"deployment": &graphql.Field{Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "RollbackFunctionDeploymentType",
						Fields: graphql.Fields{
							"id":          &graphql.Field{Type: graphql.String},
							"revision_id": &graphql.Field{Type: graphql.String},
							"status":      &graphql.Field{Type: graphql.String},
							"rollback_of": &graphql.Field{Type: graphql.String},
							"created_at":  &graphql.Field{Type: graphql.String},
						},
					})},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"revision_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.RollbackFunctionDeploymentResolverFn,
		},
		"testFunctionDraft": &graphql.Field{
			Name: "TestFunctionDraft",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "TestFunctionDraftResponse",
				Fields: graphql.Fields{
					"ok":            &graphql.Field{Type: graphql.Boolean},
					"response":      &graphql.Field{Type: scaler.ScalarJSON},
					"error":         &graphql.Field{Type: graphql.String},
					"error_class":   &graphql.Field{Type: graphql.String},
					"duration_ms":   &graphql.Field{Type: graphql.Int},
					"invocation_id": &graphql.Field{Type: graphql.String},
					"logs": &graphql.Field{Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
						Name: "TestFunctionDraftLogEntry",
						Fields: graphql.Fields{
							"level":   &graphql.Field{Type: graphql.String},
							"message": &graphql.Field{Type: graphql.String},
						},
					}))},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"source": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"payload": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
				"tenant_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.TestFunctionDraftResolverFn,
		},
		"deleteRoleFromProject": &graphql.Field{
			Name: "DeleteApitoFunction",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "DeleteApitoFunctionResponse",
				Fields: graphql.Fields{
					"message": &graphql.Field{Type: graphql.String},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"role": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteRoleResolverFn,
		},
		"deletePlanFromProject": &graphql.Field{
			Name: "DeletePlanFromProject",
			Type: graphql.NewList(privateSchemaObjects.PlanObject),
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeletePlanFromProjectResolverFn,
		},
		"upsertFieldToModel": &graphql.Field{
			Name: "UpsertFieldToModel",
			Type: privateSchemaObjects.FieldInfoObject,
			Args: graphql.FieldConfigArgument{
				"is_update": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				/* "repeated_group_identifier": &graphql.ArgumentConfig{
					Type: graphql.String,
				}, */
				"parent_field": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"is_object_field": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"serial": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
				"field_label": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"field_description": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"input_type": &graphql.ArgumentConfig{
					Type: enums.InputTypeEnum,
				},
				"field_type": &graphql.ArgumentConfig{
					Type: enums.FieldTypeEnum,
				},
				"field_sub_type": &graphql.ArgumentConfig{
					Type: enums.FieldSubTypeEnum,
				},
				"validation": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "module_validation_payload",
						Fields: graphql.InputObjectConfigFieldMap{
							"locals": &graphql.InputObjectFieldConfig{
								Type: graphql.NewList(graphql.String),
							},
							"required": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"as_title": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"placeholder": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"hide": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"is_email": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"unique": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"is_multi_choice": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"fixed_list_elements": &graphql.InputObjectFieldConfig{
								Type: scaler.ScalarJSONArray,
							},
							"fixed_list_element_type": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"is_gallery": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"is_url": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
						},
					}),
				},
				"enable_indexing": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
			},
			Resolve: s.UpsertFieldToModelResolverFn,
		},

		/* "deleteFieldFromModel": &graphql.Field{
			Name: "DeleteFieldFromModel",
			Type: privateSchemaObjects.ModelTypeObject,
			Args: graphql.FieldConfigArgument{
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				/* "repeated_group_identifier": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"parent_field": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"identifier": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"is_relation": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Boolean),
				},
			},
			Resolve: s.DeleteFieldTypeResolverFn,
		}, */
		"modelFieldOperation": &graphql.Field{
			Name: "FieldOperationType",
			Type: privateSchemaObjects.FieldInfoObject,
			Args: graphql.FieldConfigArgument{
				"type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(enums.FieldOperationEnums),
				},
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"field_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"new_name": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				/* "repeated_group_identifier": &graphql.ArgumentConfig{
					Type: graphql.String,
				}, */
				"parent_field": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"single_page_model": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"known_as": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"moved_to": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"changed_type": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.ModelFieldOperationResolverFn,
		},
		// field operation
		"rearrangeSerialOfField": &graphql.Field{
			Name: "RearrangeSerialOfFieldType",
			Type: privateSchemaObjects.ModelTypeObject,
			Args: graphql.FieldConfigArgument{
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"field_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"new_position": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
				"parent_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"move_type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.RearrangeFieldOfModelResolverFn,
		},
		// ---- end ----
		/*		"uploadImageFromUrl": &graphql.Field{
				Name: "UploadImageFromURL",
				Type: objects.FileDetailsType,
				Args: graphql.FieldConfigArgument{
					"url": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: s.UploadImageFromURLResolverFn,
			},*/
		"upsertConnectionToModel": &graphql.Field{
			Name: "AddFieldToCustomType",
			Type: graphql.NewList(privateSchemaObjects.ConnectionTypeObject),
			Args: graphql.FieldConfigArgument{
				"known_as": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"from": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"to": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"forward_connection_type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(enums.RelationTypeEnum),
				},
				"reverse_connection_type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(enums.RelationTypeEnum),
				},
			},
			Resolve: s.CreateConnectionTypeResolverFn,
		},
		"deleteConnectionFromModel": &graphql.Field{
			Name: "RemoveConnectionFromModel",
			Type: graphql.NewList(privateSchemaObjects.ConnectionTypeObject),
			Args: graphql.FieldConfigArgument{
				"known_as": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"from": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"to": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteConnectionFromModelResolverFn,
		},
		"upsertModelData": &graphql.Field{
			Name: "UpsertModelDataType",
			Type: privateSchemaObjects.DocModelObject,
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"single_page_data": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"force_update": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"local": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"payload": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
				"status": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"connect": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
				"disconnect": &graphql.ArgumentConfig{
					Type: scaler.ScalarJSON,
				},
			},
			Resolve: s.UpsertModelDataFnFn,
		},
		"deleteModelData": &graphql.Field{
			Name: "DeleteModelDataType",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "DeleteModelDataResponse",
				Fields: graphql.Fields{
					"id": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteModelDataFnFn,
		},
		"duplicateModelData": &graphql.Field{
			Name: "DuplicateModelDataType",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "DuplicateModelDataResponse",
				Fields: graphql.Fields{
					"id": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DuplicateModelDataFnFn,
		},
		/*	"deleteProject": &graphql.Field{
			Name: "deleteProject",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "DeleteProjectResponse",
				Fields: graphql.Fields{
					"msg": &graphql.Field{
						Type: graphql.String,
					},
					"token": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"token": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteProjectResolverFn,
		},*/
		"retrySchemaOperation": &graphql.Field{
			Name: "RetrySchemaOperation",
			Type: schemaOperationGraphQLObject(),
			Args: graphql.FieldConfigArgument{
				"operation_id": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.RetrySchemaOperationResolverFn,
		},
	}
}
