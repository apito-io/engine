package resolver

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

var planSlugRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func normalizePlanSlug(raw string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	slug = strings.ReplaceAll(slug, "-", "_")
	slug = strings.ReplaceAll(slug, " ", "_")
	if !planSlugRe.MatchString(slug) {
		return "", fmt.Errorf("invalid plan id %q (use lowercase alphanumeric/underscore, start with a letter)", raw)
	}
	return slug, nil
}

func planToMap(p *models.Plan) map[string]interface{} {
	if p == nil {
		return nil
	}
	models.NormalizePlanMonetization(p)
	m := map[string]interface{}{
		"id":                p.ID,
		"name":              p.Name,
		"description":       p.Description,
		"system_generated":  p.SystemGenerated,
		"currency":          p.Currency,
		"price_monthly":     p.PriceMonthly,
		"play_product_id":   p.PlayProductID,
		"play_base_plan_id": p.PlayBasePlanID,
		"paddle_price_id":   p.PaddlePriceID,
	}
	if len(p.Prices) > 0 {
		prices := make([]map[string]interface{}, 0, len(p.Prices))
		for _, row := range p.Prices {
			prices = append(prices, map[string]interface{}{
				"currency": row.Currency,
				"amount":   row.Amount,
				"default":  row.Default,
			})
		}
		m["prices"] = prices
	}
	if len(p.ProviderProducts) > 0 {
		pps := make([]map[string]interface{}, 0, len(p.ProviderProducts))
		for _, row := range p.ProviderProducts {
			pps = append(pps, map[string]interface{}{
				"provider":   row.Provider,
				"product_id": row.ProductID,
				"variant_id": row.VariantID,
			})
		}
		m["provider_products"] = pps
	}
	if p.APIPermissions != nil {
		perms := make(map[string]interface{}, len(p.APIPermissions))
		for k, v := range p.APIPermissions {
			if v == nil {
				continue
			}
			perms[k] = map[string]interface{}{
				"read": v.Read, "create": v.Create, "update": v.Update, "delete": v.Delete,
			}
		}
		m["api_permissions"] = perms
	}
	if p.LogicExecutions != nil {
		m["logic_executions"] = p.LogicExecutions
	}
	if p.Quotas != nil {
		quotas := make(map[string]interface{}, len(p.Quotas))
		for k, v := range p.Quotas {
			quotas[k] = v
		}
		m["quotas"] = quotas
	}
	return m
}

func parsePlanPricesArg(raw interface{}) ([]models.PlanPrice, error) {
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("prices must be an array")
	}
	out := make([]models.PlanPrice, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		row := models.PlanPrice{
			Currency: strings.TrimSpace(fmt.Sprint(m["currency"])),
		}
		if row.Currency == "" || row.Currency == "<nil>" {
			row.Currency = "BDT"
		}
		switch n := m["amount"].(type) {
		case float64:
			row.Amount = n
		case int:
			row.Amount = float64(n)
		case int64:
			row.Amount = float64(n)
		}
		if d, ok := m["default"].(bool); ok {
			row.Default = d
		}
		out = append(out, row)
	}
	return out, nil
}

