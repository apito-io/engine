package relationmigrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/apito-io/engine/models"
)

func normRel(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ShapeOf classifies one directed endpoint pair.
// 1:N means from has_many and to has_one, so the FK lives on to.
func ShapeOf(fromRel, toRel string) string {
	f, t := normRel(fromRel), normRel(toRel)
	switch {
	case f == "has_one" && t == "has_one":
		return ShapeOneToOne
	case f == "has_many" && t == "has_one":
		return ShapeOneToMany
	case f == "has_one" && t == "has_many":
		return ShapeManyToOne
	case f == "has_many" && t == "has_many":
		return ShapeManyToMany
	default:
		return ShapeUnknown
	}
}

// StorageChanging reports whether the two shapes use different physical link storage.
func StorageChanging(fromRel, toRel, targetFrom, targetTo string) bool {
	a, b := ShapeOf(fromRel, toRel), ShapeOf(targetFrom, targetTo)
	return a != b && a != ShapeUnknown && b != ShapeUnknown
}

type classification struct {
	Kind                 string
	BaseSafety           string
	Steps                []string
	BlockOnDegree        bool
	BlockOnContradictory bool
	WarnOnContradictory  bool
	DropsStorage         bool
}

func classifyShapes(fromShape, toShape string) classification {
	if fromShape == toShape && fromShape != ShapeUnknown {
		return classification{Kind: KindNoOp, BaseSafety: SafetyNoOp, Steps: []string{"no physical change"}}
	}
	switch {
	case (fromShape == ShapeOneToMany || fromShape == ShapeManyToOne || fromShape == ShapeOneToOne) && toShape == ShapeManyToMany:
		return classification{
			Kind: KindWidenToNN, BaseSafety: SafetyLosslessWidening, DropsStorage: true, WarnOnContradictory: fromShape == ShapeOneToOne,
			Steps: []string{
				"create the pivot table",
				"copy valid pairs",
				"verify the pair set",
				"drop obsolete foreign keys and unique indexes",
			},
		}
	case fromShape == ShapeManyToMany && (toShape == ShapeOneToMany || toShape == ShapeManyToOne):
		return classification{
			Kind: KindNarrowFromNN, BaseSafety: SafetyConditionalNarrowing, BlockOnDegree: true, DropsStorage: true,
			Steps: []string{
				"require degree <= 1 on the endpoint that will hold the foreign key",
				"create and populate the foreign key",
				"verify every pivot pair is represented",
				"drop the pivot",
			},
		}
	case (fromShape == ShapeOneToMany || fromShape == ShapeManyToOne || fromShape == ShapeManyToMany) && toShape == ShapeOneToOne:
		c := classification{
			Kind: KindNarrowTo11, BaseSafety: SafetyConditionalNarrowing, BlockOnDegree: true, BlockOnContradictory: true, DropsStorage: true,
			Steps: []string{
				"require degree <= 1 on both endpoints",
				"populate both foreign keys consistently",
				"add unique indexes",
				"drop obsolete storage after verification",
			},
		}
		if fromShape == ShapeManyToMany {
			c.Kind = KindNarrowTo11
		}
		return c
	case (fromShape == ShapeOneToMany && toShape == ShapeManyToOne) || (fromShape == ShapeManyToOne && toShape == ShapeOneToMany):
		return classification{
			Kind: KindOwnershipFlip, BaseSafety: SafetyOwnershipFlip, BlockOnDegree: true, DropsStorage: true,
			Steps: []string{
				"preflight the target holder's degree",
				"create and populate the new foreign key",
				"verify pair equality",
				"drop the old foreign key",
			},
		}
	case fromShape == ShapeOneToOne && (toShape == ShapeOneToMany || toShape == ShapeManyToOne):
		return classification{
			Kind: KindOneToOneToFK, BaseSafety: SafetyOwnershipFlip, BlockOnDegree: true, BlockOnContradictory: true, DropsStorage: true,
			Steps: []string{
				"merge consistent dual foreign-key pairs into the target holder",
				"block when the dual representation conflicts",
				"drop the obsolete foreign key and unique indexes",
			},
		}
	default:
		return classification{
			Kind: KindUnknown, BaseSafety: SafetyBlocked,
			Steps: []string{"unsupported cardinality pair"},
		}
	}
}

// ScopeKey identifies a database without including secrets.
func ScopeKey(cred *models.DriverCredentials) string {
	if cred == nil {
		return "unknown"
	}
	eng := strings.ToLower(strings.TrimSpace(cred.Engine))
	if file := strings.TrimSpace(cred.File); file != "" {
		return eng + ":" + file
	}
	return fmt.Sprintf("%s:%s:%s/%s/%s", eng, strings.TrimSpace(cred.Host), strings.TrimSpace(cred.Port), strings.TrimSpace(cred.Database), strings.TrimSpace(cred.Schema))
}

// Acknowledgement is the exact string Console and MCP must send back.
func Acknowledgement(from, to, knownAs, targetShape string) string {
	known := strings.TrimSpace(knownAs)
	if known == "" {
		known = "-"
	}
	return fmt.Sprintf("MIGRATE %s %s %s TO %s", strings.TrimSpace(from), strings.TrimSpace(to), known, targetShape)
}

// BuildPreview folds shape rules and per-database counts into the shared contract.
// It does not touch a database.
func BuildPreview(req Request, sourceStorage, targetStorage string, snaps []DatabaseSnapshot) Preview {
	req = req.Normalized()
	fromShape := ShapeOf(req.FromRelation, req.ToRelation)
	toShape := ShapeOf(req.TargetFromRelation, req.TargetToRelation)
	class := classifyShapes(fromShape, toShape)
	if req.ConflictPolicy != ConflictPolicyAbort {
		class.BaseSafety = SafetyBlocked
		class.Kind = KindUnknown
	}

	var totals Counts
	blocked := class.BaseSafety == SafetyBlocked || class.Kind == KindUnknown || fromShape == ShapeUnknown || toShape == ShapeUnknown
	var warnings, blockers, conflicts []string
	capability := CapabilitySQL
	for _, snap := range snaps {
		totals.SourcePairs += snap.Counts.SourcePairs
		totals.ValidPairs += snap.Counts.ValidPairs
		totals.Dangling += snap.Counts.Dangling
		totals.Conflicts += snap.Counts.Conflicts
		totals.Contradictory += snap.Counts.Contradictory
		totals.ProjectedDestination += snap.Counts.ProjectedDestination
		if snap.Capability == CapabilityUnsupported {
			capability = CapabilityUnsupported
			blocked = true
			blockers = append(blockers, fmt.Sprintf("%s: unsupported driver", snap.ScopeKey))
		}
		if snap.Error != "" {
			blocked = true
			blockers = append(blockers, snap.ScopeKey+": "+snap.Error)
		}
		if snap.Blocked {
			blocked = true
		}
	}
	if class.BlockOnDegree && totals.Conflicts > 0 {
		blocked = true
		msg := fmt.Sprintf("%d degree conflicts do not fit %s", totals.Conflicts, toShape)
		blockers = append(blockers, msg)
		conflicts = append(conflicts, msg)
	}
	if class.BlockOnContradictory && totals.Contradictory > 0 {
		blocked = true
		msg := fmt.Sprintf("%d contradictory pairs do not fit %s", totals.Contradictory, toShape)
		blockers = append(blockers, msg)
		conflicts = append(conflicts, msg)
	}
	if class.WarnOnContradictory && totals.Contradictory > 0 {
		warnings = append(warnings, fmt.Sprintf("%d contradictory one-to-one pairs will be unioned into the pivot", totals.Contradictory))
	}
	if totals.Dangling > 0 {
		warnings = append(warnings, fmt.Sprintf("%d dangling references will not be copied", totals.Dangling))
	}
	if req.ConflictPolicy != ConflictPolicyAbort {
		blockers = append(blockers, "conflict policy must be abort")
	}

	safety := class.BaseSafety
	rev := RevConditional
	inversePasses := false
	switch {
	case blocked:
		safety = SafetyBlocked
		rev = RevBlocked
	case class.Kind == KindNoOp:
		safety = SafetyNoOp
		rev = RevNow
		inversePasses = true
	case totals.Dangling > 0:
		rev = RevNotBySchema
	case class.Kind == KindWidenToNN:
		rev = RevConditional
		inversePasses = totals.Contradictory == 0
	default:
		rev = RevNow
		inversePasses = true
	}

	affected := req.AffectedDatabases
	if affected == 0 {
		affected = len(snaps)
	}
	if affected == 0 {
		affected = 1
	}
	ack := ""
	typed := false
	if class.Kind != KindNoOp {
		ack = Acknowledgement(req.FromModel, req.ToModel, req.KnownAs, toShape)
		typed = class.DropsStorage || class.BlockOnDegree || totals.Dangling > 0
	}
	actions := []string{"noop"}
	if class.Kind != KindNoOp && !blocked {
		actions = []string{"migrate_connection"}
	}
	if blocked && class.Kind != KindNoOp {
		actions = []string{"migrate_connection"}
	}

	disclaimers := disclaimersFor(class.Kind, rev, totals.Dangling > 0)
	p := Preview{
		TransitionKind: class.Kind, SafetyClass: safety, Reversibility: rev, InversePassesNow: inversePasses,
		SourceShape: fromShape, TargetShape: toShape, SourceStorage: sourceStorage, TargetStorage: targetStorage,
		Steps: class.Steps, SourcePairs: totals.SourcePairs, ValidPairs: totals.ValidPairs, Dangling: totals.Dangling,
		Conflicts: totals.Conflicts, Contradictory: totals.Contradictory, ProjectedDestination: totals.ProjectedDestination,
		AffectedDatabases: affected, Warnings: warnings, Blockers: blockers, Disclaimers: disclaimers,
		ConflictReport: conflicts, BackupRequired: class.Kind != KindNoOp, CodegenImpact: class.Kind != KindNoOp,
		RequiresTypedConfirmation: typed, RequiredAcknowledgement: ack, Blocked: blocked,
		ConflictPolicy: ConflictPolicyAbort, ExecutionActions: actions, Capability: capability, Databases: append([]DatabaseSnapshot(nil), snaps...),
	}
	if totals.ProjectedDestination == 0 && !blocked {
		p.ProjectedDestination = totals.ValidPairs
	}
	p.PlanHash = hashPreview(req, p)
	return p
}

func disclaimersFor(kind, rev string, dangling bool) []string {
	out := []string{
		"This stages a schema migration. It does not publish tenant DDL and it does not restore a previous database.",
		"A snapshot is the only rollback for partial DDL or rows that were already deleted.",
	}
	switch kind {
	case KindWidenToNN:
		out = append(out, "Widening keeps every valid link. The inverse stays conditional because later many-to-many writes can make it not fit.")
	case KindNarrowFromNN, KindNarrowTo11:
		out = append(out, "Narrowing is refused when any record has more than one link. Nothing is kept-first or kept-latest.")
	case KindOwnershipFlip, KindOneToOneToFK:
		out = append(out, "The foreign key moves only when every current pair fits the target holder.")
	case KindNoOp:
		return []string{"The requested cardinality matches the current edge."}
	}
	if dangling {
		out = append(out, "Dangling references are discarded. They are not valid links and cannot be reconstructed from schema history.")
	}
	if rev == RevNotBySchema {
		out = append(out, "Discarded or already-deleted link rows require a database backup or a domain-specific backfill.")
	}
	return out
}

type hashBody struct {
	SchemaVersion int                `json:"schema_version"`
	FromModel     string             `json:"from_model"`
	ToModel       string             `json:"to_model"`
	KnownAs       string             `json:"known_as"`
	FromRelation  string             `json:"from_relation"`
	ToRelation    string             `json:"to_relation"`
	TargetFrom    string             `json:"target_from_relation"`
	TargetTo      string             `json:"target_to_relation"`
	Policy        string             `json:"conflict_policy"`
	Kind          string             `json:"transition_kind"`
	TargetShape   string             `json:"target_shape"`
	Databases     []DatabaseSnapshot `json:"databases"`
}

func hashPreview(req Request, p Preview) string {
	snaps := append([]DatabaseSnapshot(nil), p.Databases...)
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].ScopeKey < snaps[j].ScopeKey })
	body := hashBody{
		SchemaVersion: req.SchemaVersion, FromModel: req.FromModel, ToModel: req.ToModel, KnownAs: req.KnownAs,
		FromRelation: req.FromRelation, ToRelation: req.ToRelation, TargetFrom: req.TargetFromRelation, TargetTo: req.TargetToRelation,
		Policy: ConflictPolicyAbort, Kind: p.TransitionKind, TargetShape: p.TargetShape, Databases: snaps,
	}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// SamePlan reports whether the staged hash still matches a fresh preview.
