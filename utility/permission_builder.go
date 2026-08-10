package utility

import (
	"errors"
	"strings"

	"github.com/apito-io/engine/models"
)

type CRUDPermissions struct {
	Create bool
	Read   bool
	Update bool
	Delete bool
}

func noneAPIPermission() *models.APIPermission {
	return &models.APIPermission{
		Read:   "none",
		Create: "none",
		Update: "none",
		Delete: "none",
	}
}

func allAPIPermission() *models.APIPermission {
	return &models.APIPermission{
		Read:   "all",
		Create: "all",
		Update: "all",
		Delete: "all",
	}
}

// LookupAPIPermission returns role permissions for a model id (canonical snake or legacy camel key).
func LookupAPIPermission(role *models.Role, modelName string) (*models.APIPermission, bool) {
	if role == nil || role.APIPermissions == nil {
		return nil, false
	}
	if ap, ok := role.APIPermissions[modelName]; ok {
		return ap, true
	}
	if ap, ok := role.APIPermissions[CamelFromAny(modelName)]; ok {
		return ap, true
	}
	return nil, false
}

// RoleBypassesDataACL is true for project admin (IsAdmin or id admin/owner),
// unless a tenant plan ceiling has been applied (PlanClamped).
func RoleBypassesDataACL(role *models.Role) bool {
	if role == nil || role.PlanClamped {
		return false
	}
	if role.IsAdmin {
		return true
	}
	id := strings.ToLower(strings.TrimSpace(role.ID))
	return id == "admin" || id == "owner"
}

// EffectivePermission returns the resolved APIPermission for a model.
//   - Admin bypass → all/all/all/all
//   - demo system role (id=="demo" && SystemGenerated) → read all, mutations none
//   - team system role → use LookupAPIPermission or none defaults (NOT blanket all)
//   - Do NOT treat SystemGenerated alone as all CRUD
//   - Missing model → all none
func EffectivePermission(role *models.Role, modelName string) *models.APIPermission {
	if role == nil {
		return noneAPIPermission()
	}
	if RoleBypassesDataACL(role) {
		return allAPIPermission()
	}
	if role.ID == "demo" && role.SystemGenerated {
		return &models.APIPermission{
			Read:   "all",
			Create: "none",
			Update: "none",
			Delete: "none",
		}
	}
	if ap, ok := LookupAPIPermission(role, modelName); ok && ap != nil {
		return ap
	}
	return noneAPIPermission()
}

// BuildCRUDPermissions is a thin wrapper around EffectivePermission for backward compatibility.
func BuildCRUDPermissions(modelName string, role *models.Role) (*models.APIPermission, error) {
	if role == nil {
		return nil, errors.New("Role Cant be Nil")
	}
	return EffectivePermission(role, modelName), nil
}

func ValidatePermissions(vv map[string]interface{}) (*models.APIPermission, error) {
	var p = &models.APIPermission{}
	if val, ok := ValidateReadScope(vv["read"]); ok && val != nil {
		p.Read = *val
	} else {
		return nil, errors.New("invalid Read Permissions")
	}
	if val, ok := ValidateCreateScope(vv["create"]); ok && val != nil {
		p.Create = *val
	} else {
		return nil, errors.New("invalid Create Permissions")
	}
	if val, ok := ValidateUpdateScope(vv["update"]); ok && val != nil {
		p.Update = *val
	} else {
		return nil, errors.New("invalid Update Permissions")
	}
	if val, ok := ValidateDeleteScope(vv["delete"]); ok && val != nil {
		p.Delete = *val
	} else {
		return nil, errors.New("invalid Delete Permissions")
	}
	return p, nil
}

// NormalizeLegacyScope maps retired scope values for migration of stored roles.
// custom_logic → none. Other values are returned unchanged (trimmed).
func NormalizeLegacyScope(s string) string {
	s = strings.TrimSpace(s)
	if s == "custom_logic" {
		return "none"
	}
	return s
}

