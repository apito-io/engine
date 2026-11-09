package resolver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

const extKeyQuotaCountCache = "_plan_quota_counts"

// enforcePlanCreateQuota blocks create when the tenant plan's max_records.<model> is reached.
// Counts are cached on param.Ext for the request so batch creates only count once.
func (s *GraphQLServer) enforcePlanCreateQuota(ctx context.Context, cache *models.ApplicationCache, param *models.CommonSystemParams, modelName string) error {
	if param == nil || param.ActivePlan == nil || s == nil || s.GraphQLExecutor == nil {
		return nil
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil
	}
	key := utility.RecordsQuotaKey(modelName)
	parsed, err := utility.ParsePlanQuota(param.ActivePlan, key)
	if err != nil {
		return err
	}
	if parsed.Window == utility.QuotaWindowUnlimited {
		return nil
	}
	from, until := "", ""
	if parsed.Window == utility.QuotaWindowMonthly {
		from, until = utility.DhakaMonthBounds(time.Now())
	}
	count, err := s.cachedModelDocCount(ctx, cache, param, modelName, from, until)
	if err != nil {
		return fmt.Errorf("plan quota check: %w", err)
	}
	return utility.CheckCountedQuota(parsed, key, count)
}

func (s *GraphQLServer) cachedModelDocCount(ctx context.Context, cache *models.ApplicationCache, param *models.CommonSystemParams, modelName, from, until string) (int, error) {
	if s == nil || param == nil {
		return 0, nil
	}
	if param.Ext == nil {
		param.Ext = make(map[string]interface{})
	}
	raw, _ := param.Ext[extKeyQuotaCountCache].(map[string]int)
	if raw == nil {
		raw = make(map[string]int)
		param.Ext[extKeyQuotaCountCache] = raw
	}
	cacheKey := modelName
	if from != "" {
		cacheKey = modelName + "@" + from
	}
	if c, ok := raw[cacheKey]; ok {
		return c, nil
	}
	var modelType *models.ModelType
	if cache != nil && cache.Project != nil && cache.Project.Schema != nil {
		for _, m := range cache.Project.Schema.Models {
			if m != nil && utility.ModelIDMatchesGraphQLField(m.Name, modelName) {
				modelType = m
				break
			}
		}
	}
	if modelType == nil {
		raw[cacheKey] = 0
		return 0, nil
	}
	if s.GraphQLExecutor == nil {
		raw[cacheKey] = 0
		return 0, nil
	}
	countParam := s.NewParam(param)
	countParam.Model = modelType
	countParam.OnlyReturnCount = true
	countParam.QuotaCountFrom = from
	countParam.QuotaCountUntil = until
	if cache != nil && cache.Project != nil && cache.Project.Schema != nil {
		countParam.ProjectSchemaModels = cache.Project.Schema.Models
	}
	driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return 0, err
	}
	if driver == nil {
		raw[cacheKey] = 0
		return 0, nil
	}
	result, err := driver.CountDocOfProject(ctx, countParam)
	if err != nil {
		return 0, err
	}
	n := 0
	switch v := result.(type) {
	case int:
		n = v
	case int64:
		n = int(v)
	case float64:
		n = int(v)
	case map[string]interface{}:
		if c, ok := v["count"].(int); ok {
			n = c
		} else if c, ok := v["count"].(float64); ok {
			n = int(c)
		} else if c, ok := v["count"].(int64); ok {
			n = int(c)
		}
	}
	raw[cacheKey] = n
	return n, nil
}
