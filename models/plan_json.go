package models

import "encoding/json"

// planPublicKeys are the Plan fields this module models. Every other object
// key is preserved on Plan.Host so a later save does not drop host data.
var planPublicKeys = map[string]struct{}{
	"id":               {},
	"name":             {},
	"description":      {},
	"api_permissions":  {},
	"logic_executions": {},
	"quotas":           {},
	"system_generated": {},
}

type planJSONCore struct {
	ID              string                    `json:"id,omitempty"`
	Name            string                    `json:"name,omitempty"`
	Description     string                    `json:"description,omitempty"`
	APIPermissions  map[string]*APIPermission `json:"api_permissions,omitempty"`
	LogicExecutions []string                  `json:"logic_executions,omitempty"`
	Quotas          map[string]int            `json:"quotas,omitempty"`
	SystemGenerated bool                      `json:"system_generated,omitempty"`
}

// UnmarshalJSON loads the public permission fields and keeps every other key.
func (p *Plan) UnmarshalJSON(b []byte) error {
	if p == nil {
		return nil
	}
	var core planJSONCore
	if err := json.Unmarshal(b, &core); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	rest := map[string]json.RawMessage{}
	for k, v := range raw {
		if _, known := planPublicKeys[k]; known {
			continue
		}
		rest[k] = v
	}
	*p = Plan{
		ID:              core.ID,
		Name:            core.Name,
		Description:     core.Description,
		APIPermissions:  core.APIPermissions,
		LogicExecutions: core.LogicExecutions,
		Quotas:          core.Quotas,
		SystemGenerated: core.SystemGenerated,
	}
	if len(rest) > 0 {
		hb, err := json.Marshal(rest)
		if err != nil {
			return err
		}
		p.Host = hb
	}
	return nil
}

// MarshalJSON writes public fields and merges preserved host keys at the top level.
func (p Plan) MarshalJSON() ([]byte, error) {
	core, err := json.Marshal(planJSONCore{
		ID:              p.ID,
		Name:            p.Name,
		Description:     p.Description,
		APIPermissions:  p.APIPermissions,
		LogicExecutions: p.LogicExecutions,
		Quotas:          p.Quotas,
		SystemGenerated: p.SystemGenerated,
	})
	if err != nil {
		return nil, err
	}
	var base map[string]interface{}
	if err := json.Unmarshal(core, &base); err != nil {
		return nil, err
	}
	if base == nil {
		base = map[string]interface{}{}
	}
	if len(p.Host) > 0 {
		var extra map[string]interface{}
		if err := json.Unmarshal(p.Host, &extra); err != nil {
			return nil, err
		}
		for k, v := range extra {
			if _, known := planPublicKeys[k]; known {
				continue
			}
			base[k] = v
		}
	}
	return json.Marshal(base)
}