func validateScopeIn(p interface{}, allowed map[string]struct{}) (*string, bool) {
	if p == nil {
		none := "none"
		return &none, true
	}
	s, ok := p.(string)
	if !ok || s == "" {
		return nil, false
	}
	s = strings.TrimSpace(s)
	// Reject retired scopes at validation time; use NormalizeLegacyScope for stored-role migration.
	if s == "custom_logic" {
		return nil, false
	}
	if _, ok := allowed[s]; !ok {
		return nil, false
	}
	out := s
	return &out, true
}

var (
	readScopes = map[string]struct{}{
		"none": {}, "all": {}, "auth": {}, "own": {},
	}
	createScopes = map[string]struct{}{
		"none": {}, "all": {}, "auth": {},
	}
	updateDeleteScopes = map[string]struct{}{
		"none": {}, "all": {}, "auth": {}, "own": {},
	}
)

// ValidateReadScope accepts none | all | auth | own.
func ValidateReadScope(p interface{}) (*string, bool) {
	return validateScopeIn(p, readScopes)
}

// ValidateCreateScope accepts none | all | auth (rejects own and custom_logic).
func ValidateCreateScope(p interface{}) (*string, bool) {
	return validateScopeIn(p, createScopes)
}

// ValidateUpdateScope accepts none | all | auth | own.
func ValidateUpdateScope(p interface{}) (*string, bool) {
	return validateScopeIn(p, updateDeleteScopes)
}

// ValidateDeleteScope accepts none | all | auth | own.
func ValidateDeleteScope(p interface{}) (*string, bool) {
	return validateScopeIn(p, updateDeleteScopes)
}

// ValidateScope accepts permission scope strings for read/update/delete-compatible sets.
// Prefer operation-aware validators. custom_logic is rejected.
func ValidateScope(p interface{}) (*string, bool) {
	return ValidateReadScope(p)
}

// MigrateAPIPermissionScopes maps custom_logic → none on each field. Returns whether any field changed.
func MigrateAPIPermissionScopes(ap *models.APIPermission) (changed bool) {
	if ap == nil {
		return false
	}
	if ap.Read == "custom_logic" {
		ap.Read = "none"
		changed = true
	}
	if ap.Create == "custom_logic" {
		ap.Create = "none"
		changed = true
	}
	if ap.Update == "custom_logic" {
		ap.Update = "none"
		changed = true
	}
	if ap.Delete == "custom_logic" {
		ap.Delete = "none"
		changed = true
	}
	return changed
}

// MigrateRoleAPIPermissions migrates custom_logic scopes on all model permissions.
// Returns the count of fields migrated.
func MigrateRoleAPIPermissions(role *models.Role) int {
	if role == nil || role.APIPermissions == nil {
		return 0
	}
	count := 0
	for _, ap := range role.APIPermissions {
		if ap == nil {
			continue
		}
		before := *ap
		if MigrateAPIPermissionScopes(ap) {
			if before.Read != ap.Read {
				count++
			}
			if before.Create != ap.Create {
				count++
			}
			if before.Update != ap.Update {
				count++
			}
			if before.Delete != ap.Delete {
				count++
			}
		}
	}
	return count
}

// CloneRole deep-copies a role for duplication. Copied roles are never admin or system-generated.
func CloneRole(src *models.Role) *models.Role {
	if src == nil {
		return &models.Role{}
	}
	dst := &models.Role{
		SystemGenerated: false,
		IsAdmin:         false,
		IsProjectUser:   src.IsProjectUser,
		ReadOnlyProject: src.ReadOnlyProject,
	}
	if len(src.LogicExecutions) > 0 {
		dst.LogicExecutions = append([]string(nil), src.LogicExecutions...)
	}
	if len(src.AdministrativePermissions) > 0 {
		dst.AdministrativePermissions = append([]string(nil), src.AdministrativePermissions...)
	}
	if src.APIPermissions != nil {
		dst.APIPermissions = make(map[string]*models.APIPermission, len(src.APIPermissions))
		for k, v := range src.APIPermissions {
			if v == nil {
				continue
			}
			cp := *v
			dst.APIPermissions[k] = &cp
		}
	}
	return dst
}

