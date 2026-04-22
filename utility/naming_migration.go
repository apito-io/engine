package utility

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/models"
)

// NamingV2ModelRenamePair is one model identifier change produced by naming schema V2 migration.
type NamingV2ModelRenamePair struct {
	Old string
	New string
}

// NamingV2PhysicalMigrator is implemented by project database drivers that can rewrite
// persisted rows (for example Arango document `type` and collection names) before the
// in-memory schema is migrated.
// relationTenantModel is the SaaS tenant root model name (e.g. "restaurant"); when non-empty,
// relation edges that still carry root-level tenant_id are rewritten into ext (tenant_id, tenant_model).
type NamingV2PhysicalMigrator interface {
	ApplyNamingV2PhysicalMigration(ctx context.Context, projectID string, pairs []NamingV2ModelRenamePair, perModelCollections bool, relationTenantModel string) error
}

// ComputeNamingV2ModelRenamePairs returns rename pairs for a project still on legacy naming.
// When the project is already on naming V2, or has no schema, it returns (nil, nil).
func ComputeNamingV2ModelRenamePairs(project *models.Project) ([]NamingV2ModelRenamePair, error) {
	if project == nil || project.Schema == nil {
		return nil, nil
	}
	if project.Schema.NamingSchemaVersion >= NamingSchemaVersionV2 {
		return nil, nil
	}
	return computeNamingV2RenamePairsFromModels(project.Schema.Models)
}

func computeNamingV2RenamePairsFromModels(modelsList []*models.ModelType) ([]NamingV2ModelRenamePair, error) {
	var pairs []NamingV2ModelRenamePair
	seen := make(map[string]struct{})
	for _, m := range modelsList {
		if m == nil {
			continue
		}
		canonical, err := LegacyStoredNameToCanonical(m.Name)
		if err != nil {
			return nil, fmt.Errorf("naming migration: model %q: %w", m.Name, err)
		}
		if _, dup := seen[canonical]; dup {
			return nil, fmt.Errorf("naming migration: duplicate canonical model id %q", canonical)
		}
		seen[canonical] = struct{}{}
		if canonical != m.Name {
			pairs = append(pairs, NamingV2ModelRenamePair{Old: m.Name, New: canonical})
		}
	}
	return pairs, nil
}

// MigrateProjectSchemaToNamingV2 rewrites stored model names to canonical snake_case,
// updates connection references and role API permission keys, and sets NamingSchemaVersion.
// Returns (true, nil) when the project was modified and should be persisted.
func MigrateProjectSchemaToNamingV2(project *models.Project) (bool, error) {
	if project == nil || project.Schema == nil {
		return false, nil
	}
	if project.Schema.NamingSchemaVersion >= NamingSchemaVersionV2 {
		return false, nil
	}

	pairs, err := computeNamingV2RenamePairsFromModels(project.Schema.Models)
	if err != nil {
		return false, err
	}

	rename := make(map[string]string)
	for _, p := range pairs {
		rename[p.Old] = p.New
	}

	if len(rename) == 0 {
		project.Schema.NamingSchemaVersion = NamingSchemaVersionV2
		return true, nil
	}

	for _, m := range project.Schema.Models {
		if m == nil {
			continue
		}
		if neu, ok := rename[m.Name]; ok {
			m.Name = neu
		}
		for _, c := range m.Connections {
			if c == nil {
				continue
			}
			if neu, ok := rename[c.Model]; ok {
				c.Model = neu
			}
		}
	}

	if project.Roles != nil {
		for _, role := range project.Roles {
			if role == nil || role.APIPermissions == nil {
				continue
			}
			newPerms := make(map[string]*models.APIPermission, len(role.APIPermissions))
			for k, v := range role.APIPermissions {
				if neu, ok := rename[k]; ok {
					newPerms[neu] = v
				} else {
					newPerms[k] = v
				}
			}
			role.APIPermissions = newPerms
		}
	}

	project.Schema.NamingSchemaVersion = NamingSchemaVersionV2
	return true, nil
}
