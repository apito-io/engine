package models

import "strings"

func argStringMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func filterString(field map[string]interface{}, key string) string {
	if field == nil {
		return ""
	}
	s, _ := field[key].(string)
	return strings.TrimSpace(s)
}

// SystemUserSearchQ returns the optional free-text q argument used by searchPlatformAccounts.
func (p *CommonSystemParams) SystemUserSearchQ() string {
	if p == nil || p.ResolveParams == nil || p.ResolveParams.Args == nil {
		return ""
	}
	s, _ := p.ResolveParams.Args["q"].(string)
	return strings.TrimSpace(s)
}

// HasSystemUserSearchFilter reports whether team search supplied a non-empty email/name filter.
// Empty eq strings do not count — otherwise GraphQL where: { email: { eq: "" } } would dump everyone.
func (p *CommonSystemParams) HasSystemUserSearchFilter() bool {
	if p == nil {
		return false
	}
	if strings.TrimSpace(p.Email) != "" {
		return true
	}
	if p.SystemUserSearchQ() != "" {
		return true
	}
	if p.ResolveParams == nil || p.ResolveParams.Args == nil {
		return false
	}
	where := argStringMap(p.ResolveParams.Args["where"])
	if where == nil {
		return false
	}
	for _, key := range []string{"email", "first_name", "last_name", "username"} {
		field := argStringMap(where[key])
		if filterString(field, "eq") != "" || filterString(field, "contains") != "" {
			return true
		}
	}
	return false
}

// SystemUserWhereEq returns where.<field>.eq from resolve args.
func (p *CommonSystemParams) SystemUserWhereEq(field string) string {
	if p == nil || p.ResolveParams == nil || p.ResolveParams.Args == nil {
		return ""
	}
	where := argStringMap(p.ResolveParams.Args["where"])
	return filterString(argStringMap(where[field]), "eq")
}

// SystemUserWhereContains returns where.<field>.contains from resolve args.
func (p *CommonSystemParams) SystemUserWhereContains(field string) string {
	if p == nil || p.ResolveParams == nil || p.ResolveParams.Args == nil {
		return ""
	}
	where := argStringMap(p.ResolveParams.Args["where"])
	return filterString(argStringMap(where[field]), "contains")
}

// SystemUserSearchLimitPage returns limit (capped) and 1-based page from GraphQL args.
func (p *CommonSystemParams) SystemUserSearchLimitPage(defaultLimit, maxLimit int) (limit, page int) {
	limit = defaultLimit
	page = 1
	if p == nil || p.ResolveParams == nil || p.ResolveParams.Args == nil {
		return limit, page
	}
	args := p.ResolveParams.Args
	if filter := argStringMap(args["filter"]); filter != nil {
		if v := intFromArg(filter["limit"]); v > 0 {
			limit = v
		}
		if v := intFromArg(filter["page"]); v > 0 {
			page = v
		}
	}
	if v := intFromArg(args["limit"]); v > 0 {
		limit = v
	}
	if v := intFromArg(args["page"]); v > 0 {
		page = v
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if page < 1 {
		page = 1
	}
	return limit, page
}

func intFromArg(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
