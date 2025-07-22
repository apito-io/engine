package plugin

import (
	"github.com/apito-io/types/protobuff"
)

// ConvertEnvVariablesFromProto converts protobuf environment variables to extensions environment variables
func ConvertEnvVariablesFromProto(protoEnvVars []*protobuff.EnvVariable) []*protobuff.EnvVariable {
	if protoEnvVars == nil {
		return nil
	}

	envVars := make([]*protobuff.EnvVariable, len(protoEnvVars))
	for i, protoVar := range protoEnvVars {
		envVars[i] = &protobuff.EnvVariable{
			Key:   protoVar.Key,
			Value: protoVar.Value,
		}
	}
	return envVars
}

// ConvertGraphQLSchemaToProto converts protobuf GraphQL schema to protobuf format (passthrough)
func ConvertGraphQLSchemaToProto(schema *protobuff.ThirdPartyGraphQLSchemas) (*protobuff.ThirdPartyGraphQLSchemas, error) {
	if schema == nil {
		return &protobuff.ThirdPartyGraphQLSchemas{}, nil
	}

	// Since input and output are the same type, just return a copy
	protoSchema := &protobuff.ThirdPartyGraphQLSchemas{
		Queries:   schema.Queries,
		Mutations: schema.Mutations,
	}

	return protoSchema, nil
}

// ConvertRestApisToProto converts protobuf REST APIs to protobuf format (passthrough)
func ConvertRestApisToProto(apis []*protobuff.ThirdPartyRESTApi) []*protobuff.ThirdPartyRESTApi {
	if apis == nil {
		return nil
	}

	protoApis := make([]*protobuff.ThirdPartyRESTApi, len(apis))
	for i, api := range apis {
		protoApis[i] = &protobuff.ThirdPartyRESTApi{
			Method: api.Method,
			Path:   api.Path,
		}
	}
	return protoApis
}

// ConvertProtoRestApisToLocal converts protobuf REST APIs to local format (passthrough)
func ConvertProtoRestApisToLocal(protoApis []*protobuff.ThirdPartyRESTApi) []*protobuff.ThirdPartyRESTApi {
	if protoApis == nil {
		return nil
	}

	apis := make([]*protobuff.ThirdPartyRESTApi, len(protoApis))
	for i, protoApi := range protoApis {
		apis[i] = &protobuff.ThirdPartyRESTApi{
			Method: protoApi.Method,
			Path:   protoApi.Path,
		}
	}
	return apis
}
