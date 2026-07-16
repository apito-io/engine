package models

import "github.com/apito-io/types/protobuff"

// Runtime identifiers for ApitoFunction.RuntimeConfig.Runtime.
// Canonical discriminator — do not add a duplicate top-level Runtime field.
const (
	FunctionRuntimeHashicorp = "hashicorp"
	FunctionRuntimeDeno      = "deno"
	FunctionRuntimeWASM      = "wasm"
)

// Trigger types for callable / event / scheduled functions.
const (
	FunctionTriggerCallable  = "callable"
	FunctionTriggerEvent     = "event"
	FunctionTriggerWebhook   = "webhook"
	FunctionTriggerScheduled = "scheduled"
)

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

// ApitoFunction is the project-scoped function definition (mutable control-plane row).
// Composite PK matches ModelType: (project_id, name). ID is a stable opaque identifier for APIs.
type ApitoFunction struct {
	ProjectID string `bun:"type:uuid,pk" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	Name      string `bun:"type:text,pk" json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	ID        string `bun:",nullzero" json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`

	Description       string `json:"description,omitempty" firestore:"description,omitempty" bson:"description,omitempty"`
	GraphQLSchemaType string `json:"graphql_schema_type,omitempty" firestore:"graphql_schema_type,omitempty" bson:"graphql_schema_type,omitempty"` // query or mutation

	EnvVars      []*protobuff.EnvVariable          `bun:"type:json,nullzero" json:"env_vars,omitempty" firestore:"env_vars,omitempty" bson:"env_vars,omitempty"`
	FunctionPath string                            `json:"function_path,omitempty" firestore:"function_path,omitempty" bson:"function_path,omitempty"`
	Request      *ApitoFunctionRequestResponseType `bun:"type:json,nullzero" json:"request,omitempty" firestore:"request,omitempty" bson:"request,omitempty"`
	Response     *ApitoFunctionRequestResponseType `bun:"type:json,nullzero" json:"response,omitempty" firestore:"response,omitempty" bson:"response,omitempty"`

	FunctionProviderID       string `json:"function_provider_id,omitempty" firestore:"function_provider_id,omitempty" bson:"function_provider_id,omitempty"`
	ProviderExportedVariable string `json:"provider_exported_variable,omitempty" firestore:"provider_exported_variable,omitempty" bson:"provider_exported_variable,omitempty"`
	FunctionExportedVariable string `json:"function_exported_variable,omitempty" firestore:"function_exported_variable,omitempty" bson:"function_exported_variable,omitempty"`

	RuntimeConfig *ApitoFunctionRuntimeConfig `bun:"type:json,nullzero" json:"runtime_config,omitempty" firestore:"runtime_config,omitempty" bson:"runtime_config,omitempty"`

	// TriggerType: callable (default), event, webhook, scheduled.
	TriggerType string `bun:",nullzero" json:"trigger_type,omitempty" firestore:"trigger_type,omitempty" bson:"trigger_type,omitempty"`

	// ActiveRevisionID points at the currently deployed FunctionRevision.
	ActiveRevisionID string `bun:",nullzero" json:"active_revision_id,omitempty" firestore:"active_revision_id,omitempty" bson:"active_revision_id,omitempty"`

	// Capabilities is a JSON list of capability strings (data.read:model, http, …).
	Capabilities []string `bun:"type:json,nullzero" json:"capabilities,omitempty" firestore:"capabilities,omitempty" bson:"capabilities,omitempty"`

	// Source holds inline Deno/TS/JS draft source (not an active deployment).
	Source string `bun:"type:text,nullzero" json:"source,omitempty" firestore:"source,omitempty" bson:"source,omitempty"`

	UpdatedAt string `json:"updated_at,omitempty" firestore:"updated_at,omitempty" bson:"updated_at,omitempty"`
	CreatedAt string `json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`

	Language  string `json:"language,omitempty" firestore:"language,omitempty" bson:"language,omitempty"`
	BinaryURL string `json:"binary_url,omitempty" firestore:"binary_url,omitempty" bson:"binary_url,omitempty"`

	RestAPISecretURLKey string `json:"rest_api_secret_url_key,omitempty" firestore:"rest_api_secret_url_key,omitempty" bson:"rest_api_secret_url_key,omitempty"`
}

// EffectiveRuntime returns the canonical runtime discriminator.
func (f *ApitoFunction) EffectiveRuntime() string {
	if f == nil {
		return ""
	}
	if f.RuntimeConfig != nil && f.RuntimeConfig.Runtime != "" {
		return f.RuntimeConfig.Runtime
	}
	if f.FunctionProviderID != "" && len(f.FunctionProviderID) >= 3 && f.FunctionProviderID[:3] == "hc-" {
		return FunctionRuntimeHashicorp
	}
	return ""
}

// IsApitoFunctionsRuntime reports whether this function uses the Apito Functions platform (not HashiCorp).
func (f *ApitoFunction) IsApitoFunctionsRuntime() bool {
	r := f.EffectiveRuntime()
	return r == FunctionRuntimeDeno || r == FunctionRuntimeWASM
}

// IsValidFunctionRuntime reports whether runtime is a known platform runtime.
func IsValidFunctionRuntime(runtime string) bool {
	switch runtime {
	case FunctionRuntimeDeno, FunctionRuntimeWASM, FunctionRuntimeHashicorp, "":
		return true
	default:
		return false
	}
}
