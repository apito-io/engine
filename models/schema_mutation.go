package models

import "context"

// SchemaMutationHook runs before a schema-changing system mutation applies physical DDL.
// Return handled=true to stop open-core; handled=false delegates to the default orchestration path.
type SchemaMutationHook func(ctx context.Context, req *SchemaMutationRequest) (*SchemaMutationResult, bool, error)

// SchemaMutationRequest carries neutral metadata for staging or bypass decisions.
type SchemaMutationRequest struct {
	OperationType string
	Project       *Project
	Args          map[string]interface{}
	UserID        string
	Role          string
}

// SchemaMutationResult is returned when the hook stages a change instead of applying immediately.
type SchemaMutationResult struct {
	ProjectID    string
	ChangesetID  string
	DraftVersion int
	HasDraft     bool
	Message      string
	Response     interface{}
}