func parseProviderProductsArg(raw interface{}) ([]models.PlanProviderProduct, error) {
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("provider_products must be an array")
	}
	out := make([]models.PlanProviderProduct, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		row := models.PlanProviderProduct{
			Provider:  strings.TrimSpace(fmt.Sprint(m["provider"])),
			ProductID: strings.TrimSpace(fmt.Sprint(m["product_id"])),
			VariantID: strings.TrimSpace(fmt.Sprint(m["variant_id"])),
		}
		if row.Provider == "" || row.Provider == "<nil>" || row.ProductID == "" || row.ProductID == "<nil>" {
			continue
		}
		if row.VariantID == "<nil>" {
			row.VariantID = ""
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *GraphQLServer) GetProjectPlansResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	// CapPlansRead only — mobile/SDK login keys must load catalog (prices, provider_products)
	// without a full project-admin token. Upsert/delete still require plans.write + admin.
	if err := requireAccessCapability(router, CapPlansRead); err != nil {
		return nil, err
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if models.EnsureProjectPlansSeeds(cache.Project) {
		if err := s.SystemDriver.UpdateProject(cache.Ctx, cache.Project, true); err != nil {
			return nil, err
		}
	}
	out := make([]map[string]interface{}, 0, len(cache.Project.Plans))
	for _, plan := range cache.Project.Plans {
		if plan == nil {
			continue
		}
		out = append(out, planToMap(plan))
	}
	return out, nil
}

func (s *GraphQLServer) UpsertPlanToProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapPlansWrite); err != nil {
		return nil, err
	}
	s.injectMetaData("UpsertPlanToProjectResolverFn", router)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	project := cache.Project
	models.EnsureProjectPlansSeeds(project)

	slug, err := normalizePlanSlug(getArgString(p.Args, "id"))
	if err != nil {
		return nil, err
	}

	plan, existing := project.Plans[slug]
	if !existing || plan == nil {
		plan = &models.Plan{ID: slug, SystemGenerated: false}
		project.Plans[slug] = plan
	}
	plan.ID = slug
	// Only free is system-generated; Pro and other paid tiers stay operator-owned.
	if slug == "free" {
		plan.SystemGenerated = true
	} else {
		plan.SystemGenerated = false
	}
	models.ClearPlanSeedOmission(project, slug)

	if name := strings.TrimSpace(getArgString(p.Args, "name")); name != "" {
		plan.Name = name
	} else if plan.Name == "" {
		plan.Name = slug
	}
	if _, ok := p.Args["description"]; ok {
		plan.Description = getArgString(p.Args, "description")
	}

	if _, ok := p.Args["logic_executions"]; ok {
		plan.LogicExecutions = nil
		if logicExecutions, ok := p.Args["logic_executions"].([]interface{}); ok {
			plan.LogicExecutions = make([]string, 0, len(logicExecutions))
			for _, l := range logicExecutions {
				if s, ok := l.(string); ok {
					plan.LogicExecutions = append(plan.LogicExecutions, s)
				}
			}
		}
	}

	if val, ok := p.Args["api_permissions"].(map[string]interface{}); ok {
		permissions := make(map[string]*models.APIPermission)
		for k, vv := range val {
			pm, ok := vv.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid api_permissions for %s", k)
			}
			validated, err := utility.ValidatePermissions(pm)
			if err != nil {
				return nil, err
			}
			permissions[k] = validated
		}
		plan.APIPermissions = permissions
	}

	if val, ok := p.Args["quotas"].(map[string]interface{}); ok {
		quotas := make(map[string]int, len(val))
		for k, vv := range val {
			switch n := vv.(type) {
			case int:
				quotas[k] = n
			case int64:
				quotas[k] = int(n)
			case float64:
				quotas[k] = int(n)
			default:
				return nil, fmt.Errorf("invalid quota value for %s", k)
			}
		}
		plan.Quotas = quotas
	}

	if _, ok := p.Args["prices"]; ok {
		prices, err := parsePlanPricesArg(p.Args["prices"])
		if err != nil {
			return nil, err
		}
		plan.Prices = prices
	}
	if _, ok := p.Args["provider_products"]; ok {
		pps, err := parseProviderProductsArg(p.Args["provider_products"])
		if err != nil {
			return nil, err
		}
		plan.ProviderProducts = pps
	}
	if _, ok := p.Args["currency"]; ok {
		plan.Currency = strings.TrimSpace(getArgString(p.Args, "currency"))
	}
	if _, ok := p.Args["price_monthly"]; ok {
		switch n := p.Args["price_monthly"].(type) {
		case float64:
			plan.PriceMonthly = n
		case int:
			plan.PriceMonthly = float64(n)
		case int64:
			plan.PriceMonthly = float64(n)
		}
	}
	if _, ok := p.Args["play_product_id"]; ok {
		plan.PlayProductID = strings.TrimSpace(getArgString(p.Args, "play_product_id"))
	}
	if _, ok := p.Args["play_base_plan_id"]; ok {
		plan.PlayBasePlanID = strings.TrimSpace(getArgString(p.Args, "play_base_plan_id"))
	}
	if _, ok := p.Args["paddle_price_id"]; ok {
		plan.PaddlePriceID = strings.TrimSpace(getArgString(p.Args, "paddle_price_id"))
	}
	// If only legacy fields were sent, synthesize arrays; always sync both directions.
	if _, ok := p.Args["prices"]; !ok {
		if plan.Currency != "" || plan.PriceMonthly > 0 {
			if len(plan.Prices) == 0 {
				cur := plan.Currency
				if cur == "" {
					cur = "BDT"
				}
				plan.Prices = []models.PlanPrice{{Currency: cur, Amount: plan.PriceMonthly, Default: true}}
			}
		}
	}
	if _, ok := p.Args["provider_products"]; !ok {
		if len(plan.ProviderProducts) == 0 && (plan.PlayProductID != "" || plan.PaddlePriceID != "") {
			models.NormalizePlanMonetization(plan)
		}
	}
	models.SyncPlanLegacyMonetization(plan)

	if err := s.SystemDriver.UpdateProject(cache.Ctx, project, true); err != nil {
		return nil, err
	}
	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}
	return planToMap(plan), nil
}

