package objects

import (
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/scaler"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-platform/graphql"
)

func (s *SchemaObjects) GetAuditLogObject(systemUserObj, projectDetailsObj *graphql.Object) *graphql.Object {

	s.dataLoader["user_loader"] = dataloader.NewBatchedLoader(s.SystemDataloaders.UserLoaderFn)
	//s.dataLoader["video_loader"] = dataloader.NewBatchedLoader(s.SystemDataloaders.VideoLoaderFn)

	return graphql.NewObject(graphql.ObjectConfig{
		Name: "AuditLogInfo",
		Fields: graphql.Fields{
			"_key": &graphql.Field{Type: graphql.String},
			"id":   &graphql.Field{Type: graphql.String},
			"user": &graphql.Field{
				Type: systemUserObj,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					source := p.Source.(*models.SystemUser)

					var (
						v       = p.Context.Value
						loaders = v("loaders").(map[string]*dataloader.Loader)
						key     = models.NewResolverKey(source.ID, nil)
						lid     = "user_loader"
					)

					thunk := loaders[lid].Load(p.Context, key)

					return func() (interface{}, error) {
						return thunk()
					}, nil
				},
			},
			"project":                &graphql.Field{Type: projectDetailsObj},
			"request_payload":        &graphql.Field{Type: graphql.String},
			"request_path":           &graphql.Field{Type: graphql.String},
			"response_code":          &graphql.Field{Type: graphql.Int},
			"response_payload":       &graphql.Field{Type: graphql.String},
			"activity":               &graphql.Field{Type: graphql.String},
			"internal_function":      &graphql.Field{Type: graphql.String},
			"graphql_operation_name": &graphql.Field{Type: graphql.String},
			"graphql_payload":        &graphql.Field{Type: graphql.String},
			"graphql_variable":       &graphql.Field{Type: graphql.String},
			"internal_error":         &graphql.Field{Type: graphql.String},
			"created_at":             &graphql.Field{Type: graphql.String},
		},
	})
}

func (s *SchemaObjects) GetSystemUserObject(prefix string) *graphql.Object {

	//s.dataLoader["organization_loader"] = dataloader.NewBatchedLoader(s.SystemDataloaders.OrganizationsLoaderFn)

	name := prefix + "SystemUserInfo"
	return graphql.NewObject(graphql.ObjectConfig{
		Name: name,
		Fields: graphql.Fields{
			"_key":                       &graphql.Field{Type: graphql.String},
			"id":                         &graphql.Field{Type: graphql.String},
			"first_name":                 &graphql.Field{Type: graphql.String},
			"last_name":                  &graphql.Field{Type: graphql.String},
			"role":                       &graphql.Field{Type: graphql.String},
			"username":                   &graphql.Field{Type: graphql.String},
			"email":                      &graphql.Field{Type: graphql.String},
			"avatar":                     &graphql.Field{Type: graphql.String},
			"current_project_id":         &graphql.Field{Type: graphql.String},
			"project_user":               &graphql.Field{Type: graphql.Boolean},
			"administrative_permissions": &graphql.Field{Type: graphql.NewList(graphql.String)},

			"is_admin": &graphql.Field{Type: graphql.Boolean},

			"read_only_project":      &graphql.Field{Type: graphql.Boolean},
			"project_limit":          &graphql.Field{Type: graphql.Int},
			"user_subscription_type": &graphql.Field{Type: graphql.String}, // Adjust this type accordingly

			"created_at": &graphql.Field{Type: graphql.String},
			"updated_at": &graphql.Field{Type: graphql.String},

			"project_assigned_role":      &graphql.Field{Type: graphql.String},
			"project_access_permissions": &graphql.Field{Type: graphql.NewList(graphql.String)},

			//"organizations": &graphql.Field{
			//	Type: graphql.NewList(s.OrganizationObject),
			//	Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			//		source := p.Source.(*protobuff.SystemUser)
			//
			//		var (
			//			v       = p.Context.Value
			//			loaders = v("loaders").(map[string]*dataloader.Loader)
			//			key     = models.NewResolverKey(source.ID, nil)
			//			lid     = "organization_loader"
			//		)
			//
			//		thunk := loaders[lid].Load(p.Context, key)
			//
			//		return func() (interface{}, error) {
			//			return thunk()
			//		}, nil
			//	},
			//},
			//"teams":                      &graphql.Field{Type: graphql.NewList(teamsObj)},
		},
	})
}

