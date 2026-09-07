package executor

import (
	"context"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/tailor-platform/graphql"
)

func commonDocIsLive(doc *types.DefaultDocumentStructure) bool {
	if doc == nil {
		return false
	}
	if doc.Meta != nil && strings.EqualFold(strings.TrimSpace(doc.Meta.Status), "deleted") {
		return false
	}
	return true
}

func collectDocIDsFromInterface(v interface{}) []string {
	var ids []string
	switch t := v.(type) {
	case *types.DefaultDocumentStructure:
		if t != nil && strings.TrimSpace(t.ID) != "" {
			ids = append(ids, t.ID)
		}
	case []*types.DefaultDocumentStructure:
		for _, d := range t {
			ids = append(ids, collectDocIDsFromInterface(d)...)
		}
	case map[string]interface{}:
		if id, ok := t["id"].(string); ok && strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
	case []interface{}:
		for _, item := range t {
			ids = append(ids, collectDocIDsFromInterface(item)...)
		}
	case []map[string]interface{}:
		for _, item := range t {
			ids = append(ids, collectDocIDsFromInterface(item)...)
		}
	}
	return ids
}

func uniqueNonEmpty(ids []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// LoadCommonDocsByIDs fetches common-model rows from the base project DB.
func (s *GraphQLExecutor) LoadCommonDocsByIDs(ctx context.Context, param *models.CommonSystemParams, ids []string) (map[string]*types.DefaultDocumentStructure, error) {
	ids = uniqueNonEmpty(ids)
	out := make(map[string]*types.DefaultDocumentStructure, len(ids))
	if s == nil || param == nil || param.Model == nil || len(ids) == 0 {
		return out, nil
	}
	if !models.ParamPerTenantSeparateDB(param) || !models.ModelIsCommon(param.Model) {
		return out, nil
	}
	driver, err := s.GetProjectDriver(models.WithBaseProjectDB(ctx))
	if err != nil {
		return nil, err
	}
	hyd := *param
	hyd.DocumentIDs = ids
	hyd.ResolveParams = &graphql.ResolveParams{
		Args: map[string]interface{}{
			"limit": len(ids),
			"page":  1,
		},
	}
	docs, err := driver.QueryMultiDocumentOfProject(models.WithBaseProjectDB(ctx), &hyd)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if !commonDocIsLive(doc) {
			continue
		}
		out[doc.ID] = doc
	}
	return out, nil
}

func applyCommonHydrationToDoc(doc *types.DefaultDocumentStructure, byID map[string]*types.DefaultDocumentStructure) *types.DefaultDocumentStructure {
	if doc == nil {
		return nil
	}
	src := byID[doc.ID]
	if src == nil {
		return nil
	}
	doc.Data = src.Data
	if src.Meta != nil {
		doc.Meta = src.Meta
	}
	if src.Type != "" {
		doc.Type = src.Type
	}
	return doc
}

func applyCommonHydrationToMap(m map[string]interface{}, byID map[string]*types.DefaultDocumentStructure) map[string]interface{} {
	if m == nil {
		return nil
	}
	id, _ := m["id"].(string)
	src := byID[id]
	if src == nil {
		return nil
	}
	if src.Data != nil {
		m["data"] = src.Data
	}
	if src.Meta != nil {
		m["meta"] = src.Meta
	}
	if src.Type != "" {
		m["type"] = src.Type
	}
	return m
}

// HydrateCommonRelationMaps hydrates executor DataLoaderHandler JSON maps.
func (s *GraphQLExecutor) HydrateCommonRelationMaps(ctx context.Context, param *models.CommonSystemParams, keyed map[string]interface{}) (map[string]interface{}, error) {
	if s == nil || param == nil || keyed == nil || !models.ParamPerTenantSeparateDB(param) || !models.ModelIsCommon(param.Model) {
		return keyed, nil
	}
	ids := collectDocIDsFromInterface(keyed)
	byID, err := s.LoadCommonDocsByIDs(ctx, param, ids)
	if err != nil {
		return nil, err
	}
	for k, raw := range keyed {
		switch t := raw.(type) {
		case map[string]interface{}:
			keyed[k] = applyCommonHydrationToMap(t, byID)
		case []interface{}:
			var kept []interface{}
			for _, item := range t {
				if m, ok := item.(map[string]interface{}); ok {
					if hyd := applyCommonHydrationToMap(m, byID); hyd != nil {
						kept = append(kept, hyd)
					}
				}
			}
			keyed[k] = kept
		}
	}
	return keyed, nil
}

// HydrateCommonDocuments fills Data on tenant shadow rows from the base copy.
func (s *GraphQLExecutor) HydrateCommonDocuments(ctx context.Context, param *models.CommonSystemParams, docs []*types.DefaultDocumentStructure) ([]*types.DefaultDocumentStructure, error) {
	if s == nil || param == nil || !models.ParamPerTenantSeparateDB(param) || !models.ModelIsCommon(param.Model) {
		return docs, nil
	}
	ids := collectDocIDsFromInterface(docs)
	byID, err := s.LoadCommonDocsByIDs(ctx, param, ids)
	if err != nil {
		return nil, err
	}
	var kept []*types.DefaultDocumentStructure
	for _, d := range docs {
		if hyd := applyCommonHydrationToDoc(d, byID); hyd != nil {
			kept = append(kept, hyd)
		}
	}
	return kept, nil
}
