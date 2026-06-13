package mariadb

import (
	"context"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// EnsureRelationArtifactsFromSchema creates missing SQL artifacts for schema-declared relations
// (pivot tables, FK columns, constraints) via AddRelationFields. Idempotent where supported.
// Needed when projects were authored on Arango (edges only) or bootstrap only created model tables.
func (d *Driver) EnsureRelationArtifactsFromSchema(ctx context.Context, modelsList []*models.ModelType) error {
	if d == nil || len(modelsList) == 0 {
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
			if conn == nil {
				continue
			}
			// Each relation has a forward entry on one model and a backward entry on the peer.
			// Process once from the forward side only (matches addRelation mutation argument order).
			if strings.TrimSpace(conn.Type) != "forward" {
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
			rev := schemaReverseConnection(toModel, fromName, conn)
			if rev == nil || strings.TrimSpace(rev.Type) != "backward" {
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
			fromArg, toArg := addRelationFieldsArgsFromSchemaPair(fromModel, conn, toModel, rev)
			if fromArg == nil || toArg == nil {
				continue
			}
			if err := d.AddRelationFields(ctx, fromArg, toArg); err != nil {
				return err
			}
		}
	}
	return nil
}

// addRelationFieldsArgsFromSchemaPair builds (from, to) for AddRelationFields from a forward/backward schema pair.
func addRelationFieldsArgsFromSchemaPair(forwardModel *models.ModelType, forwardConn *models.ConnectionType, backwardModel *models.ModelType, backwardConn *models.ConnectionType) (*models.ConnectionType, *models.ConnectionType) {
	if forwardModel == nil || forwardConn == nil || backwardModel == nil || backwardConn == nil {
		return nil, nil
	}
	clone := func(c *models.ConnectionType) *models.ConnectionType {
		if c == nil {
			return nil
		}
		cp := *c
		return &cp
	}
	switch {
	case forwardConn.Relation == "has_one" && backwardConn.Relation == "has_many":
		// FK on the has_one holder; AddRelationFields(has_one, has_many) with schema peer targets.
		return clone(forwardConn), clone(backwardConn)
	case forwardConn.Relation == "has_many" && backwardConn.Relation == "has_one":
		// Default: schema peer targets, has_one conn first → FK on backward (e.g. category has_many→food, food has_one).
		if strings.TrimSpace(utility.PhysicalSQLTableName(forwardModel.Name)) != strings.TrimSpace(utility.PhysicalSQLTableName(backwardModel.Name)) {
			return clone(backwardConn), clone(forwardConn)
		}
		// Same-named edge case (e.g. food_order has_many→employee): FK on forward holder; declaring model names.
		return &models.ConnectionType{
				Model:    forwardModel.Name,
				Relation: forwardConn.Relation,
				Type:     forwardConn.Type,
				KnownAs:  forwardConn.KnownAs,
			}, &models.ConnectionType{
				Model:    backwardModel.Name,
				Relation: backwardConn.Relation,
				Type:     backwardConn.Type,
				KnownAs:  backwardConn.KnownAs,
			}
	case forwardConn.Relation == "has_one" && backwardConn.Relation == "has_one",
		forwardConn.Relation == "has_many" && backwardConn.Relation == "has_many":
		return clone(forwardConn), clone(backwardConn)
	default:
		return nil, nil
	}
}

// schemaReverseConnection finds the reciprocal connection on toModel pointing at fromPhysicalName.
func schemaReverseConnection(toModel *models.ModelType, fromPhysicalName string, conn *models.ConnectionType) *models.ConnectionType {
	if toModel == nil || conn == nil {
		return nil
	}
	fromPhysicalName = strings.TrimSpace(fromPhysicalName)
	for _, back := range toModel.Connections {
		if back == nil {
			continue
		}
		if back.KnownAs != conn.KnownAs {
			continue
		}
		backKey := strings.TrimSpace(utility.PhysicalSQLTableName(back.Model))
		if backKey == fromPhysicalName {
			return back
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