// Methods for other types as defined earlier:
func (s *SchemaObjects) GetProjectDetailsObject(userDefinedSchemaObj, pluginDetailsObj, settingsObj, apiTokenObj, driverCredObj, systemUserObj, systemMsgObj, workSpaceObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "ProjectModel",
		Fields: graphql.Fields{
			"_key":                         &graphql.Field{Type: graphql.String},
			"id":                           &graphql.Field{Type: graphql.String},
			"name":                         &graphql.Field{Type: graphql.String},
			"description":                  &graphql.Field{Type: graphql.String},
			"project_icon":                 &graphql.Field{Type: graphql.String},
			"schema":                       &graphql.Field{Type: userDefinedSchemaObj},
			"created_at":                   &graphql.Field{Type: graphql.String},
			"updated_at":                   &graphql.Field{Type: graphql.String},
			"expire_at":                    &graphql.Field{Type: graphql.String},
			"plugins":                      &graphql.Field{Type: graphql.NewList(pluginDetailsObj)},
			"settings":                     &graphql.Field{Type: settingsObj},
			"roles":                        &graphql.Field{Type: scaler.ScalarJSON},
			"plans":                        &graphql.Field{Type: scaler.ScalarJSON},
			"tokens":                       &graphql.Field{Type: graphql.NewList(apiTokenObj)},
			"driver":                       &graphql.Field{Type: driverCredObj},
			"project_template":             &graphql.Field{Type: graphql.String},
			"trial_ends":                   &graphql.Field{Type: graphql.String},
			"teams":                        &graphql.Field{Type: graphql.NewList(systemUserObj)},
			"system_messages":              &graphql.Field{Type: graphql.NewList(systemMsgObj)},
			"workspaces":                   &graphql.Field{Type: graphql.NewList(workSpaceObj)},
			"default_storage_plugin":       &graphql.Field{Type: graphql.String},
			"default_function_plugin":      &graphql.Field{Type: graphql.String},
			"project_type": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "general", nil
				},
			},
			"project_plan":                 &graphql.Field{Type: graphql.String},
			"project_secret_key":           &graphql.Field{Type: graphql.String},
		},
	})
}

func (s *SchemaObjects) GetModelTypeObject(fieldInfoObj, connectionTypeObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "ModelType",
		Fields: graphql.Fields{
			"name":             &graphql.Field{Type: graphql.String},
			"fields":           &graphql.Field{Type: graphql.NewList(fieldInfoObj)},
			"connections":      &graphql.Field{Type: graphql.NewList(connectionTypeObj)},
			"hook_ids":         &graphql.Field{Type: graphql.NewList(graphql.String)},
			"locals":           &graphql.Field{Type: graphql.NewList(graphql.String)},
			"repeated_groups":  &graphql.Field{Type: graphql.NewList(graphql.String)},
			"system_generated": &graphql.Field{Type: graphql.Boolean},
			"single_page":      &graphql.Field{Type: graphql.Boolean},
			"single_page_uuid": &graphql.Field{Type: graphql.String},
			"has_connections":  &graphql.Field{Type: graphql.Boolean},
			"enable_revision":  &graphql.Field{Type: graphql.Boolean},
			"is_project_auth_user_model": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if m, ok := p.Source.(*models.ModelType); ok {
						return models.ModelIsProjectAuthUserModel(m), nil
					}
					return false, nil
				},
			},
			"revision_filter": &graphql.Field{Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name: "RevisionFilter",
				Fields: graphql.Fields{
					"key":   &graphql.Field{Type: graphql.String},
					"value": &graphql.Field{Type: graphql.String},
				},
			}))},
		},
	})
}

