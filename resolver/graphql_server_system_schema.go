package resolver

import (
	"github.com/apito-io/engine/resolver/dataloader"
	"github.com/apito-io/engine/scaler"
	"github.com/apito-io/engine/schemas/args"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) BuildServerQueriesAndMutations() {

	s.SystemDataloaders, _ = dataloader.GetSystemDataloader(s.SystemDriver)

	privateSchemaObjects := objects.GetSchemaObjects(s.SystemDriver, s.SystemDataloaders)
	privateSchemaObjects.ObjectModels = privateSchemaObjects.InitPrivateObjects()
	s.PrivateSchemaObjects = privateSchemaObjects

	s.SystemQueriesChan <- &graphql.Fields{
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
		/*"projectModelInfoByName": &graphql.Field{
			Name:    "ProjectModelInfoByName",
			Type:    privateSchemaObjects.ModelTypeObject,
			Args:    graphql.FieldConfigArgument{},
			Resolve: s.ProjectModelInfoResolverFn,
		},*/
		"getPlugins": &graphql.Field{
			Name: "GetSystemAndThirdPartyPlugins",
			Type: graphql.NewList(privateSchemaObjects.PluginDetailsObject),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "PluginsWhereFilterArgObject",
						Fields: graphql.InputObjectConfigFieldMap{
							"system_type": &graphql.InputObjectFieldConfig{
								Type: args.IntegerFilter,
							},
						},
					}),
				},
			},
			Resolve: s.ProjectsPlugins,
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
						Type: scaler.ScalarMap,
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
					Type: scaler.ScalarMap,
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
		"searchUsers": &graphql.Field{
			Name: "SearchUsersOfApito",
			Type: graphql.NewList(privateSchemaObjects.SystemUserObject),
			Args: graphql.FieldConfigArgument{
				"_id": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"filter": args.FilterArg,
				"where": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "SearchUsersWhereFilterArgObject",
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
			Resolve: s.SearchApitoUsersResolverFn,
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
						Type: scaler.ScalarMap,
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
						Type: graphql.String,
					},
					"plugins": &graphql.Field{
						Type: graphql.NewList(graphql.String),
					},
				},
			}),
			Resolve: s.LoadedFunctionProviderResolverFn,
		},
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
				},
				"type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(enums.PluginSystemType),
				},
				"plugin": &graphql.ArgumentConfig{
					Type: graphql.NewInputObject(graphql.InputObjectConfig{
						Name: "UpsertPluginToProjectPayload",
						Fields: graphql.InputObjectConfigFieldMap{
							"id": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"title": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"icon": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"version": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"description": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"type": &graphql.InputObjectFieldConfig{
								Type: enums.PluginTypeEnums,
							},
							"role": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"exported_variable": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							/*							"credentials": &graphql.InputObjectFieldConfig{
														Type: graphql.NewInputObject(graphql.InputObjectConfig{
															Name: "PluginCredentialsPayload",
															Fields: graphql.InputObjectConfigFieldMap{
																"account_id": &graphql.InputObjectFieldConfig{
																	Type: graphql.String,
																},
																"access_key": &graphql.InputObjectFieldConfig{
																	Type: graphql.String,
																},
																"secret_key": &graphql.InputObjectFieldConfig{
																	Type: graphql.String,
																},
																"api_key": &graphql.InputObjectFieldConfig{
																	Type: graphql.String,
																},
																"region": &graphql.InputObjectFieldConfig{
																	Type: graphql.String,
																},
																"url": &graphql.InputObjectFieldConfig{
																	Type: graphql.String,
																},
																"env": &graphql.InputObjectFieldConfig{
																	Type: graphql.String,
																},
															},
														}),
													},*/
							"env_vars": &graphql.InputObjectFieldConfig{
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
							"enable": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"repository_url": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"branch": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"author": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"activate_status": &graphql.InputObjectFieldConfig{
								Type: enums.PluginActivationType,
							},
						},
					}),
				},
			},
			Resolve: s.UpsertPluginResolverFn,
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
		"generateApiToken": &graphql.Field{
			Name: "GenerateApiToken",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "GenerateApiTokenResponse",
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
			Resolve: s.GenerateApiTokenResolverFn,
		},
		"deleteApiToken": &graphql.Field{
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
					Type: graphql.NewNonNull(graphql.String),
				},
				"duration": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
			},
			Resolve: s.DeleteApiTokenResolverFn,
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
				"locals": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
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
			},
			Resolve: s.AddModelToProjectResolverFn,
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
				"response": &graphql.ArgumentConfig{
					Type: graphql.String,
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
			},
			Resolve: s.UpsertFunctionToProjectResolverFn,
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
				"repeated_group_identifier": &graphql.ArgumentConfig{
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
							"list_type": &graphql.InputObjectFieldConfig{
								Type: graphql.String,
							},
							"is_multi_choice": &graphql.InputObjectFieldConfig{
								Type: graphql.Boolean,
							},
							"fixed_list_elements": &graphql.InputObjectFieldConfig{
								Type: graphql.NewList(graphql.String),
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
			},
			Resolve: s.UpsertFieldToModelResolverFn,
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
				"parent_field": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"old_serial": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
				"new_serial": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: s.RearrangeFieldOfModelResolverFn,
		},
		"deleteFieldFromModel": &graphql.Field{
			Name: "DeleteFieldFromModel",
			Type: privateSchemaObjects.ModelTypeObject,
			Args: graphql.FieldConfigArgument{
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"repeated_group_identifier": &graphql.ArgumentConfig{
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
		},
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
					Type: graphql.NewNonNull(graphql.String),
				},
				"repeated_group_identifier": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: s.ModelFieldOperationResolverFn,
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
					Type: graphql.NewNonNull(enums.ConnectionTypeEnum),
				},
				"reverse_connection_type": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(enums.ConnectionTypeEnum),
				},
			},
			Resolve: s.CreateConnectionTypeResolverFn,
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
				"model_name": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"local": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"payload": &graphql.ArgumentConfig{
					Type: scaler.ScalarMap,
				},
				"status": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"faker": &graphql.ArgumentConfig{
					Type: graphql.Boolean,
				},
				"connect": &graphql.ArgumentConfig{
					Type: scaler.ScalarMap,
				},
				"disconnect": &graphql.ArgumentConfig{
					Type: scaler.ScalarMap,
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
	}
}
