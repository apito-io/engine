package relationmigrate

import "github.com/apito-io/engine/models"

// ConflictPolicyAbort is the only version-one conflict policy.
// The engine never keeps an arbitrary link when several cannot fit.
const ConflictPolicyAbort = "abort"

const (
	ShapeOneToOne   = "1:1"
	ShapeOneToMany  = "1:N"
	ShapeManyToOne  = "N:1"
	ShapeManyToMany = "N:N"
	ShapeUnknown    = "unknown"
)

const (
	KindNoOp           = "no_op"
	KindWidenToNN      = "widen_to_nn"
	KindNarrowFromNN   = "narrow_from_nn"
	KindNarrowTo11     = "narrow_to_11"
	KindOwnershipFlip  = "ownership_flip"
	KindOneToOneToFK   = "one_to_one_to_fk"
	KindUnknown        = "unknown"
)

const (
	SafetyLosslessWidening       = "lossless_widening"
	SafetyConditionalNarrowing   = "conditional_narrowing"
	SafetyOwnershipFlip          = "ownership_flip"
	SafetyBlocked                = "blocked"
	SafetyNoOp                   = "no_op"
	SafetyUnsupported            = "unsupported_driver"
)

const (
	RevNow            = "reversible_now"
	RevConditional    = "conditionally_reversible"
	RevNotBySchema    = "not_reversible_by_schema"
	RevBlocked        = "blocked"
)

const (
	CapabilitySQL          = "sql"
	CapabilityMetadataOnly = "metadata_only"
	CapabilityUnsupported  = "unsupported_driver"
)

// Request is one directed cardinality change for a single edge.
type Request struct {
	ProjectID            string `json:"project_id,omitempty"`
	SchemaVersion        int    `json:"schema_version,omitempty"`
	FromModel            string `json:"from_model"`
	ToModel              string `json:"to_model"`
	KnownAs              string `json:"known_as,omitempty"`
	FromRelation         string `json:"from_relation"`
	ToRelation           string `json:"to_relation"`
	TargetFromRelation   string `json:"target_from_relation"`
	TargetToRelation     string `json:"target_to_relation"`
	ConflictPolicy       string `json:"conflict_policy"`
	AffectedDatabases    int    `json:"affected_databases,omitempty"`
}

// Counts are decisive preflight numbers for one database.
type Counts struct {
	SourcePairs          int `json:"source_pairs"`
	ValidPairs           int `json:"valid_pairs"`
	Dangling             int `json:"dangling"`
	Conflicts            int `json:"conflicts"`
	Contradictory        int `json:"contradictory"`
	ProjectedDestination int `json:"projected_destination"`
}

// DatabaseSnapshot is one physical database's preflight. ScopeKey is stable
// for that database (file path or host/database/schema), never a secret.
type DatabaseSnapshot struct {
	ScopeKey   string `json:"scope_key"`
	Capability string `json:"capability,omitempty"`
	Counts     Counts `json:"counts"`
	Blocked    bool   `json:"blocked"`
	Error      string `json:"error,omitempty"`
}

// Preview is the shared Console and MCP contract.
type Preview struct {
	PlanHash                 string             `json:"plan_hash"`
	TransitionKind           string             `json:"transition_kind"`
	SafetyClass              string             `json:"safety_class"`
	Reversibility            string             `json:"reversibility"`
	InversePassesNow         bool               `json:"inverse_passes_now"`
	SourceShape              string             `json:"source_shape"`
	TargetShape              string             `json:"target_shape"`
	SourceStorage            string             `json:"source_storage"`
	TargetStorage            string             `json:"target_storage"`
	Steps                    []string           `json:"steps,omitempty"`
	SourcePairs              int                `json:"source_pairs"`
	ValidPairs               int                `json:"valid_pairs"`
	Dangling                 int                `json:"dangling"`
	Conflicts                int                `json:"conflicts"`
	Contradictory            int                `json:"contradictory"`
	ProjectedDestination     int                `json:"projected_destination"`
	AffectedDatabases        int                `json:"affected_databases"`
	Warnings                 []string           `json:"warnings,omitempty"`
	Blockers                 []string           `json:"blockers,omitempty"`
	Disclaimers              []string           `json:"disclaimers,omitempty"`
	ConflictReport           []string           `json:"conflict_report,omitempty"`
	BackupRequired           bool               `json:"backup_required"`
	CodegenImpact            bool               `json:"codegen_impact"`
	RequiresTypedConfirmation bool              `json:"requires_typed_confirmation"`
	RequiredAcknowledgement  string             `json:"required_acknowledgement"`
	Blocked                  bool               `json:"blocked"`
	ConflictPolicy           string             `json:"conflict_policy"`
	ExecutionActions         []string           `json:"execution_actions,omitempty"`
	Capability               string             `json:"capability,omitempty"`
	Databases                []DatabaseSnapshot `json:"databases,omitempty"`
}

// Receipt is persisted per database after a physical attempt.
type Receipt struct {
	ScopeKey             string `json:"scope_key,omitempty"`
	Status               string `json:"status"`
	SourcePairs          int    `json:"source_pairs"`
	CopiedPairs          int    `json:"copied_pairs"`
	DestinationPairs     int    `json:"destination_pairs"`
	Dangling             int    `json:"dangling"`
	Conflicts            int    `json:"conflicts"`
	Dropped              []string `json:"dropped,omitempty"`
	Error                string `json:"error,omitempty"`
	Idempotent           bool   `json:"idempotent,omitempty"`
}

// Pair is one link in canonical model order (lower physical name first).
type Pair struct {
	Left  string
	Right string
}

func (r *Request) Normalized() Request {
	out := Request{}
	if r != nil {
		out = *r
	}
	if out.ConflictPolicy == "" {
		out.ConflictPolicy = ConflictPolicyAbort
	}
	out.FromRelation = normRel(out.FromRelation)
	out.ToRelation = normRel(out.ToRelation)
	out.TargetFromRelation = normRel(out.TargetFromRelation)
	out.TargetToRelation = normRel(out.TargetToRelation)
	return out
}

// Connections builds the forward/backward connection pair the SQL drivers already use.
func (r Request) CurrentConnections() (from, to *models.ConnectionType) {
	return &models.ConnectionType{Model: r.ToModel, Relation: r.FromRelation, Type: "forward", KnownAs: r.KnownAs},
		&models.ConnectionType{Model: r.FromModel, Relation: r.ToRelation, Type: "backward", KnownAs: r.KnownAs}
}

func (r Request) TargetConnections() (from, to *models.ConnectionType) {
	return &models.ConnectionType{Model: r.ToModel, Relation: r.TargetFromRelation, Type: "forward", KnownAs: r.KnownAs},
		&models.ConnectionType{Model: r.FromModel, Relation: r.TargetToRelation, Type: "backward", KnownAs: r.KnownAs}
}