// PermissionOp identifies a CRUD operation for scope ranking / min.
type PermissionOp string

const (
	OpRead   PermissionOp = "read"
	OpCreate PermissionOp = "create"
	OpUpdate PermissionOp = "update"
	OpDelete PermissionOp = "delete"
)

// ScopeRank returns the ordinal of a scope for the given operation.
// Lattice: none < own < auth < all. Create has no own (own ranks as none).
func ScopeRank(op PermissionOp, scope string) int {
	s := strings.TrimSpace(strings.ToLower(scope))
	if s == "" || s == "none" || s == "custom_logic" {
		return 0
	}
	if op == OpCreate && s == "own" {
		return 0
	}
	switch s {
	case "own":
		return 1
	case "auth":
		return 2
	case "all":
		return 3
	default:
		return 0
	}
}

// MinScope returns the more restrictive of a and b for the given operation.
func MinScope(op PermissionOp, a, b string) string {
	ra, rb := ScopeRank(op, a), ScopeRank(op, b)
	if ra <= rb {
		return normalizeScopeForOp(op, a)
	}
	return normalizeScopeForOp(op, b)
}

func normalizeScopeForOp(op PermissionOp, scope string) string {
	s := strings.TrimSpace(strings.ToLower(scope))
	if s == "" || s == "custom_logic" {
		return "none"
	}
	if op == OpCreate && s == "own" {
		return "none"
	}
	switch s {
	case "none", "own", "auth", "all":
		return s
	default:
		return "none"
	}
}

// LookupPlanAPIPermission returns plan permissions for a model, falling back to "*" wildcard.
func LookupPlanAPIPermission(plan *models.Plan, modelName string) (*models.APIPermission, bool) {
	if plan == nil || plan.APIPermissions == nil {
		return nil, false
	}
	if ap, ok := plan.APIPermissions[modelName]; ok && ap != nil {
		return ap, true
	}
	if ap, ok := plan.APIPermissions[CamelFromAny(modelName)]; ok && ap != nil {
		return ap, true
	}
	if ap, ok := plan.APIPermissions["*"]; ok && ap != nil {
		return ap, true
	}
	return nil, false
}

// PlanAllowsLogic reports whether a plan permits a logic/function execution.
// Empty LogicExecutions or "*" entry means fully permissive.
func PlanAllowsLogic(plan *models.Plan, fn string) bool {
	if plan == nil {
		return true
	}
	if len(plan.LogicExecutions) == 0 {
		return true
	}
	fn = strings.TrimSpace(fn)
	for _, e := range plan.LogicExecutions {
		e = strings.TrimSpace(e)
		if e == "*" || (fn != "" && e == fn) {
			return true
		}
	}
	return false
}

// IntersectLogicExecutions returns the intersection of role and plan logic lists.
// A nil/empty plan list or "*" means the role list is unchanged (fully permissive plan).
func IntersectLogicExecutions(roleLogic, planLogic []string) []string {
	if len(planLogic) == 0 {
		return append([]string(nil), roleLogic...)
	}
	planAllowsAll := false
	planSet := make(map[string]struct{}, len(planLogic))
	for _, e := range planLogic {
		e = strings.TrimSpace(e)
		if e == "*" {
			planAllowsAll = true
			break
		}
		if e != "" {
			planSet[e] = struct{}{}
		}
	}
	if planAllowsAll {
		return append([]string(nil), roleLogic...)
	}
	if len(roleLogic) == 0 {
		return nil
	}
	out := make([]string, 0, len(roleLogic))
	for _, e := range roleLogic {
		e = strings.TrimSpace(e)
		if e == "*" {
			// Role allows all; result is the plan allowlist.
			for p := range planSet {
				out = append(out, p)
			}
			return out
		}
		if _, ok := planSet[e]; ok {
			out = append(out, e)
		}
	}
	return out
}

