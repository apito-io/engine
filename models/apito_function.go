package models

import "github.com/apito-io/types/protobuff"

type ApitoFunctionRuntimeConfig struct {
	Runtime string `json:"runtime,omitempty" firestore:"runtime,omitempty" bson:"runtime,omitempty"`
	Memory  int64  `json:"memory,omitempty" firestore:"memory,omitempty" bson:"memory,omitempty"`
	Handler string `json:"handler,omitempty" firestore:"handler,omitempty" bson:"handler,omitempty"`
	TimeOut int64  `json:"time_out,omitempty" firestore:"time_out,omitempty" bson:"time_out,omitempty"`
}

type ApitoFunctionRequestResponseType struct {
	Model           string      `json:"model,omitempty" firestore:"model,omitempty" bson:"model,omitempty"`
	Params          interface{} `json:"params,omitempty" firestore:"params,omitempty" bson:"params,omitempty"`
	IsArray         bool        `json:"is_array,omitempty" firestore:"is_array,omitempty" bson:"is_array,omitempty"`
	OptionalPayload bool        `json:"optional_payload,omitempty" firestore:"optional_payload,omitempty" bson:"optional_payload,omitempty"`
}

type ApitoFunction struct {
	ProjectID          string                            `bun:"type:uuid,pk" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	ID                 string                            `json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	Name               string                            `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Description        string                            `json:"description,omitempty" firestore:"description,omitempty" bson:"description,omitempty"`
	GraphQLSchemaType  string                            `json:"graphql_schema_type,omitempty" firestore:"graphql_schema_type,omitempty" bson:"graphql_schema_type,omitempty"` // query or mutation
	EnvVars            []*protobuff.EnvVariable          `json:"env_vars,omitempty" firestore:"env_vars,omitempty" bson:"env_vars,omitempty"`
	FunctionPath       string                            `json:"function_path,omitempty" firestore:"function_path,omitempty" bson:"function_path,omitempty"`
	Request            *ApitoFunctionRequestResponseType `json:"request,omitempty" firestore:"request,omitempty" bson:"request,omitempty"`
	Response           *ApitoFunctionRequestResponseType `json:"response,omitempty" firestore:"response,omitempty" bson:"response,omitempty"`
	FunctionProviderID string                            `json:"function_provider_id,omitempty" firestore:"function_provider_id,omitempty" bson:"function_provider_id,omitempty"`
	//FunctionConnected        bool                              `json:"function_connected,omitempty" firestore:"function_connected,omitempty"`
	ProviderExportedVariable string                      `json:"provider_exported_variable,omitempty" firestore:"provider_exported_variable,omitempty" bson:"provider_exported_variable,omitempty"`
	FunctionExportedVariable string                      `json:"function_exported_variable,omitempty" firestore:"function_exported_variable,omitempty" bson:"function_exported_variable,omitempty"`
	RuntimeConfig            *ApitoFunctionRuntimeConfig `json:"runtime_config,omitempty" firestore:"runtime_config,omitempty" bson:"runtime_config,omitempty"`

	UpdatedAt string `json:"updated_at,omitempty" firestore:"updated_at,omitempty" bson:"updated_at,omitempty"`
	CreatedAt string `json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"` // @got

	Language  string `json:"language,omitempty" firestore:"language,omitempty" bson:"language,omitempty"`       // golang, js, python supported
	BinaryURL string `json:"binary_url,omitempty" firestore:"binary_url,omitempty" bson:"binary_url,omitempty"` // s3 upload path

	RestAPISecretURLKey string `json:"rest_api_secret_url_key,omitempty" firestore:"rest_api_secret_url_key,omitempty" bson:"rest_api_secret_url_key,omitempty"` // used to access this function directly via url
}
