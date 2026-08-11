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

	// Monetization (ApitoProjectPayment) — optional list price + store SKUs. Not used by ApitoPayment (platform Paddle).
	Currency       string  `json:"currency,omitempty" firestore:"currency,omitempty" bson:"currency,omitempty"`               // derived from default prices[] row
	PriceMonthly   float64 `json:"price_monthly,omitempty" firestore:"price_monthly,omitempty" bson:"price_monthly,omitempty"` // derived from default prices[] row
	PlayProductID  string  `json:"play_product_id,omitempty" firestore:"play_product_id,omitempty" bson:"play_product_id,omitempty"` // legacy; prefer ProviderProducts
	PlayBasePlanID string  `json:"play_base_plan_id,omitempty" firestore:"play_base_plan_id,omitempty" bson:"play_base_plan_id,omitempty"`
	PaddlePriceID  string  `json:"paddle_price_id,omitempty" firestore:"paddle_price_id,omitempty" bson:"paddle_price_id,omitempty"`

	// Prices is the source of truth for multi-currency list prices.
	Prices []PlanPrice `json:"prices,omitempty" firestore:"prices,omitempty" bson:"prices,omitempty"`
	// ProviderProducts links this plan to store/catalog IDs (google_play, paddle, stripe, …).
	ProviderProducts []PlanProviderProduct `json:"provider_products,omitempty" firestore:"provider_products,omitempty" bson:"provider_products,omitempty"`
}

// PlanPrice is one currency amount for a tenant SaaS plan.
type PlanPrice struct {
	Currency string  `json:"currency,omitempty" firestore:"currency,omitempty" bson:"currency,omitempty"`
	Amount   float64 `json:"amount,omitempty" firestore:"amount,omitempty" bson:"amount,omitempty"`
	Default  bool    `json:"default,omitempty" firestore:"default,omitempty" bson:"default,omitempty"`
}

// PlanProviderProduct maps a billing provider to this plan's product_id (common name).
type PlanProviderProduct struct {
	Provider  string `json:"provider,omitempty" firestore:"provider,omitempty" bson:"provider,omitempty"`
	ProductID string `json:"product_id,omitempty" firestore:"product_id,omitempty" bson:"product_id,omitempty"`
	VariantID string `json:"variant_id,omitempty" firestore:"variant_id,omitempty" bson:"variant_id,omitempty"`
}

// DefaultSeededPlans returns the only system-generated starter plan: free.
// Any other tiers are created per project via Console, MCP upsert_plan, or
// product upsert scripts — never product-specific logic in open-core.
func DefaultSeededPlans() map[string]*Plan {
	all := &APIPermission{Read: "all", Create: "all", Update: "all", Delete: "all"}
	return map[string]*Plan{
		"free": {
			ID:              "free",
			Name:            "Free",
			Description:     "Default starter plan — fully permissive until customized",
			APIPermissions:  map[string]*APIPermission{"*": all},
			LogicExecutions: []string{"*"},
			Quotas:          map[string]int{},
			SystemGenerated: true,
		},
	}
}

// EnsureProjectPlansSeeds guarantees free exists as the required system default.
// Legacy non-free plans still flagged system_generated are demoted to custom so
// operators can delete or reshape them (Console / MCP / scripts).
// Returns true when the in-memory project was mutated (caller should persist).
func EnsureProjectPlansSeeds(project *Project) (changed bool) {
	if project == nil {
		return false
	}
	if project.Plans == nil {
		project.Plans = make(map[string]*Plan)
		changed = true
	}
	omit := make(map[string]struct{}, len(project.PlanSeedOmissions))
	for _, id := range project.PlanSeedOmissions {
		id = NormalizePlanSlug(id)
		if id == "" || id == "free" {
			continue
		}
		omit[id] = struct{}{}
	}
	for id, seed := range DefaultSeededPlans() {
		if _, skipped := omit[id]; skipped {
			continue
		}
		if existing, ok := project.Plans[id]; ok && existing != nil {
			if existing.ID != id {
				existing.ID = id
				changed = true
			}
			if !existing.SystemGenerated {
				existing.SystemGenerated = true
				changed = true
			}
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
		changed = true
	}
	// Demote any non-free plan still flagged system_generated (legacy seeds).
	for id, pl := range project.Plans {
		if pl == nil {
			continue
		}
		slug := NormalizePlanSlug(id)
		if slug == "" {
			slug = NormalizePlanSlug(pl.ID)
		}
		if slug == "free" {
			if pl.ID != "free" {
				pl.ID = "free"
				changed = true
			}
			if !pl.SystemGenerated {
				pl.SystemGenerated = true
				changed = true
			}
			continue
		}
		if pl.SystemGenerated {
			pl.SystemGenerated = false
			changed = true
		}
	}
	return changed
}

// MarkPlanSeedOmitted records that a default seed slug was removed by the operator.
func MarkPlanSeedOmitted(project *Project, slug string) {
	if project == nil {
		return
	}
	slug = NormalizePlanSlug(slug)
	if slug == "" || slug == "free" {
		return
	}
	for _, id := range project.PlanSeedOmissions {
		if NormalizePlanSlug(id) == slug {
			return
		}
	}
	project.PlanSeedOmissions = append(project.PlanSeedOmissions, slug)
}

// ClearPlanSeedOmission allows a previously deleted seed slug to be re-created / upserted.
func ClearPlanSeedOmission(project *Project, slug string) {
	if project == nil || len(project.PlanSeedOmissions) == 0 {
		return
	}
	slug = NormalizePlanSlug(slug)
	out := project.PlanSeedOmissions[:0]
	for _, id := range project.PlanSeedOmissions {
		if NormalizePlanSlug(id) == slug {
			continue
		}
		out = append(out, id)
	}
	project.PlanSeedOmissions = out
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