// IntersectAPIPermission applies the plan ceiling with read-only grace:
//   create/update/delete = min(role, plan) always
//   read = min(role, plan) when plan.read != none; otherwise falls back to role.read
// Returns the intersected permission and whether read grace applied.
func IntersectAPIPermission(roleAP, planAP *models.APIPermission) (out *models.APIPermission, readGrace bool) {
	roleAP = coalesceAP(roleAP)
	planAP = coalesceAP(planAP)
	out = &models.APIPermission{
		Create: MinScope(OpCreate, roleAP.Create, planAP.Create),
		Update: MinScope(OpUpdate, roleAP.Update, planAP.Update),
		Delete: MinScope(OpDelete, roleAP.Delete, planAP.Delete),
	}
	planRead := normalizeScopeForOp(OpRead, planAP.Read)
	if planRead == "none" {
		out.Read = normalizeScopeForOp(OpRead, roleAP.Read)
		readGrace = ScopeRank(OpRead, roleAP.Read) > 0
	} else {
		out.Read = MinScope(OpRead, roleAP.Read, planAP.Read)
	}
	return out, readGrace
}

func coalesceAP(ap *models.APIPermission) *models.APIPermission {
	if ap == nil {
		return noneAPIPermission()
	}
	return ap
}

// IntersectRoleWithPlan clones the role and clamps it under the plan ceiling.
// Clears IsAdmin so RoleBypassesDataACL no longer wins for tenant app users.
// Nil plan returns a clone with IsAdmin cleared still? No — nil plan means no ceiling (permissive).
// Plan with empty api_permissions is treated as fully permissive (seeded "*" style).
func IntersectRoleWithPlan(role *models.Role, plan *models.Plan) *models.Role {
	if role == nil {
		return nil
	}
	if plan == nil {
		return CloneRoleKeepingIdentity(role)
	}
	out := CloneRoleKeepingIdentity(role)
	// Plan present → clamp admin bypass so tenant admins cannot escape the ceiling.
	out.IsAdmin = false
	out.PlanClamped = true

	out.LogicExecutions = IntersectLogicExecutions(role.LogicExecutions, plan.LogicExecutions)

	// Build model key union from role + plan (excluding wildcard).
	modelsSet := make(map[string]struct{})
	if role.APIPermissions != nil {
		for k := range role.APIPermissions {
			if k != "*" {
				modelsSet[k] = struct{}{}
			}
		}
	}
	if plan.APIPermissions != nil {
		for k := range plan.APIPermissions {
			if k != "*" {
				modelsSet[k] = struct{}{}
			}
		}
	}

	out.APIPermissions = make(map[string]*models.APIPermission, len(modelsSet)+1)
	for modelName := range modelsSet {
		roleAP, _ := LookupAPIPermission(role, modelName)
		if roleAP == nil {
			roleAP = noneAPIPermission()
		}
		planAP, ok := LookupPlanAPIPermission(plan, modelName)
		if !ok || planAP == nil {
			// No plan entry and no wildcard → fully permissive plan for this model.
			planAP = allAPIPermission()
		}
		intersected, _ := IntersectAPIPermission(roleAP, planAP)
		out.APIPermissions[modelName] = intersected
	}
	return out
}