func (s *SchemaObjects) GetCloudFunctionObject(cloudFunctionRequestResponseObj, funcEnvVarObj, functionRuntimeConfigObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "CloudFunctionType",
		Fields: graphql.Fields{
			"id":                         &graphql.Field{Type: graphql.String},
			"name":                       &graphql.Field{Type: graphql.String},
			"description":                &graphql.Field{Type: graphql.String},
			"function_path":              &graphql.Field{Type: graphql.String},
			"graphql_schema_type":        &graphql.Field{Type: graphql.String},
			"request":                    &graphql.Field{Type: cloudFunctionRequestResponseObj},
			"response":                   &graphql.Field{Type: cloudFunctionRequestResponseObj},
			"env_vars":                   &graphql.Field{Type: graphql.NewList(funcEnvVarObj)},
			"function_provider_id":       &graphql.Field{Type: graphql.String},
			"function_connected":         &graphql.Field{Type: graphql.Boolean},
			"provider_exported_variable": &graphql.Field{Type: graphql.String},
			"function_exported_variable": &graphql.Field{Type: graphql.String},
			"runtime_config":             &graphql.Field{Type: functionRuntimeConfigObj},
			"language":                   &graphql.Field{Type: graphql.String},
			"binary_url":                 &graphql.Field{Type: graphql.String},
			"source":                     &graphql.Field{Type: graphql.String},
			"trigger_type":               &graphql.Field{Type: graphql.String},
			"active_revision_id":         &graphql.Field{Type: graphql.String},
			"active_revision_hash":       &graphql.Field{Type: graphql.String},
			"capabilities":               &graphql.Field{Type: graphql.NewList(graphql.String)},
			"updated_at":                 &graphql.Field{Type: graphql.String},
			"created_at":                 &graphql.Field{Type: graphql.String},
			"rest_api_secret_url_key":    &graphql.Field{Type: graphql.String},
		},
	})
}

func (s *SchemaObjects) GetCloudFunctionRequestResponseType(fieldInfoObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "CloudFunctionRequestResponse",
		Fields: graphql.Fields{
			"model":            &graphql.Field{Type: graphql.String},
			"params":           &graphql.Field{Type: graphql.NewList(fieldInfoObj)},
			"is_array":         &graphql.Field{Type: graphql.Boolean},
			"optional_payload": &graphql.Field{Type: graphql.Boolean},
		},
	})
}

func (s *SchemaObjects) GetFunctionRuntimeConfigTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.ApitoFunctionRuntimeConfig{})
	return obj
}

func (s *SchemaObjects) GetUserDefinedSchemaObject(modelTypeObj, cloudFunctionObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "UserDefinedSchema",
		Fields: graphql.Fields{
			"models":    &graphql.Field{Type: graphql.NewList(modelTypeObj)},
			"functions": &graphql.Field{Type: graphql.NewList(cloudFunctionObj)},
		},
	})
}

func (s *SchemaObjects) GetPluginDetailsObject(funcEnvVarObject *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "PluginDetailsFields",
		Fields: graphql.Fields{
			"icon":              &graphql.Field{Type: graphql.String},
			"id":                &graphql.Field{Type: graphql.String},
			"title":             &graphql.Field{Type: graphql.String},
			"version":           &graphql.Field{Type: graphql.String},
			"description":       &graphql.Field{Type: graphql.String},
			"type":              &graphql.Field{Type: enums.PluginTypeEnums}, // Adjust the type accordingly
			"role":              &graphql.Field{Type: graphql.String},
			"enable":            &graphql.Field{Type: graphql.Boolean},
			"exported_variable": &graphql.Field{Type: graphql.String},
			"env_vars":          &graphql.Field{Type: graphql.NewList(funcEnvVarObject)},
			"repository_url":    &graphql.Field{Type: graphql.String},
			"branch":            &graphql.Field{Type: graphql.String},
			"author":            &graphql.Field{Type: graphql.String},
			"load_status":       &graphql.Field{Type: enums.PluginLoadTypeEnums},  // Adjust the type accordingly
			"activate_status":   &graphql.Field{Type: enums.PluginActivationType}, // Adjust the type accordingly
			"used_in_projects":  &graphql.Field{Type: graphql.NewList(graphql.String)},
		},
	})
}

