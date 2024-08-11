package objects

import (
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-inc/graphql"
)

func (s *SchemaObjects) GetSystemUserObject() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "SystemUserInfo",
		Fields: graphql.Fields{
			"_key":               &graphql.Field{Type: graphql.String},
			"id":                 &graphql.Field{Type: graphql.String},
			"first_name":         &graphql.Field{Type: graphql.String},
			"last_name":          &graphql.Field{Type: graphql.String},
			"username":           &graphql.Field{Type: graphql.String},
			"email":              &graphql.Field{Type: graphql.String},
			"avatar":             &graphql.Field{Type: graphql.String},
			"current_project_id": &graphql.Field{Type: graphql.String},
			"project_user":       &graphql.Field{Type: graphql.Boolean},
			"created_at":         &graphql.Field{Type: graphql.String},
			"updated_at":         &graphql.Field{Type: graphql.String},
		},
	})
}

// Methods for other types as defined earlier:
func (s *SchemaObjects) GetProjectDetailsObject(userDefinedSchemaObj, pluginDetailsObj, apiTokenObj, driverCredObj, systemUserObj, systemMsgObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "ProjectModel",
		Fields: graphql.Fields{
			"_key":                    &graphql.Field{Type: graphql.String},
			"id":                      &graphql.Field{Type: graphql.String},
			"name":                    &graphql.Field{Type: graphql.String},
			"description":             &graphql.Field{Type: graphql.String},
			"schema":                  &graphql.Field{Type: userDefinedSchemaObj},
			"created_at":              &graphql.Field{Type: graphql.String},
			"updated_at":              &graphql.Field{Type: graphql.String},
			"expire_at":               &graphql.Field{Type: graphql.String},
			"plugins":                 &graphql.Field{Type: graphql.NewList(pluginDetailsObj)},
			"tokens":                  &graphql.Field{Type: graphql.NewList(apiTokenObj)},
			"driver":                  &graphql.Field{Type: driverCredObj},
			"project_template":        &graphql.Field{Type: graphql.String},
			"system_messages":         &graphql.Field{Type: graphql.NewList(systemMsgObj)},
			"default_storage_plugin":  &graphql.Field{Type: graphql.String},
			"default_function_plugin": &graphql.Field{Type: graphql.String},
			"locals":                  &graphql.Field{Type: graphql.NewList(graphql.String)},
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
			"request":                    &graphql.Field{Type: cloudFunctionRequestResponseObj},
			"response":                   &graphql.Field{Type: cloudFunctionRequestResponseObj},
			"env_vars":                   &graphql.Field{Type: graphql.NewList(funcEnvVarObj)},
			"function_provider_id":       &graphql.Field{Type: graphql.String},
			"function_connected":         &graphql.Field{Type: graphql.Boolean},
			"provider_exported_variable": &graphql.Field{Type: graphql.String},
			"function_exported_variable": &graphql.Field{Type: graphql.String},
			"runtime_config":             &graphql.Field{Type: functionRuntimeConfigObj},
			"updated_at":                 &graphql.Field{Type: graphql.String},
			"created_at":                 &graphql.Field{Type: graphql.String},
		},
	})
}

func (s *SchemaObjects) GetCloudFunctionRequestResponseType(fieldInfoObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "CloudFunctionRequestResponse",
		Fields: graphql.Fields{
			"model":  &graphql.Field{Type: graphql.String},
			"params": &graphql.Field{Type: graphql.NewList(fieldInfoObj)},
		},
	})
}

func (s *SchemaObjects) GetFunctionRuntimeConfigTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(protobuff.FunctionRuntimeConfig{})
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
			"enable":            &graphql.Field{Type: graphql.Boolean},
			"exported_variable": &graphql.Field{Type: graphql.String},
			"env_vars":          &graphql.Field{Type: graphql.NewList(funcEnvVarObject)},
			"repository_url":    &graphql.Field{Type: graphql.String},
			"branch":            &graphql.Field{Type: graphql.String},
			"author":            &graphql.Field{Type: graphql.String},
			"load_status":       &graphql.Field{Type: enums.PluginLoadTypeEnums},  // Adjust the type accordingly
			"activate_status":   &graphql.Field{Type: enums.PluginActivationType}, // Adjust the type accordingly
		},
	})
}

func (s *SchemaObjects) GetFieldInfoObject(validationTypeObj *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
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
					"validation":       &graphql.Field{Type: validationTypeObj},
					"serial":           &graphql.Field{Type: graphql.Int},
					"label":            &graphql.Field{Type: graphql.String},
					"system_generated": &graphql.Field{Type: graphql.Boolean},
					"parent_field":     &graphql.Field{Type: graphql.String},
				},
			}))},
			"validation":                &graphql.Field{Type: validationTypeObj},
			"serial":                    &graphql.Field{Type: graphql.Int},
			"label":                     &graphql.Field{Type: graphql.String},
			"repeated_group_identifier": &graphql.Field{Type: graphql.String},
			"is_object_field":           &graphql.Field{Type: graphql.Boolean},
		},
	})
}

func (s *SchemaObjects) GetConnectionTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(protobuff.ConnectionType{})
	return obj
}

func (s *SchemaObjects) GetFileDetailsTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(protobuff.FileDetails{})
	return obj
}

func (s *SchemaObjects) GetDocModelTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(shared.DefaultDocumentStructure{})
	return obj
}

func (s *SchemaObjects) GetValidationTypeObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(protobuff.Validation{})
	return obj
}

func (s *SchemaObjects) GetFunctionEnvVariablesObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(protobuff.FunctionEnvVariables{})
	return obj
}

// GetAPITokenObject retrieves the GraphQL object for APIToken
func (s *SchemaObjects) GetAPITokenObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(protobuff.APIToken{})
	return obj
}

// GetDriverCredentialObject retrieves the GraphQL object for DriverCredentials
func (s *SchemaObjects) GetDriverCredentialObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(protobuff.DriverCredentials{})
	return obj
}

// GetSystemMessageObject retrieves the GraphQL object for SystemMessage
func (s *SchemaObjects) GetSystemMessageObject() *graphql.Object {
	obj, _ := utility.GetGraphQLObject(protobuff.SystemMessage{})
	return obj
}
