package objects

import (
	"github.com/apito-io/engine/interfaces"
	dl "github.com/apito-io/engine/resolver/dataloader"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-platform/graphql"
)

type ObjectModels struct {
	MetaObject                  *graphql.Object
	OrganizationObject          *graphql.Object
	SettingsObject              *graphql.Object
	APITokenObject              *graphql.Object
	DriverCredentialObject      *graphql.Object
	SystemMessageObject         *graphql.Object
	WorkspaceObject             *graphql.Object
	TeamObject                  *graphql.Object
	AuditLogObject              *graphql.Object
	SystemUserObject            *graphql.Object
	ProjectDetailsObject        *graphql.Object
	UserDefinedSchemaObject     *graphql.Object
	PluginDetailsObject         *graphql.Object
	ModelTypeObject             *graphql.Object
	CloudFunctionObject         *graphql.Object
	FunctionEnvVariablesObject  *graphql.Object
	FunctionRuntimeConfigObject *graphql.Object
	RoleObject                  *graphql.Object
	//ProjectRoleObject           *graphql.Object
	ConnectionTypeObject      *graphql.Object
	FileDetailsTypeObject     *graphql.Object
	ValidationTypeObject      *graphql.Object
	FieldInfoObject           *graphql.Object
	DocModelObject            *graphql.Object
	UsagesTrackingModelObject *graphql.Object
	ProjectUsageModelObject   *graphql.Object
	WebHookModelObject        *graphql.Object
	MonthlySubscriptionObject *graphql.Object
	InvoiceModelObject        *graphql.Object
}

type SchemaObjects struct {
	systemDriver      interfaces.ApitoSystemDB
	SystemDataloaders *dl.SystemDataloader
	dataLoader        map[string]*dataloader.Loader
	*ObjectModels
}

func GetSchemaObjects(systemDb interfaces.ApitoSystemDB, systemDataloader *dl.SystemDataloader) *SchemaObjects {
	return &SchemaObjects{
		systemDriver:      systemDb,
		SystemDataloaders: systemDataloader,
		dataLoader:        make(map[string]*dataloader.Loader),
		ObjectModels:      &ObjectModels{
			//OrganizationObject: &graphql.Object{},
			//TeamObject:         &graphql.Object{},
			//SystemUserObject:   &graphql.Object{},
		},
	}
}