func (s *SchemaObjects) GetFieldInfoObject(validationTypeObj *graphql.Object) *graphql.Object {
	// SubFieldInfo is recursive so projectModelsInfo can return arbitrarily nested
	// repeated/object trees (e.g. exam.routine.details.date_and_time). The previous
	// NestedSubFieldInfo leaf type truncated at depth 2 and hid deeper children from
	// CLI/MCP schema sync.
	var subFieldInfoType *graphql.Object
	subFieldInfoType = graphql.NewObject(graphql.ObjectConfig{
		Name: "SubFieldInfo",
		Fields: graphql.FieldsThunk(func() graphql.Fields {
			return graphql.Fields{
				"identifier":       &graphql.Field{Type: graphql.String},
				"description":      &graphql.Field{Type: graphql.String},
				"input_type":       &graphql.Field{Type: graphql.String},
				"field_type":       &graphql.Field{Type: graphql.String},
				"field_sub_type":   &graphql.Field{Type: graphql.String},
				"validation":       &graphql.Field{Type: validationTypeObj},
				"serial":           &graphql.Field{Type: graphql.Int},
				"label":            &graphql.Field{Type: graphql.String},
				"system_generated": &graphql.Field{Type: graphql.Boolean},
				"parent_field":     &graphql.Field{Type: graphql.String},
				"sub_field_info":   &graphql.Field{Type: graphql.NewList(subFieldInfoType)},
			}
		}),
	})

	return graphql.NewObject(graphql.ObjectConfig{
		Name: "FieldInfo",
		Fields: graphql.Fields{
			"identifier":       &graphql.Field{Type: graphql.String},
			"description":      &graphql.Field{Type: graphql.String},
			"input_type":       &graphql.Field{Type: graphql.String},
			"field_type":       &graphql.Field{Type: graphql.String},
			"field_sub_type":   &graphql.Field{Type: graphql.String},
			"system_generated": &graphql.Field{Type: graphql.Boolean},
			"sub_field_info":   &graphql.Field{Type: graphql.NewList(subFieldInfoType)},
			"validation":       &graphql.Field{Type: validationTypeObj},
			"serial":           &graphql.Field{Type: graphql.Int},
			"label":            &graphql.Field{Type: graphql.String},
			//"repeated_group_identifier": &graphql.Field{Type: graphql.String},
			"parent_field":    &graphql.Field{Type: graphql.String},
			"is_object_field": &graphql.Field{Type: graphql.Boolean},
			"enable_indexing": &graphql.Field{Type: graphql.Boolean},
		},
	})
}

func (s *SchemaObjects) GetConnectionTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.ConnectionType{})
	return obj
}

func (s *SchemaObjects) GetFileDetailsTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.FileDetails{})
	return obj
}

func (s *SchemaObjects) GetDocModelTypeObject() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "DocumentModelType",
		Fields: graphql.Fields{
			"_key":            &graphql.Field{Type: graphql.String},
			"id":              &graphql.Field{Type: graphql.String},
			"type":            &graphql.Field{Type: graphql.String},
			"data":            &graphql.Field{Type: scaler.ScalarJSON},
			"meta":            &graphql.Field{Type: s.GetMetaObject()},
			"expire_at":       &graphql.Field{Type: graphql.String},
			"relation_doc_id": &graphql.Field{Type: graphql.String},
		},
	})
}

func (s *SchemaObjects) GetRoleObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.Role{})
	return obj
}