func SamePlan(expected string, fresh Preview) bool {
	return strings.TrimSpace(expected) != "" && strings.TrimSpace(expected) == fresh.PlanHash
}

type expectedSnapshotKey struct{}

// WithExpectedSnapshots attaches the staged per-database preflight to a publish context.
func WithExpectedSnapshots(ctx context.Context, snaps []DatabaseSnapshot) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	cp := append([]DatabaseSnapshot(nil), snaps...)
	return context.WithValue(ctx, expectedSnapshotKey{}, cp)
}

// ExpectedSnapshots returns the staged preflight, if the publish worker set one.
func ExpectedSnapshots(ctx context.Context) []DatabaseSnapshot {
	if ctx == nil {
		return nil
	}
	snaps, _ := ctx.Value(expectedSnapshotKey{}).([]DatabaseSnapshot)
	return snaps
}

// SnapshotDrift reports when a fresh database preflight no longer matches the staged plan.
func SnapshotDrift(expected []DatabaseSnapshot, fresh DatabaseSnapshot) error {
	if len(expected) == 0 {
		return nil
	}
	for _, snap := range expected {
		if snap.ScopeKey != fresh.ScopeKey {
			continue
		}
		if snap.Counts != fresh.Counts || snap.Blocked != fresh.Blocked {
			return fmt.Errorf("relation plan is stale for %s; preview again", fresh.ScopeKey)
		}
		return nil
	}
	return fmt.Errorf("relation plan has no snapshot for %s; preview again", fresh.ScopeKey)
}

// MetadataOnlyPreview is for drivers whose relation storage does not change with cardinality.
func MetadataOnlyPreview(driver string, req Request) Preview {
	req = req.Normalized()
	snap := DatabaseSnapshot{ScopeKey: driver, Capability: CapabilityMetadataOnly}
	p := BuildPreview(req, "metadata", "metadata", []DatabaseSnapshot{snap})
	p.Capability = CapabilityMetadataOnly
	p.BackupRequired = false
	p.Disclaimers = append(p.Disclaimers, driver+" stores relations without SQL foreign keys. Cardinality is metadata only.")
	p.PlanHash = hashPreview(req, p)
	return p
}

// UnsupportedPreview blocks the transition instead of pretending SQL ran.
func UnsupportedPreview(driver string, req Request) Preview {
	req = req.Normalized()
	snap := DatabaseSnapshot{ScopeKey: driver, Capability: CapabilityUnsupported, Blocked: true, Error: "unsupported_driver"}
	p := BuildPreview(req, "unsupported", "unsupported", []DatabaseSnapshot{snap})
	p.Capability = CapabilityUnsupported
	p.SafetyClass = SafetyUnsupported
	p.Blocked = true
	p.PlanHash = hashPreview(req, p)
	return p
}