func (s *SchemaObjects) InitPrivateObjects() *ObjectModels {

	teamsObject := s.GetTeamObject("")
	organizationObject := s.GetOrganizationObject("")

	systemUserObject := s.GetSystemUserObject("")

	funcEnvVarObject := s.GetFunctionEnvVariablesObject()
	pluginDetailsObject := s.GetPluginDetailsObject(funcEnvVarObject)

	validationTypeObject := s.GetValidationTypeObject()

	fieldInfoObject := s.GetFieldInfoObject(validationTypeObject)
	cloudFunctionRequestResponseObject := s.GetCloudFunctionRequestResponseType(fieldInfoObject)

	functionRuntimeConfigObject := s.GetFunctionRuntimeConfigTypeObject()

	cloudFunctionObject := s.GetCloudFunctionObject(cloudFunctionRequestResponseObject, funcEnvVarObject, functionRuntimeConfigObject)

	connectionTypeObject := s.GetConnectionTypeObject()

	modelTypeObject := s.GetModelTypeObject(fieldInfoObject, connectionTypeObject)

	userDefinedSchemaObject := s.GetUserDefinedSchemaObject(modelTypeObject, cloudFunctionObject)

	settingsObject := s.GetSettingsObject()
	apiTokenObject := s.GetProjectTokenObject()
	driverCredObject := s.GetDriverCredentialObject()

	systemMessageObject := s.GetSystemMessageObject("")
	workSpaceObject := s.GetWorkspaceObject()

	projectDetailsObject := s.GetProjectDetailsObject(userDefinedSchemaObject, pluginDetailsObject, settingsObject, apiTokenObject, driverCredObject, systemUserObject, systemMessageObject, workSpaceObject)

	auditLogObject := s.GetAuditLogObject(systemUserObject, projectDetailsObject)

	//projectWithRoleObject := s.GetProjectWithRoleObject()
	roleObject := s.GetRoleObject()

	docModelObject := s.GetDocModelTypeObject()

	metaObject := s.GetMetaObject()

	return &ObjectModels{
		SystemUserObject:           systemUserObject,
		MetaObject:                 metaObject,
		OrganizationObject:         organizationObject,
		SettingsObject:             settingsObject,
		APITokenObject:             apiTokenObject,
		DriverCredentialObject:     driverCredObject,
		SystemMessageObject:        systemMessageObject,
		WorkspaceObject:            workSpaceObject,
		TeamObject:                 teamsObject,
		AuditLogObject:             auditLogObject,
		ProjectDetailsObject:       projectDetailsObject,
		UserDefinedSchemaObject:    userDefinedSchemaObject,
		PluginDetailsObject:        pluginDetailsObject,
		ModelTypeObject:            modelTypeObject,
		CloudFunctionObject:        cloudFunctionObject,
		FunctionEnvVariablesObject: funcEnvVarObject,
		ConnectionTypeObject:       connectionTypeObject,
		ValidationTypeObject:       validationTypeObject,
		FieldInfoObject:            fieldInfoObject,
		RoleObject:                 roleObject,
		//ProjectRoleObject:           projectWithRoleObject,
		FunctionRuntimeConfigObject: functionRuntimeConfigObject,
		FileDetailsTypeObject:       s.GetFileDetailsTypeObject(),
		DocModelObject:              docModelObject,
		WebHookModelObject:          s.GetWebHookModelTypeObject(),
	}
	/*	//var AuditLogType, _ = utility.GetGraphQLObject(protobuff.AuditLogs{})

		var Organization, _ = utility.GetGraphQLObject(protobuff.Organization{})

		var AddOnsType, _ = utility.GetGraphQLObject(protobuff.AddOnsDetails{})

		var APITokenType, _ = utility.GetGraphQLObject(protobuff.APIToken{})

		var DriverCredentialType, _ = utility.GetGraphQLObject(protobuff.DriverCredentials{})

		var SystemMessageType, _ = utility.GetGraphQLObject(protobuff.SystemMessage{})

		var Workspaces, _ = utility.GetGraphQLObject(protobuff.Workspace{})

		var Team, _ = utility.GetGraphQLObject(protobuff.Team{})

		var AuditLogType = graphql.NewObject(graphql.ObjectConfig{
			Name: "AuditLogInfo",
			Fields: graphql.Fields{
				"_key":    &graphql.Field{Type: graphql.String},
				"id":      &graphql.Field{Type: graphql.String},
				"user":    &graphql.Field{Type: SystemUserType},
				"project": &graphql.Field{Type: ProjectDetails},

				"request_payload": &graphql.Field{Type: graphql.String},
				"request_path":    &graphql.Field{Type: graphql.String},
				"response_code":   &graphql.Field{Type: graphql.Int},

				"response_payload":  &graphql.Field{Type: graphql.String},
				"activity":          &graphql.Field{Type: graphql.String},
				"internal_function": &graphql.Field{Type: graphql.String},

				"graphql_operation_name": &graphql.Field{Type: graphql.String},
				"graphql_payload":        &graphql.Field{Type: graphql.String},
				"graphql_variable":       &graphql.Field{Type: graphql.String},

				"internal_error": &graphql.Field{Type: graphql.String},
				"created_at":     &graphql.Field{Type: graphql.String},
			},
		})

		var SystemUserType = graphql.NewObject(graphql.ObjectConfig{
			Name: "SystemUserInfo",
			Fields: graphql.Fields{
				"_key":       &graphql.Field{Type: graphql.String},
				"id":         &graphql.Field{Type: graphql.String},
				"first_name": &graphql.Field{Type: graphql.String},
				"last_name":  &graphql.Field{Type: graphql.String},

				"role":     &graphql.Field{Type: graphql.String},
				"username": &graphql.Field{Type: graphql.String},
				"email":    &graphql.Field{Type: graphql.String},
				//"secret": 		   &graphql.Field{Type: graphql.String},
				"avatar":                     &graphql.Field{Type: graphql.String},
				"current_project_id":         &graphql.Field{Type: graphql.String},
				"project_user":               &graphql.Field{Type: graphql.Boolean},
				"administrative_permissions": &graphql.Field{Type: graphql.NewList(graphql.String)},

				"is_admin":       &graphql.Field{Type: graphql.Boolean},

				"read_only_project":      &graphql.Field{Type: graphql.Boolean},
				"project_limit":          &graphql.Field{Type: graphql.Int},
				"user_subscription_type": &graphql.Field{Type: enums.UserSubscriptionTypeEnums},
				"created_at":             &graphql.Field{Type: graphql.String},
				"updated_at":             &graphql.Field{Type: graphql.String},

				"organizations": &graphql.Field{Type: graphql.NewList(Organization)},
				"teams":         &graphql.Field{Type: graphql.NewList(Team)},
			},
		})

		var ProjectDetails = graphql.NewObject(graphql.ObjectConfig{
			Name: "ProjectModel",
			Fields: graphql.Fields{
				"_key":                &graphql.Field{Type: graphql.String},
				"id":                  &graphql.Field{Type: graphql.String},
				"project_name":        &graphql.Field{Type: graphql.String},
				"project_description": &graphql.Field{Type: graphql.String},
				"schema":              &graphql.Field{Type: UserDefinedSchemaType},
				"created_at":          &graphql.Field{Type: graphql.String},
				"updated_at":          &graphql.Field{Type: graphql.String},
				"expire_at":           &graphql.Field{Type: graphql.String},
				"plugins":             &graphql.Field{Type: graphql.NewList(PluginDetailsType)},
				"addons":              &graphql.Field{Type: AddOnsType},
				"tokens":              &graphql.Field{Type: graphql.NewList(APITokenType)},
				"roles":               &graphql.Field{Type: scaler.ScalarMap},
				"driver":              &graphql.Field{Type: DriverCredentialType},
				//"temp_banned": &graphql.Field{Type: graphql.Boolean},
				"project_template": &graphql.Field{Type: graphql.String},
				"trial_ends":       &graphql.Field{Type: graphql.String},

				"teams":                   &graphql.Field{Type: graphql.NewList(SystemUserType)},
				"system_messages":         &graphql.Field{Type: graphql.NewList(SystemMessageType)},
				"workspaces":              &graphql.Field{Type: graphql.NewList(Workspaces)},
				"default_storage_plugin":  &graphql.Field{Type: graphql.String},
				"default_function_plugin": &graphql.Field{Type: graphql.String},
			},
		})

		var UserDefinedSchemaType = graphql.NewObject(graphql.ObjectConfig{
			Name: "UserDefinedSchema",
			Fields: graphql.Fields{
				"models":    &graphql.Field{Type: graphql.NewList(ModelType)},
				"functions": &graphql.Field{Type: graphql.NewList(CloudFunctionType)},
			},
		})

		var PluginDetailsType = graphql.NewObject(graphql.ObjectConfig{
			Name: "PluginDetailsFields",
			Fields: graphql.Fields{
				"icon":        &graphql.Field{Type: graphql.String},
				"id":          &graphql.Field{Type: graphql.String},
				"title":       &graphql.Field{Type: graphql.String},
				"version":     &graphql.Field{Type: graphql.String},
				"description": &graphql.Field{Type: graphql.String},
				"type":        &graphql.Field{Type: enums.PluginTypeEnums},
				"role":        &graphql.Field{Type: graphql.String},

				/*"credentials": &graphql.Field{
					Name: "PluginCredentialsType",
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "PluginCredentialsObject",
						Fields: graphql.Fields{
							"account_id": &graphql.Field{
								Type: graphql.String,
							},
							"access_key": &graphql.Field{
								Type: graphql.String,
							},
							"secret_key": &graphql.Field{
								Type: graphql.String,
							},
							"api_key": &graphql.Field{
								Type: graphql.String,
							},
							"region": &graphql.Field{
								Type: graphql.String,
							},
							"url": &graphql.Field{
								Type: graphql.String,
							},
							"env": &graphql.Field{
								Type: graphql.String,
							},
						},
					}),
				},

				"exported_variable": &graphql.Field{Type: graphql.String},
				"enable":            &graphql.Field{Type: graphql.Boolean},

				"env_vars":       &graphql.Field{Type: graphql.NewList(FunctionEnvVariablesType)},
				"repository_url": &graphql.Field{Type: graphql.String},
				"branch":         &graphql.Field{Type: graphql.String},
				"author":         &graphql.Field{Type: graphql.String},

				"load_status":     &graphql.Field{Type: enums.PluginLoadTypeEnums},
				"activate_status": &graphql.Field{Type: enums.PluginActivationType},
			},
		})

		//var ModelType, _ = utility.GetGraphQLObject(protobuff.ModelType{})

		var ModelType = graphql.NewObject(graphql.ObjectConfig{
			Name: "ModelType",
			Fields: graphql.Fields{
				"name":             &graphql.Field{Type: graphql.String},
				"fields":           &graphql.Field{Type: graphql.NewList(FieldInfoType)},
				"connections":      &graphql.Field{Type: graphql.NewList(ConnectionType)},
				"hook_ids":         &graphql.Field{Type: graphql.NewList(graphql.String)},
				"locals":           &graphql.Field{Type: graphql.NewList(graphql.String)},
				"repeated_groups":  &graphql.Field{Type: graphql.NewList(graphql.String)},
				"system_generated": &graphql.Field{Type: graphql.Boolean},
				"single_page":      &graphql.Field{Type: graphql.Boolean},
				"single_page_uuid": &graphql.Field{Type: graphql.String},
				"has_connections":  &graphql.Field{Type: graphql.Boolean},
			},
		})

		//var CloudFunctionType, _ = utility.GetGraphQLObject(protobuff.CloudFunction{})

		var CloudFunctionRequestResponseType = graphql.NewObject(graphql.ObjectConfig{
			Name: "CloudFunctionRequestResponse",
			Fields: graphql.Fields{
				"model":  &graphql.Field{Type: graphql.String},
				"params": &graphql.Field{Type: graphql.NewList(FieldInfoType)},
			},
		})

		var FunctionRuntimeConfigType, _ = utility.GetGraphQLObject(protobuff.FunctionRuntimeConfig{})
		var FunctionEnvVariablesType, _ = utility.GetGraphQLObject(protobuff.FunctionEnvVariables{})

		var CloudFunctionType = graphql.NewObject(graphql.ObjectConfig{
			Name: "CloudFunctionType",
			Fields: graphql.Fields{
				"id":          &graphql.Field{Type: graphql.String},
				"name":        &graphql.Field{Type: graphql.String},
				"description": &graphql.Field{Type: graphql.String},

				"function_path":        &graphql.Field{Type: graphql.String},
				"request":              &graphql.Field{Type: CloudFunctionRequestResponseType},
				"response":             &graphql.Field{Type: CloudFunctionRequestResponseType},
				"env_vars":             &graphql.Field{Type: graphql.NewList(FunctionEnvVariablesType)},
				"function_provider_id": &graphql.Field{Type: graphql.String},
				"function_connected":   &graphql.Field{Type: graphql.Boolean},

				"provider_exported_variable": &graphql.Field{Type: graphql.String},
				"function_exported_variable": &graphql.Field{Type: graphql.String},

				"runtime_config": &graphql.Field{Type: FunctionRuntimeConfigType},

				"updated_at": &graphql.Field{Type: graphql.String},
				"created_at": &graphql.Field{Type: graphql.String},
			},
		})

		var RoleType, _ = utility.GetGraphQLObject(protobuff.Role{})

		var ProjectRoleType, _ = utility.GetGraphQLObject(protobuff.Role{})

		var ConnectionType, _ = utility.GetGraphQLObject(protobuff.ConnectionType{})

		var FileDetailsType, _ = utility.GetGraphQLObject(protobuff.FileDetails{})

		var ValidationType, _ = utility.GetGraphQLObject(protobuff.Validation{})

		var FieldInfoType = graphql.NewObject(graphql.ObjectConfig{
			Name: "FieldInfo",
			Fields: graphql.Fields{
				"identifier":       &graphql.Field{Type: graphql.String},
				"description":      &graphql.Field{Type: graphql.String},
				"input_type":       &graphql.Field{Type: graphql.String},
				"field_type":       &graphql.Field{Type: graphql.String},
				"field_sub_type":   &graphql.Field{Type: graphql.String},
				"system_generated": &graphql.Field{Type: graphql.Boolean},
				"parent_field":     &graphql.Field{Type: graphql.String},
				"sub_field_info": &graphql.Field{Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "SubFieldInfo",
					Fields: graphql.Fields{
						"identifier":       &graphql.Field{Type: graphql.String},
						"description":      &graphql.Field{Type: graphql.String},
						"input_type":       &graphql.Field{Type: graphql.String},
						"field_type":       &graphql.Field{Type: graphql.String},
						"field_sub_type":   &graphql.Field{Type: graphql.String},
						"validation":       &graphql.Field{Type: ValidationType},
						"serial":           &graphql.Field{Type: graphql.Int},
						"label":            &graphql.Field{Type: graphql.String},
						"system_generated": &graphql.Field{Type: graphql.Boolean},
						"parent_field":     &graphql.Field{Type: graphql.String},
					},
				}))},
				"validation":                &graphql.Field{Type: ValidationType},
				"serial":                    &graphql.Field{Type: graphql.Int},
				"label":                     &graphql.Field{Type: graphql.String},
				"repeated_group_identifier": &graphql.Field{Type: graphql.String},
				"is_object_field":           &graphql.Field{Type: graphql.Boolean},
			},
		})

		var DocModelType, _ = utility.GetGraphQLObject(shared.DefaultDocumentStructure{})

		var UsagesTrackingModelType, _ = utility.GetGraphQLObject(protobuff.UsagesTracking{})

		var ProjectUsageModelType = graphql.NewObject(graphql.ObjectConfig{
			Name: "ProjectUsage",
			Fields: graphql.Fields{
				"usages": &graphql.Field{Type: UsagesTrackingModelType},
				"limits": &graphql.Field{Type: UsagesTrackingModelType},
			},
		})

		var WebHookModelType, _ = utility.GetGraphQLObject(protobuff.Webhook{})

		var SubscriptionType, _ = utility.GetGraphQLObject(protobuff.MonthlySubscription{})

		var InvoiceModelType, _ = utility.GetGraphQLObject(protobuff.ProjectInvoices{})
	*/
}