func (s *GraphQLServer) DuplicatePlanInProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapPlansWrite); err != nil {
		return nil, err
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	project := cache.Project
	models.EnsureProjectPlansSeeds(project)

	sourceKey, err := normalizePlanSlug(getArgString(p.Args, "source_id"))
	if err != nil {
		return nil, err
	}
	newKey, err := normalizePlanSlug(getArgString(p.Args, "new_id"))
	if err != nil {
		return nil, err
	}
	src, ok := project.Plans[sourceKey]
	if !ok || src == nil {
		return nil, errors.New("source plan not found")
	}
	if _, exists := project.Plans[newKey]; exists {
		return nil, errors.New("a plan with that id already exists")
	}
	cp := clonePlan(src)
	cp.ID = newKey
	cp.SystemGenerated = false
	if name := strings.TrimSpace(getArgString(p.Args, "name")); name != "" {
		cp.Name = name
	} else {
		cp.Name = newKey
	}
	project.Plans[newKey] = cp
	if err := s.SystemDriver.UpdateProject(cache.Ctx, project, true); err != nil {
		return nil, err
	}
	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}
	return planToMap(cp), nil
}

func (s *GraphQLServer) DeletePlanFromProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapPlansWrite); err != nil {
		return nil, err
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	project := cache.Project
	models.EnsureProjectPlansSeeds(project)

	slug, err := normalizePlanSlug(getArgString(p.Args, "id"))
	if err != nil {
		return nil, err
	}
	plan, ok := project.Plans[slug]
	if !ok || plan == nil {
		return nil, errors.New("plan not found")
	}
	if slug == "free" {
		return nil, errors.New("cannot delete the free plan (required default entitlement)")
	}
	if s.Cfg != nil && s.Cfg.PlanDeleteGuardHook != nil {
		if err := s.Cfg.PlanDeleteGuardHook(cache.Ctx, project.ID, slug); err != nil {
			return nil, err
		}
	}
	delete(project.Plans, slug)
	if plan.SystemGenerated {
		models.MarkPlanSeedOmitted(project, slug)
	}
	if err := s.SystemDriver.UpdateProject(cache.Ctx, project, true); err != nil {
		return nil, err
	}
	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(project.Plans))
	for _, pl := range project.Plans {
		out = append(out, planToMap(pl))
	}
	return out, nil
}

func clonePlan(src *models.Plan) *models.Plan {
	if src == nil {
		return &models.Plan{}
	}
	models.NormalizePlanMonetization(src)
	dst := &models.Plan{
		ID:             src.ID,
		Name:           src.Name,
		Description:    src.Description,
		SystemGenerated: src.SystemGenerated,
		Currency:       src.Currency,
		PriceMonthly:   src.PriceMonthly,
		PlayProductID:  src.PlayProductID,
		PlayBasePlanID: src.PlayBasePlanID,
		PaddlePriceID:  src.PaddlePriceID,
	}
	if len(src.LogicExecutions) > 0 {
		dst.LogicExecutions = append([]string(nil), src.LogicExecutions...)
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
	if src.Quotas != nil {
		dst.Quotas = make(map[string]int, len(src.Quotas))
		for k, v := range src.Quotas {
			dst.Quotas[k] = v
		}
	}
	if len(src.Prices) > 0 {
		dst.Prices = append([]models.PlanPrice(nil), src.Prices...)
	}
	if len(src.ProviderProducts) > 0 {
		dst.ProviderProducts = append([]models.PlanProviderProduct(nil), src.ProviderProducts...)
	}
	return dst
}
