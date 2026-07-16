package models

import "github.com/uptrace/bun"

// FunctionRevision is an immutable source/artifact snapshot for a function.
type FunctionRevision struct {
	bun.BaseModel `bun:"table:function_revisions,alias:fnrev"`

	ID        string `bun:"type:text,pk" json:"id,omitempty"`
	ProjectID string `bun:"type:uuid,notnull" json:"project_id,omitempty"`
	Name      string `bun:"type:text,notnull" json:"name,omitempty"` // function name
	Revision  int64  `bun:",notnull" json:"revision,omitempty"`

	Runtime      string `json:"runtime,omitempty"`
	Language     string `json:"language,omitempty"`
	Source       string `bun:"type:text,nullzero" json:"source,omitempty"`
	ArtifactKey  string `bun:",nullzero" json:"artifact_key,omitempty"`
	ArtifactHash string `bun:",nullzero" json:"artifact_hash,omitempty"`
	ABIVersion   string `bun:",nullzero" json:"abi_version,omitempty"`
	SDKVersion   string `bun:",nullzero" json:"sdk_version,omitempty"`
	RuntimeVer   string `bun:",nullzero" json:"runtime_version,omitempty"`

	Capabilities []string `bun:"type:json,nullzero" json:"capabilities,omitempty"`
	CreatedBy    string   `bun:",nullzero" json:"created_by,omitempty"`
	CreatedAt    string   `json:"created_at,omitempty"`
}

// FunctionBuild tracks a build of a revision into an immutable artifact.
type FunctionBuild struct {
	bun.BaseModel `bun:"table:function_builds,alias:fnbuild"`

	ID         string `bun:"type:text,pk" json:"id,omitempty"`
	ProjectID  string `bun:"type:uuid,notnull" json:"project_id,omitempty"`
	Name       string `bun:"type:text,notnull" json:"name,omitempty"`
	RevisionID string `bun:"type:text,notnull" json:"revision_id,omitempty"`

	Status        string `json:"status,omitempty"` // queued, building, succeeded, failed
	Logs          string `bun:"type:text,nullzero" json:"logs,omitempty"`
	ArtifactKey   string `bun:",nullzero" json:"artifact_key,omitempty"`
	ArtifactHash  string `bun:",nullzero" json:"artifact_hash,omitempty"`
	ErrorMessage  string `bun:",nullzero" json:"error_message,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	CompletedAt   string `bun:",nullzero" json:"completed_at,omitempty"`
}

// FunctionDeployment activates a successful revision.
type FunctionDeployment struct {
	bun.BaseModel `bun:"table:function_deployments,alias:fndeploy"`

	ID         string `bun:"type:text,pk" json:"id,omitempty"`
	ProjectID  string `bun:"type:uuid,notnull" json:"project_id,omitempty"`
	Name       string `bun:"type:text,notnull" json:"name,omitempty"`
	RevisionID string `bun:"type:text,notnull" json:"revision_id,omitempty"`
	BuildID    string `bun:",nullzero" json:"build_id,omitempty"`

	Environment string `bun:",nullzero" json:"environment,omitempty"` // production, staging, …
	Status      string `json:"status,omitempty"`                       // active, rolled_back, superseded
	DeployedBy  string `bun:",nullzero" json:"deployed_by,omitempty"`
	RollbackOf  string `bun:",nullzero" json:"rollback_of,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// FunctionInvocation records a single execution (callable or event).
type FunctionInvocation struct {
	bun.BaseModel `bun:"table:function_invocations,alias:fninv"`

	ID             string `bun:"type:text,pk" json:"id,omitempty"`
	ProjectID      string `bun:"type:uuid,notnull" json:"project_id,omitempty"`
	Name           string `bun:"type:text,notnull" json:"name,omitempty"`
	RevisionID     string `bun:",nullzero" json:"revision_id,omitempty"`
	IdempotencyKey string `bun:",nullzero" json:"idempotency_key,omitempty"`

	TenantScope string `bun:",nullzero" json:"tenant_scope,omitempty"`
	Principal   string `bun:",nullzero" json:"principal,omitempty"` // caller or service
	Status      string `json:"status,omitempty"`                   // accepted, running, committed, failed, dead_lettered

	DurationMs   int64  `json:"duration_ms,omitempty"`
	ErrorClass   string `bun:",nullzero" json:"error_class,omitempty"`
	ResultDigest string `bun:",nullzero" json:"result_digest,omitempty"`
	Logs         string `bun:"type:text,nullzero" json:"logs,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	CompletedAt  string `bun:",nullzero" json:"completed_at,omitempty"`
}

// FunctionIdempotencyRecord is the durable ledger for atomic batches / event retries.
type FunctionIdempotencyRecord struct {
	bun.BaseModel `bun:"table:function_idempotency,alias:fnidem"`

	ProjectID      string `bun:"type:uuid,pk" json:"project_id,omitempty"`
	TenantScope    string `bun:"type:text,pk" json:"tenant_scope,omitempty"`
	FunctionName   string `bun:"type:text,pk" json:"function_name,omitempty"`
	IdempotencyKey string `bun:"type:text,pk" json:"idempotency_key,omitempty"`

	RequestHash  string `json:"request_hash,omitempty"`
	Status       string `json:"status,omitempty"` // running, committed, failed
	ResultJSON   string `bun:"type:text,nullzero" json:"result_json,omitempty"`
	ResultDigest string `bun:",nullzero" json:"result_digest,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}
