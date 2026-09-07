package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-platform/graphql"
)

const commonRelationFilterIDCap = 10000
const commonFilterNoMatchID = "__apito_no_match__"

func relationFilterHasFieldPredicates(filterMap map[string]interface{}) bool {
	for k := range filterMap {
		if k != "_id" {
			return true
		}
	}
	return false
}

func findSchemaModelByName(modelsList []*models.ModelType, name string) *models.ModelType {
	name = strings.TrimSpace(name)
	for _, m := range modelsList {
		if m != nil && (utility.ModelIDMatchesGraphQLField(m.Name, name) || strings.EqualFold(m.Name, name)) {
			return m
		}
	}
	return nil
}

func connectionForRelationKey(model *models.ModelType, key string) *models.ConnectionType {
	if model == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	for _, c := range model.Connections {
		if c == nil {
			continue
		}
		if strings.EqualFold(c.KnownAs, key) || utility.ModelIDMatchesGraphQLField(c.Model, key) || strings.EqualFold(c.Model, key) {
			return c
		}
	}
	return nil
}

// rewriteCommonModelRelationFilters resolves common-model field filters against the
// base project DB, then replaces them with an IN-list of ids for the tenant query.
func (s *GraphQLServer) prepareCommonModelQuery(ctx context.Context, param *models.CommonSystemParams) error {
	if err := rejectUnsupportedCommonRelationSort(param); err != nil {
		return err
	}
	return s.rewriteCommonModelRelationFilters(ctx, param)
}

func rejectUnsupportedCommonRelationSort(param *models.CommonSystemParams) error {
	if param == nil || param.ResolveParams == nil || param.ResolveParams.Args == nil {
		return nil
	}
	raw, ok := param.ResolveParams.Args["sort"].(map[string]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	local := map[string]struct{}{}
	if param.Model != nil {
		for _, f := range param.Model.Fields {
			if f != nil && strings.TrimSpace(f.Identifier) != "" {
				local[f.Identifier] = struct{}{}
			}
		}
	}
	for key := range raw {
		if _, ok := local[key]; ok {
			continue
		}
		conn := connectionForRelationKey(param.Model, key)
		if conn == nil {
			continue
		}
		related := findSchemaModelByName(param.ProjectSchemaModels, conn.Model)
		if models.ModelIsCommon(related) {
			return fmt.Errorf("sorting by common-model relation %q is not supported", key)
		}
	}
	return nil
}

func (s *GraphQLServer) rewriteCommonModelRelationFilters(ctx context.Context, param *models.CommonSystemParams) error {
	if s == nil || param == nil || param.ResolveParams == nil || param.ResolveParams.Args == nil {
		return nil
	}
	if !models.ParamPerTenantSeparateDB(param) || models.ModelIsCommon(param.Model) {
		return nil
	}
	relationArg, ok := param.ResolveParams.Args["relation"].(map[string]interface{})
	if !ok || len(relationArg) == 0 {
		return nil
	}
	for relationKey, rawFilter := range relationArg {
		filterMap, ok := rawFilter.(map[string]interface{})
		if !ok || !relationFilterHasFieldPredicates(filterMap) {
			continue
		}
		conn := connectionForRelationKey(param.Model, relationKey)
		if conn == nil {
			continue
		}
		related := findSchemaModelByName(param.ProjectSchemaModels, conn.Model)
		if !models.ModelIsCommon(related) {
			continue
		}
		fieldFilters := make(map[string]interface{}, len(filterMap))
		for k, v := range filterMap {
			if k == "_id" {
				continue
			}
			fieldFilters[k] = v
		}
		ids, err := s.resolveCommonModelIDs(ctx, param, related, fieldFilters)
		if err != nil {
			return err
		}
		if existing, ok := filterMap["_id"]; ok {
			ids = intersectIDFilter(existing, ids)
		}
		if len(ids) == 0 {
			ids = []string{commonFilterNoMatchID}
		}
		relationArg[relationKey] = map[string]interface{}{
			"_id": map[string]interface{}{"in": ids},
		}
	}
	return nil
}

func intersectIDFilter(existing interface{}, ids []string) []string {
	allowed := map[string]struct{}{}
	switch t := existing.(type) {
	case map[string]interface{}:
		if eq, ok := t["eq"].(string); ok {
			allowed[eq] = struct{}{}
		}
		switch in := t["in"].(type) {
		case []string:
			for _, id := range in {
				allowed[id] = struct{}{}
			}
		case []interface{}:
			for _, v := range in {
				if id, ok := v.(string); ok {
					allowed[id] = struct{}{}
				}
			}
		}
	}
	if len(allowed) == 0 {
		return ids
	}
	var out []string
	for _, id := range ids {
		if _, ok := allowed[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func (s *GraphQLServer) resolveCommonModelIDs(ctx context.Context, parent *models.CommonSystemParams, related *models.ModelType, where map[string]interface{}) ([]string, error) {
	relParam := *parent
	relParam.Model = related
	relParam.DocumentIDs = nil
	relParam.ResolveParams = &graphql.ResolveParams{
		Args: map[string]interface{}{
			"where": where,
			"limit": commonRelationFilterIDCap + 1,
			"page":  1,
		},
	}
	driver, err := s.GraphQLExecutor.GetProjectDriver(models.WithBaseProjectDB(ctx))
	if err != nil {
		return nil, err
	}
	docs, err := driver.QueryMultiDocumentOfProject(models.WithBaseProjectDB(ctx), &relParam)
	if err != nil {
		return nil, err
	}
	if len(docs) > commonRelationFilterIDCap {
		return nil, fmt.Errorf("common-model filter on %q matched more than %d rows; narrow the filter", related.Name, commonRelationFilterIDCap)
	}
	ids := make([]string, 0, len(docs))
	for _, d := range docs {
		if d != nil && strings.TrimSpace(d.ID) != "" {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}
