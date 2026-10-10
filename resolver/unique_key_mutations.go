package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	schemasvc "github.com/apito-io/engine/services/schema"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

type uniqueKeyDDL interface {
	ApplyUniqueKeyDDL(ctx context.Context, modelsList []*models.ModelType, drops []models.UniqueIndexDrop) error
}

func (s *GraphQLServer) UpsertUniqueKeyResolverFn(p graphql.ResolveParams) (interface{}, error) {
	return s.uniqueKeyMutation(p, false)
}

func (s *GraphQLServer) DeleteUniqueKeyResolverFn(p graphql.ResolveParams) (interface{}, error) {
	return s.uniqueKeyMutation(p, true)
}

func (s *GraphQLServer) uniqueKeyMutation(p graphql.ResolveParams, deleteKey bool) (interface{}, error) {
	v := p.Context.Value
	router := v("router").(echo.Context)
	if err := requireAccessCapability(router, CapSchemaWrite); err != nil {
		return nil, err
	}
	s.injectMetaData("uniqueKeyMutation", router)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	project := cache.Project
	modelName, err := canonicalModelArg(p.Args, "model_name")
	if err != nil {
		return nil, err
	}
	keyName := strings.TrimSpace(stringArgRaw(p.Args, "name"))
	if keyName == "" {
		return nil, fmt.Errorf("unique key name is required")
	}
	opType := models.SchemaOpTypeUpsertUniqueKey
	if deleteKey {
		opType = models.SchemaOpTypeDeleteUniqueKey
	}
	args := graphqlArgsMap(p)
	args["model_name"] = modelName
	args["name"] = keyName
	if !deleteKey {
		args["columns"] = stringListArg(p.Args, "columns")
	}
	return s.tryStageSchemaMutation(cache, project, opType, args, func() (interface{}, error) {
		if project == nil || project.Schema == nil {
			return nil, fmt.Errorf("project schema is empty")
		}
		model := findSchemaModelByName(project.Schema.Models, modelName)
		if model == nil {
			return nil, fmt.Errorf("model %q not found", modelName)
		}
		var key *models.UniqueKey
		var drops []models.UniqueIndexDrop
		if deleteKey {
			prev := takeUniqueKey(model, keyName)
			if prev == nil {
				return nil, fmt.Errorf("unique key %q not found on %s", keyName, modelName)
			}
			drops = append(drops, uniqueIndexDrop(model.Name, prev.Name))
			key = prev
		} else {
			validated, err := utility.ValidateUniqueKey(model, keyName, stringListArg(args, "columns"))
			if err != nil {
				return nil, err
			}
			if prev := takeUniqueKey(model, validated.Name); prev != nil && !sameColumns(prev.Columns, validated.Columns) {
				drops = append(drops, uniqueIndexDrop(model.Name, prev.Name))
			}
			model.UniqueKeys = append(model.UniqueKeys, validated)
			key = validated
		}
		driver, skipBaseDDL, err := s.getSchemaBaseProjectDriverIfNeeded(cache.Ctx, project, model)
		if err != nil {
			return nil, err
		}
		err = s.runSchemaChange(cache.Ctx, schemasvc.RunInput{
			Ctx: cache.Ctx, Project: project,
			OperationType: opType,
			Request:       args,
			BaseDriver:    driver, SkipBaseDDL: skipBaseDDL,
			ApplyDDL: func(ctx context.Context, d interfaces.ProjectDBInterface) error {
				applier, ok := d.(uniqueKeyDDL)
				if !ok {
					return nil
				}
				return applier.ApplyUniqueKeyDDL(ctx, project.Schema.Models, drops)
			},
			PersistSystem: func() error {
				if err := s.SystemDriver.TouchProjectUpdatedAt(cache.Ctx, project.ID); err != nil {
					return err
				}
				return s.SystemDriver.UpsertModelType(cache.Ctx, project.ID, model)
			},
			RefreshCache: func() error {
				_, err := s.refreshProjectAndReCache(cache.Ctx, project.ID)
				return err
			},
		})
		if err != nil {
			return nil, err
		}
		return uniqueKeyResponse(model.Name, key, false, ""), nil
	})
}

func canonicalModelArg(args map[string]interface{}, key string) (string, error) {
	raw := strings.TrimSpace(stringArgRaw(args, key))
	if raw == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	name, err := utility.CanonicalizeModelName(raw)
	if err != nil {
		name, err = utility.LegacyStoredNameToCanonical(raw)
		if err != nil {
			return "", err
		}
	}
	return name, nil
}

func stringArgRaw(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func stringListArg(args map[string]interface{}, key string) []string {
	if args == nil {
		return nil
	}
	switch v := args[key].(type) {
	case []string:
		return append([]string(nil), v...)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			} else if item != nil {
				out = append(out, fmt.Sprintf("%v", item))
			}
		}
		return out
	default:
		return nil
	}
}

func takeUniqueKey(m *models.ModelType, name string) *models.UniqueKey {
	if m == nil {
		return nil
	}
	name = strings.ToLower(strings.TrimSpace(name))
	out := make([]*models.UniqueKey, 0, len(m.UniqueKeys))
	var found *models.UniqueKey
	for _, k := range m.UniqueKeys {
		if k == nil {
			continue
		}
		if strings.EqualFold(k.Name, name) && found == nil {
			found = k
			continue
		}
		out = append(out, k)
	}
	m.UniqueKeys = out
	return found
}

func sameColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func uniqueIndexDrop(modelName, keyName string) models.UniqueIndexDrop {
	table := utility.PhysicalSQLTableName(modelName)
	return models.UniqueIndexDrop{Table: table, Name: models.CompositeUniqueIndexName(table, strings.ToLower(strings.TrimSpace(keyName)))}
}

func uniqueKeyResponse(modelName string, key *models.UniqueKey, staged bool, changesetID string) map[string]interface{} {
	name, cols := "", []string(nil)
	if key != nil {
		name = key.Name
		cols = key.Columns
	}
	msg := ""
	if staged {
		msg = "Staged; not published"
	}
	return map[string]interface{}{
		"name":         name,
		"columns":      cols,
		"model_name":   modelName,
		"staged":       staged,
		"changeset_id": changesetID,
		"message":      msg,
	}
}