func (s *SchemaObjects) GetPlanObject() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Plan",
		Fields: graphql.Fields{
			"id":               &graphql.Field{Type: graphql.String},
			"name":             &graphql.Field{Type: graphql.String},
			"description":      &graphql.Field{Type: graphql.String},
			"api_permissions":  &graphql.Field{Type: scaler.ScalarJSON},
			"logic_executions": &graphql.Field{Type: graphql.NewList(graphql.String)},
			"quotas":           &graphql.Field{Type: scaler.ScalarJSON},
			"system_generated": &graphql.Field{Type: graphql.Boolean},
			"currency":          &graphql.Field{Type: graphql.String},
			"price_monthly":     &graphql.Field{Type: graphql.Float},
			"play_product_id":   &graphql.Field{Type: graphql.String},
			"play_base_plan_id": &graphql.Field{Type: graphql.String},
			"paddle_price_id":   &graphql.Field{Type: graphql.String},
			"prices":            &graphql.Field{Type: scaler.ScalarJSON},
			"provider_products": &graphql.Field{Type: scaler.ScalarJSON},
		},
	})
}

// be-careful this creates panic so if you need this then create one
// do not build it
//func (s *SchemaObjects) GetProjectWithRoleObject() *graphql.Object {
//	obj, _ := utility.GetGraphQLObject(protobuff.ProjectWithRoles{})
//	return obj
//}

func (s *SchemaObjects) GetProjectUsageModelTypeObject(usageTrackingModelObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "ProjectUsageModelType",
		Fields: graphql.Fields{
			"usages": &graphql.Field{Type: usageTrackingModelObj},
			"limits": &graphql.Field{Type: usageTrackingModelObj},
		},
	})
}

func (s *SchemaObjects) GetWebHookModelTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.Webhook{})
	return obj
}

func (s *SchemaObjects) GetValidationTypeObject() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Validation",
		Fields: graphql.Fields{
			"required":                &graphql.Field{Type: graphql.Boolean},
			"hide":                    &graphql.Field{Type: graphql.Boolean},
			"as_title":                &graphql.Field{Type: graphql.Boolean},
			"locals":                  &graphql.Field{Type: graphql.NewList(graphql.String)},
			"unique":                  &graphql.Field{Type: graphql.Boolean},
			"char_limit":              &graphql.Field{Type: graphql.NewList(graphql.Int)},
			"int_range_limit":         &graphql.Field{Type: graphql.NewList(graphql.Int)},
			"double_range_limit":      &graphql.Field{Type: graphql.NewList(graphql.Float)},
			"placeholder":             &graphql.Field{Type: graphql.String},
			"fixed_list_elements":     &graphql.Field{Type: scaler.ScalarJSONArray},
			"fixed_list_element_type": &graphql.Field{Type: graphql.String},
			"is_multi_choice":         &graphql.Field{Type: graphql.Boolean},
			"is_email":                &graphql.Field{Type: graphql.Boolean},
			"is_gallery":              &graphql.Field{Type: graphql.Boolean},
			"is_password":             &graphql.Field{Type: graphql.Boolean},
			"is_url":                  &graphql.Field{Type: graphql.Boolean},
			"hide_for_roles":          &graphql.Field{Type: graphql.NewList(graphql.String)},
		},
	})
}

func (s *SchemaObjects) GetFunctionEnvVariablesObject() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "EnvVariable",
		Fields: graphql.Fields{
			"key":       &graphql.Field{Type: graphql.String},
			"value":     &graphql.Field{Type: graphql.String},
			"hide":      &graphql.Field{Type: graphql.Boolean},
			"is_system": &graphql.Field{Type: graphql.Boolean},
		},
	})
}