// CloneRoleKeepingIdentity deep-copies a role including ID / admin / system flags.
func CloneRoleKeepingIdentity(src *models.Role) *models.Role {
	if src == nil {
		return &models.Role{}
	}
	dst := &models.Role{
		ID:              src.ID,
		SystemGenerated: src.SystemGenerated,
		IsAdmin:         src.IsAdmin,
		IsProjectUser:   src.IsProjectUser,
		ReadOnlyProject: src.ReadOnlyProject,
	}
	if len(src.LogicExecutions) > 0 {
		dst.LogicExecutions = append([]string(nil), src.LogicExecutions...)
	}
	if len(src.AdministrativePermissions) > 0 {
		dst.AdministrativePermissions = append([]string(nil), src.AdministrativePermissions...)
	}
	if src.APIPermissions != nil {
		dst.APIPermissions = make(map[string]*models.APIPermission, len(src.APIPermissions))
		for k, v := range src.APIPermissions {
			if v == nil {
				continue
			}
			cp := *v
			dst.APIPermissions[k] = &cp
		}
	}
	return dst
}

// ErrQuotaExceeded is returned when a create would exceed a plan quota.
const ErrQuotaExceeded = "plan quota exceeded"

// QuotaKeyAppUsers is the quotas map key for max app users in a tenant.
const QuotaKeyAppUsers = "max_app_users"

// QuotaKeyStorageMB is the quotas map key for storage limit in megabytes.
const QuotaKeyStorageMB = "storage_mb"

// QuotaKeyRecordsPrefix prefixes per-model record limits: max_records.<model>.
const QuotaKeyRecordsPrefix = "max_records."

// CheckPlanRecordsQuota returns ErrQuotaExceeded when currentCount >= limit (>0).
// Limit 0 means unlimited.
func CheckPlanRecordsQuota(plan *models.Plan, modelName string, currentCount int) error {
	limit := PlanQuotaLimit(plan, RecordsQuotaKey(modelName))
	if limit <= 0 {
		return nil
	}
	if currentCount >= limit {
		return errors.New(ErrQuotaExceeded + ": " + RecordsQuotaKey(modelName))
	}
	return nil
}

// CheckPlanAppUsersQuota returns ErrQuotaExceeded when currentCount >= max_app_users.
func CheckPlanAppUsersQuota(plan *models.Plan, currentCount int) error {
	limit := PlanQuotaLimit(plan, QuotaKeyAppUsers)
	if limit <= 0 {
		return nil
	}
	if currentCount >= limit {
		return errors.New(ErrQuotaExceeded + ": " + QuotaKeyAppUsers)
	}
	return nil
}

// RecordsQuotaKey builds max_records.<model> quota key.
func RecordsQuotaKey(modelName string) string {
	return QuotaKeyRecordsPrefix + strings.TrimSpace(modelName)
}

// PlanQuotaLimit returns the numeric quota for key, or 0 when unlimited / missing.
func PlanQuotaLimit(plan *models.Plan, key string) int {
	if plan == nil || plan.Quotas == nil {
		return 0
	}
	return plan.Quotas[key]
}

// ComputeReadGraceByModel returns per-model true when read grace applied under the plan.
func ComputeReadGraceByModel(role *models.Role, plan *models.Plan) map[string]bool {
	out := make(map[string]bool)
	if role == nil || plan == nil {
		return out
	}
	modelsSet := make(map[string]struct{})
	if role.APIPermissions != nil {
		for k := range role.APIPermissions {
			if k != "*" {
				modelsSet[k] = struct{}{}
			}
		}
	}
	if plan.APIPermissions != nil {
		for k := range plan.APIPermissions {
			if k != "*" {
				modelsSet[k] = struct{}{}
			}
		}
	}
	for modelName := range modelsSet {
		roleAP, _ := LookupAPIPermission(role, modelName)
		if roleAP == nil {
			roleAP = noneAPIPermission()
		}
		planAP, ok := LookupPlanAPIPermission(plan, modelName)
		if !ok || planAP == nil {
			planAP = allAPIPermission()
		}
		_, grace := IntersectAPIPermission(roleAP, planAP)
		if grace {
			out[modelName] = true
		}
	}
	return out
}
