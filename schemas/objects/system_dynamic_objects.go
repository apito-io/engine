package objects

import (
	"github.com/apito-io/buffers/interfaces"
	dl "github.com/apito-io/engine/resolver/dataloader"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-inc/graphql"
)

type ObjectModels struct {
	APITokenObject              *graphql.Object
	DriverCredentialObject      *graphql.Object
	SystemMessageObject         *graphql.Object
	SystemUserObject            *graphql.Object
	ProjectDetailsObject        *graphql.Object
	UserDefinedSchemaObject     *graphql.Object
	PluginDetailsObject         *graphql.Object
	ModelTypeObject             *graphql.Object
	CloudFunctionObject         *graphql.Object
	FunctionEnvVariablesObject  *graphql.Object
	FunctionRuntimeConfigObject *graphql.Object
	ConnectionTypeObject        *graphql.Object
	FileDetailsTypeObject       *graphql.Object
	ValidationTypeObject        *graphql.Object
	FieldInfoObject             *graphql.Object
	DocModelObject              *graphql.Object
}

type SchemaObjects struct {
	systemDriver      interfaces.SystemDBInterface
	SystemDataloaders *dl.SystemDataloader
	dataLoader        map[string]*dataloader.Loader
	*ObjectModels
}

func GetSchemaObjects(systemDb interfaces.SystemDBInterface, systemDataloader *dl.SystemDataloader) *SchemaObjects {
	return &SchemaObjects{
		systemDriver:      systemDb,
		SystemDataloaders: systemDataloader,
		dataLoader:        make(map[string]*dataloader.Loader),
	}
}

func (s *SchemaObjects) InitPrivateObjects() *ObjectModels {

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

	systemUserObject := s.GetSystemUserObject()

	apiTokenObject := s.GetAPITokenObject()
	driverCredObject := s.GetDriverCredentialObject()

	systemMessageObject := s.GetSystemMessageObject()

	projectDetailsObject := s.GetProjectDetailsObject(userDefinedSchemaObject, pluginDetailsObject, apiTokenObject, driverCredObject, systemUserObject, systemMessageObject)

	//projectWithRoleObject := s.GetProjectWithRoleObject()

	return &ObjectModels{
		APITokenObject:             apiTokenObject,
		DriverCredentialObject:     driverCredObject,
		SystemMessageObject:        systemMessageObject,
		SystemUserObject:           systemUserObject,
		ProjectDetailsObject:       projectDetailsObject,
		UserDefinedSchemaObject:    userDefinedSchemaObject,
		PluginDetailsObject:        pluginDetailsObject,
		ModelTypeObject:            modelTypeObject,
		CloudFunctionObject:        cloudFunctionObject,
		FunctionEnvVariablesObject: funcEnvVarObject,
		ConnectionTypeObject:       connectionTypeObject,
		ValidationTypeObject:       validationTypeObject,
		FieldInfoObject:            fieldInfoObject,
		//ProjectRoleObject:           projectWithRoleObject,
		FunctionRuntimeConfigObject: functionRuntimeConfigObject,
		FileDetailsTypeObject:       s.GetFileDetailsTypeObject(),
		DocModelObject:              s.GetDocModelTypeObject(),
	}
}
