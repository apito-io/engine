package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

type TeamMemberAddRequest struct {
	ProjectID   string   `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	UserID      string   `json:"user_id,omitempty" firestore:"user_id,omitempty" bson:"user_id,omitempty"`
	Email       string   `json:"email,omitempty" firestore:"email,omitempty" bson:"email,omitempty"`
	Role        string   `json:"role,omitempty" firestore:"role,omitempty" bson:"role,omitempty"`
	TeamID      string   `json:"team_id,omitempty" firestore:"team_id,omitempty" bson:"team_id,omitempty"`
	Permissions []string `json:"permissions,omitempty" firestore:"permissions,omitempty" bson:"permissions,omitempty"`
}

type APIPermission struct {
	Read   string `json:"read,omitempty" firestore:"read,omitempty" bson:"read,omitempty"`
	Create string `json:"create,omitempty" firestore:"create,omitempty" bson:"create,omitempty"`
	Update string `json:"update,omitempty" firestore:"update,omitempty" bson:"update,omitempty"`
	Delete string `json:"delete,omitempty" firestore:"delete,omitempty" bson:"delete,omitempty"`
}

type Role struct {
	ID                        string                    `bun:"id,pk,notnull,type:uuid,default:gen_random_uuid()" json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	APIPermissions            map[string]*APIPermission `bun:",type:jsonb" json:"api_permissions,omitempty" firestore:"permissions,omitempty" bson:"api_permissions,omitempty"`
	AdministrativePermissions []string                  `json:"administrative_permissions,omitempty" firestore:"administrative_permissions,omitempty" bson:"administrative_permissions,omitempty"`
	LogicExecutions           []string                  `json:"logic_executions,omitempty" firestore:"logic_executions,omitempty" bson:"logic_executions,omitempty"`
	SystemGenerated           bool                      `json:"system_generated,omitempty" firestore:"system_generated,omitempty" bson:"system_generated,omitempty"`
	IsAdmin                   bool                      `json:"is_admin,omitempty" firestore:"is_admin,omitempty" bson:"is_admin,omitempty"`
	IsProjectUser             bool                      `json:"is_project_user,omitempty" firestore:"is_project_user,omitempty" bson:"is_project_user,omitempty"`
	ReadOnlyProject           bool                      `json:"read_only_project,omitempty" firestore:"read_only_project,omitempty" bson:"read_only_project,omitempty"`
	// PlanClamped is set at request time when a tenant plan ceiling was applied.
	// When true, RoleBypassesDataACL returns false even for admin/owner ids.
	PlanClamped bool `json:"-" firestore:"-" bson:"-" exclude:"true"`
}

// Plan is a project-defined permission ceiling + quotas assigned to tenants via plan_tier slug.
// Effective app-user permissions = intersect(role, plan). Console operators are exempt.
type Plan struct {
	ID              string                    `json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	Name            string                    `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Description     string                    `json:"description,omitempty" firestore:"description,omitempty" bson:"description,omitempty"`
	APIPermissions  map[string]*APIPermission `json:"api_permissions,omitempty" firestore:"api_permissions,omitempty" bson:"api_permissions,omitempty"`
	LogicExecutions []string                  `json:"logic_executions,omitempty" firestore:"logic_executions,omitempty" bson:"logic_executions,omitempty"`
	// Quotas keys: max_app_users, max_records.<model>, storage_mb. Zero / missing = unlimited.
	Quotas          map[string]int `json:"quotas,omitempty" firestore:"quotas,omitempty" bson:"quotas,omitempty"`
	SystemGenerated bool           `json:"system_generated,omitempty" firestore:"system_generated,omitempty" bson:"system_generated,omitempty"`
}

// DefaultSeededPlans returns fully-permissive starter plans so deploying
// plan enforcement changes nothing until an operator tightens free.
func DefaultSeededPlans() map[string]*Plan {
	all := &APIPermission{Read: "all", Create: "all", Update: "all", Delete: "all"}
	mk := func(id, name, desc string) *Plan {
		return &Plan{
			ID:              id,
			Name:            name,
			Description:     desc,
			APIPermissions:  map[string]*APIPermission{"*": all},
			LogicExecutions: []string{"*"},
			Quotas:          map[string]int{},
			SystemGenerated: true,
		}
	}
	return map[string]*Plan{
		"free":      mk("free", "Free", "Default starter plan — fully permissive until customized"),
		"paid":      mk("paid", "Paid", "Paid plan — fully permissive until customized"),
		"paid_plus": mk("paid_plus", "Paid Plus", "Paid Plus plan — fully permissive until customized"),
		"ultra":     mk("ultra", "Ultra", "Ultra plan — fully permissive until customized"),
	}
}

// EnsureProjectPlansSeeds merges missing system-generated starter plans into project.Plans.
func EnsureProjectPlansSeeds(project *Project) {
	if project == nil {
		return
	}
	if project.Plans == nil {
		project.Plans = make(map[string]*Plan)
	}
	for id, seed := range DefaultSeededPlans() {
		if _, ok := project.Plans[id]; ok {
			continue
		}
		cp := *seed
		if seed.APIPermissions != nil {
			cp.APIPermissions = make(map[string]*APIPermission, len(seed.APIPermissions))
			for k, v := range seed.APIPermissions {
				if v == nil {
					continue
				}
				vv := *v
				cp.APIPermissions[k] = &vv
			}
		}
		if seed.LogicExecutions != nil {
			cp.LogicExecutions = append([]string(nil), seed.LogicExecutions...)
		}
		if seed.Quotas != nil {
			cp.Quotas = make(map[string]int, len(seed.Quotas))
			for k, v := range seed.Quotas {
				cp.Quotas[k] = v
			}
		}
		project.Plans[id] = &cp
	}
}

// NormalizePlanSlug lowercases and normalizes common aliases (paid+ → paid_plus).
func NormalizePlanSlug(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	trimmed = strings.ReplaceAll(trimmed, "-", "_")
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	switch trimmed {
	case "paid+", "paidplus":
		return "paid_plus"
	case "":
		return "free"
	default:
		return trimmed
	}
}

// ValidatePlanSlugAgainstProject ensures the slug exists in project.Plans (after seeding defaults).
func ValidatePlanSlugAgainstProject(project *Project, raw string) (string, error) {
	EnsureProjectPlansSeeds(project)
	slug := NormalizePlanSlug(raw)
	if project == nil || project.Plans == nil {
		return "", fmt.Errorf("invalid plan_tier: %s (no plans defined)", slug)
	}
	if _, ok := project.Plans[slug]; !ok {
		return "", fmt.Errorf("invalid plan_tier: %s (not a defined project plan)", slug)
	}
	return slug, nil
}

// MarshalApiPermissions serializes ApiPermissions to JSON.
func (u *Role) MarshalAPIPermissions() ([]byte, error) {
	return json.Marshal(u.APIPermissions)
}

// UnmarshalApiPermissions deserializes JSON to ApiPermissions.
func (u *Role) UnmarshalAPIPermissions(data []byte) error {
	return json.Unmarshal(data, &u.APIPermissions)
}
