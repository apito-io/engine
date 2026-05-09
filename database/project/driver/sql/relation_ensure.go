package sql

import (
	"context"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// EnsureRelationArtifactsFromSchema creates missing SQL artifacts for schema-declared relations
// (has_many↔has_many pivot tables via AddRelationFields). Idempotent: pivot CREATE uses IF NOT EXISTS.
// Needed when projects were authored on Arango (edges only) or bootstrap only created model tables.
func (S *SQLDriver) EnsureRelationArtifactsFromSchema(ctx context.Context, modelsList []*models.ModelType) error {
	if S == nil || len(modelsList) == 0 {
		return nil
	}
	byName := make(map[string]*models.ModelType)
	for _, m := range modelsList {
		if m == nil {
			continue
		}
		k := strings.TrimSpace(utility.PhysicalSQLTableName(m.Name))
		if k != "" {
			byName[k] = m
		}
	}
	seen := make(map[string]struct{})
	for _, fromModel := range modelsList {
		if fromModel == nil {
			continue
		}
		fromName := strings.TrimSpace(utility.PhysicalSQLTableName(fromModel.Name))
		for _, conn := range fromModel.Connections {
			if conn == nil || conn.Relation != "has_many" {
				continue
			}
			toKey := strings.TrimSpace(utility.PhysicalSQLTableName(conn.Model))
			if toKey == "" {
				continue
			}
			toModel, ok := byName[toKey]
			if !ok {
				continue
			}
			var rev *models.ConnectionType
			for _, back := range toModel.Connections {
				if back == nil {
					continue
				}
				if back.Relation != "has_many" {
					continue
				}
				if back.KnownAs != conn.KnownAs {
					continue
				}
				backKey := strings.TrimSpace(utility.PhysicalSQLTableName(back.Model))
				if backKey == fromName {
					rev = back
					break
				}
			}
			if rev == nil {
				continue
			}
			a, b := fromName, toKey
			if a > b {
				a, b = b, a
			}
			dedup := a + "\x00" + b + "\x00" + conn.KnownAs
			if _, dup := seen[dedup]; dup {
				continue
			}
			seen[dedup] = struct{}{}
			if err := S.AddRelationFields(ctx, conn, rev); err != nil {
				return err
			}
		}
	}
	return nil
}

func findSchemaModel(modelsList []*models.ModelType, name string) *models.ModelType {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	for _, m := range modelsList {
		if m == nil {
			continue
		}
		if utility.ModelIDMatchesGraphQLField(m.Name, name) {
			return m
		}
	}
	return nil
}

// sqlConnectionIsTrueManyToMany is true when both endpoints declare has_many toward each other
// (same KnownAs), i.e. SQL uses a pivot table. Person↔Task typically; Person↔Work with Work has_one Person is false.
func sqlConnectionIsTrueManyToMany(modelsList []*models.ModelType, fromModelName, toModelName string) bool {
	if len(modelsList) == 0 {
		return false
	}
	a := findSchemaModel(modelsList, fromModelName)
	b := findSchemaModel(modelsList, toModelName)
	if a == nil || b == nil {
		return false
	}
	var out *models.ConnectionType
	for _, c := range a.Connections {
		if c == nil || c.Relation != "has_many" {
			continue
		}
		if !utility.ModelIDMatchesGraphQLField(c.Model, b.Name) {
			continue
		}
		out = c
		break
	}
	if out == nil {
		return false
	}
	for _, c := range b.Connections {
		if c == nil || c.Relation != "has_many" {
			continue
		}
		if !utility.ModelIDMatchesGraphQLField(c.Model, a.Name) {
			continue
		}
		if c.KnownAs != out.KnownAs {
			continue
		}
		return true
	}
	return false
}

// sqlConnectionAnchorID returns the parent document id embedded in GraphQL connection variables.
func sqlConnectionAnchorID(connection map[string]interface{}) (string, bool) {
	if connection == nil {
		return "", false
	}
	v, ok := connection["_id"]
	if !ok || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		return s, s != ""
	default:
		return "", false
	}
}

// sqlConnectionAnchorModelName is the schema model id for the document in connection._id
// (matches how the console sends forward vs backward connection payloads).
func sqlConnectionAnchorModelName(connectionType, fromModel, toModel string) string {
	switch strings.TrimSpace(connectionType) {
	case "forward":
		return strings.TrimSpace(fromModel)
	case "backward":
		return strings.TrimSpace(toModel)
	default:
		return ""
	}
}

// fkPhysicalColumnOnModelToTarget returns (column, true) when holder's SQL row has a system-generated
// FK column referencing targetModelName (e.g. work → person gives person_id on table work).
func fkPhysicalColumnOnModelToTarget(holder *models.ModelType, targetModelName, knownAs string) (string, bool) {
	if holder == nil {
		return "", false
	}
	want := relationFKColumnNameParts(targetModelName, knownAs)
	if want == "" {
		return "", false
	}
	for _, f := range holder.Fields {
		if f == nil || !f.SystemGenerated {
			continue
		}
		gql := strings.TrimSpace(f.Identifier)
		if gql == "" {
			continue
		}
		phys := PhysicalSQLColumnForSystemRelationField(gql, holder)
		if phys == want {
			return phys, true
		}
	}
	return "", false
}