// GetOrganizationObject retrieves the GraphQL object for Organization
func (s *SchemaObjects) GetOrganizationObject(prefix string) *graphql.Object {

	s.dataLoader["organization_teams_loader"] = dataloader.NewBatchedLoader(s.SystemDataloaders.OrganizationsTeamsLoaderFn)
	s.dataLoader["organization_users_loader"] = dataloader.NewBatchedLoader(s.SystemDataloaders.OrganizationsUsersLoaderFn)

	name := prefix + "Organization"
	return graphql.NewObject(graphql.ObjectConfig{
		Name: name,
		Fields: graphql.Fields{
			"_key":        &graphql.Field{Type: graphql.String},
			"id":          &graphql.Field{Type: graphql.String},
			"name":        &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
			"teams": &graphql.Field{
				Type: graphql.NewList(s.GetTeamObject(name)),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					source := p.Source.(*models.Organization)

					var (
						v       = p.Context.Value
						loaders = v("loaders").(map[string]*dataloader.Loader)
						key     = models.NewResolverKey(source.ID, nil)
						lid     = "organization_teams_loader"
					)

					thunk := loaders[lid].Load(p.Context, key)

					return func() (interface{}, error) {
						return thunk()
					}, nil
				},
			},
			"users": &graphql.Field{
				Type: graphql.NewList(s.GetSystemUserObject(name)),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					source := p.Source.(*models.Organization)

					var (
						v       = p.Context.Value
						loaders = v("loaders").(map[string]*dataloader.Loader)
						key     = models.NewResolverKey(source.ID, nil)
						lid     = "organization_users_loader"
					)

					thunk := loaders[lid].Load(p.Context, key)

					return func() (interface{}, error) {
						return thunk()
					}, nil
				},
			},
		},
	})
}

// GetSettingsObject retrieves the GraphQL object for AddOnsDetails
func (s *SchemaObjects) GetSettingsObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.ProjectSettings{})
	return obj
}

// GetProjectTokenObject retrieves the GraphQL object for ProjectToken
func (s *SchemaObjects) GetProjectTokenObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.ProjectToken{})
	return obj
}

// GetMetaObject retrieves the GraphQL object for ProjectToken
func (s *SchemaObjects) GetMetaObject() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "MetaField",
		Fields: graphql.Fields{
			"created_at": &graphql.Field{Type: graphql.String},
			"updated_at": &graphql.Field{Type: graphql.String},
			"created_by": &graphql.Field{Type: s.GetSystemUserObject("CreatedBy"),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					source := p.Source.(*types.MetaField)
					// #TODO: get the user from the database
					return source.CreatedBy, nil
				},
			},
			"last_modified_by": &graphql.Field{Type: s.GetSystemUserObject("ModifiedBy"),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					source := p.Source.(*types.MetaField)
					// #TODO: get the user from the database
					return source.LastModifiedBy, nil
				},
			},
			"status":           &graphql.Field{Type: graphql.String},
			"root_revision_id": &graphql.Field{Type: graphql.String},
			"revision":         &graphql.Field{Type: graphql.Boolean},
			"revision_at":      &graphql.Field{Type: graphql.String},
			"resource_id":      &graphql.Field{Type: graphql.String},
		},
	})
}

// GetDriverCredentialObject retrieves the GraphQL object for DriverCredentials
func (s *SchemaObjects) GetDriverCredentialObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.DriverCredentials{})
	return obj
}

// GetSystemMessageObject retrieves the GraphQL object for SystemMessage
func (s *SchemaObjects) GetSystemMessageObject(prefix string) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: prefix + "SystemMessage",
		Fields: graphql.Fields{
			"_key":        &graphql.Field{Type: graphql.String},
			"message":     &graphql.Field{Type: graphql.String},
			"code":        &graphql.Field{Type: graphql.String},
			"redirection": &graphql.Field{Type: graphql.String},
			"hide":        &graphql.Field{Type: graphql.Boolean},
		},
	})
}

// GetWorkspaceObject retrieves the GraphQL object for Workspace
func (s *SchemaObjects) GetWorkspaceObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(models.Workspace{})
	return obj
}

// GetTeamObject retrieves the GraphQL object for Team
func (s *SchemaObjects) GetTeamObject(prefix string) *graphql.Object {
	name := prefix + "Team"
	return graphql.NewObject(graphql.ObjectConfig{
		Name: name,
		Fields: graphql.Fields{
			"_key":        &graphql.Field{Type: graphql.String},
			"id":          &graphql.Field{Type: graphql.String},
			"name":        &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
			"created_by":  &graphql.Field{Type: graphql.String},
			"users":       &graphql.Field{Type: graphql.NewList(s.GetSystemUserObject(name))},
			//"projects":    &graphql.Field{Type: graphql.NewList(s.ProjectDetailsObject)},
		},
	})
}
