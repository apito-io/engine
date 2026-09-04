package plugin

import (
	"sort"
	"strings"

	"github.com/apito-io/types/protobuff"
	"google.golang.org/protobuf/types/known/structpb"
)

// Contributions is the signed + runtime plugin surface: API, UI, fields.
// Absence of a section means the plugin does not contribute that surface.
type Contributions struct {
	API    *APIContribution    `json:"api,omitempty" yaml:"api,omitempty"`
	UI     *UIContribution     `json:"ui,omitempty" yaml:"ui,omitempty"`
	Fields []FieldContribution `json:"fields,omitempty" yaml:"fields,omitempty"`
}

// APIContribution describes GraphQL, REST, and events.
type APIContribution struct {
	Scope     string          `json:"scope,omitempty" yaml:"scope,omitempty"` // system | project
	Queries   []APIOperation  `json:"queries,omitempty" yaml:"queries,omitempty"`
	Mutations []APIOperation  `json:"mutations,omitempty" yaml:"mutations,omitempty"`
	REST      []RESTOperation `json:"rest,omitempty" yaml:"rest,omitempty"`
	Events    []string        `json:"events,omitempty" yaml:"events,omitempty"`
}

// APIOperation is one GraphQL query or mutation.
type APIOperation struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Args        string `json:"args,omitempty" yaml:"args,omitempty"`
	Returns     string `json:"returns,omitempty" yaml:"returns,omitempty"`
}

// RESTOperation is one REST endpoint.
type RESTOperation struct {
	Method      string `json:"method" yaml:"method"`
	Path        string `json:"path" yaml:"path"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// UIContribution describes console routes, nav, and settings.
type UIContribution struct {
	Available    bool              `json:"ui_available" yaml:"ui_available"`
	Routes       []UIRoute         `json:"routes,omitempty" yaml:"routes,omitempty"`
	Navigation   []NavPlacement    `json:"navigation,omitempty" yaml:"navigation,omitempty"`
	Settings     []SettingsSurface `json:"settings,omitempty" yaml:"settings,omitempty"`
	ComponentIDs []string          `json:"component_ids,omitempty" yaml:"component_ids,omitempty"`
}

// UIRoute is a console plugin route.
type UIRoute struct {
	Path  string `json:"path" yaml:"path"`
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
}

// NavPlacement is an allowlisted sidebar insert.
type NavPlacement struct {
	After    string `json:"after,omitempty" yaml:"after,omitempty"`
	Before   string `json:"before,omitempty" yaml:"before,omitempty"`
	Group    string `json:"group,omitempty" yaml:"group,omitempty"`
	Settings bool   `json:"settings,omitempty" yaml:"settings,omitempty"`
	Hidden   bool   `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	Label    string `json:"label,omitempty" yaml:"label,omitempty"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	Icon     string `json:"icon,omitempty" yaml:"icon,omitempty"`
}

// SettingsSurface is a host settings page for the plugin.
type SettingsSurface struct {
	Path  string `json:"path,omitempty" yaml:"path,omitempty"`
	Label string `json:"label,omitempty" yaml:"label,omitempty"`
}

// FieldContribution is a model/content field renderer bound to a core storage type.
type FieldContribution struct {
	ID               string `json:"id" yaml:"id"`
	Label            string `json:"label" yaml:"label"`
	Icon             string `json:"icon,omitempty" yaml:"icon,omitempty"`
	StorageType      string `json:"storage_type" yaml:"storage_type"`
	FormComponent    string `json:"form_component,omitempty" yaml:"form_component,omitempty"`
	DisplayComponent string `json:"display_component,omitempty" yaml:"display_component,omitempty"`
	ValidationSchema string `json:"validation_schema,omitempty" yaml:"validation_schema,omitempty"`
	ContentForm      bool   `json:"content_form" yaml:"content_form"`
}

// RuntimeAPISnapshot is SchemaRegister / RESTApiRegister truth captured at load.
type RuntimeAPISnapshot struct {
	Queries   []APIOperation
	Mutations []APIOperation
	REST      []RESTOperation
}

var contribIndex = map[string]*Contributions{}

var allowedNavKeys = map[string]struct{}{
	"content": {}, "database": {}, "model": {}, "users": {}, "storage": {},
	"graphql": {}, "rest": {}, "auth": {}, "logic": {}, "plugins": {},
	"settings": {},
}

var allowedStorageTypes = map[string]struct{}{
	"text": {}, "media": {}, "object": {}, "multiline": {}, "date": {},
	"number": {}, "boolean": {}, "list": {}, "geo": {}, "repeated": {},
}

// SetPluginContributions records declared + merged contributions for a plugin id.
func SetPluginContributions(id string, c *Contributions) {
	id = strings.TrimSpace(id)
	capMu.Lock()
	defer capMu.Unlock()
	if c == nil || c.Empty() {
		delete(contribIndex, id)
		return
	}
	copied := c.Clone()
	contribIndex[id] = copied
}

// ContributionsFor returns merged contributions for a plugin id.
func ContributionsFor(id string) *Contributions {
	capMu.RLock()
	defer capMu.RUnlock()
	src := contribIndex[id]
	if src == nil {
		return nil
	}
	return src.Clone()
}

// MergeRuntimeContributions overlays live SchemaRegister/REST metadata onto declared contributions.
func MergeRuntimeContributions(id string, snap *RuntimeAPISnapshot) {
	id = strings.TrimSpace(id)
	if id == "" || snap == nil {
		return
	}
	capMu.Lock()
	defer capMu.Unlock()
	current := contribIndex[id]
	if current == nil {
		current = &Contributions{}
	}
	merged := MergeContributions(current, snap, nil)
	if merged == nil || merged.Empty() {
		delete(contribIndex, id)
		return
	}
	contribIndex[id] = merged
}

// MergeContributions prefers runtime API names, keeps declared UI/fields, fills defaults.
func MergeContributions(declared *Contributions, snap *RuntimeAPISnapshot, defaults *Contributions) *Contributions {
	out := &Contributions{}
	if defaults != nil {
		out = defaults.Clone()
	}
	if declared != nil {
		if declared.API != nil {
			out.API = mergeAPI(out.API, declared.API, false)
		}
		if declared.UI != nil {
			out.UI = declared.UI.clone()
		}
		if len(declared.Fields) > 0 {
			out.Fields = cloneFields(declared.Fields)
		}
	}
	if snap != nil && !snap.empty() {
		if out.API == nil {
			out.API = &APIContribution{}
		}
		if len(snap.Queries) > 0 {
			out.API.Queries = cloneOps(snap.Queries)
		}
		if len(snap.Mutations) > 0 {
			out.API.Mutations = cloneOps(snap.Mutations)
		}
		if len(snap.REST) > 0 {
			out.API.REST = cloneREST(snap.REST)
		}
	}
	if out.Empty() {
		return nil
	}
	return NormalizeContributions(out)
}

// NormalizeContributions drops unknown nav anchors and storage types.
func NormalizeContributions(c *Contributions) *Contributions {
	if c == nil {
		return nil
	}
	out := c.Clone()
	if out.UI != nil {
		var nav []NavPlacement
		for _, n := range out.UI.Navigation {
			if n.After != "" {
				if _, ok := allowedNavKeys[n.After]; !ok {
					n.After = ""
				}
			}
			if n.Before != "" {
				if _, ok := allowedNavKeys[n.Before]; !ok {
					n.Before = ""
				}
			}
			nav = append(nav, n)
		}
		out.UI.Navigation = nav
	}
	var fields []FieldContribution
	for _, f := range out.Fields {
		f.StorageType = strings.ToLower(strings.TrimSpace(f.StorageType))
		if f.StorageType == "" {
			f.StorageType = "text"
		}
		if _, ok := allowedStorageTypes[f.StorageType]; !ok {
			continue
		}
		fields = append(fields, f)
	}
	out.Fields = fields
	if out.API != nil && out.API.Scope != "" {
		scope := strings.ToLower(strings.TrimSpace(out.API.Scope))
		if scope != "system" && scope != "project" {
			out.API.Scope = ""
		} else {
			out.API.Scope = scope
		}
	}
	return out
}

// SnapshotRuntimeAPI extracts operation metadata from plugin register payloads.
func SnapshotRuntimeAPI(schemas *protobuff.ThirdPartyGraphQLSchemas, apis []*protobuff.ThirdPartyRESTApi) *RuntimeAPISnapshot {
	snap := &RuntimeAPISnapshot{}
	if schemas != nil {
		snap.Queries = graphqlOpsFromStruct(schemas.Queries)
		snap.Mutations = graphqlOpsFromStruct(schemas.Mutations)
	}
	for _, api := range apis {
		if api == nil {
			continue
		}
		snap.REST = append(snap.REST, RESTOperation{
			Method:      strings.ToUpper(strings.TrimSpace(api.Method)),
			Path:        api.Path,
			Description: api.Description,
		})
	}
	sort.Slice(snap.REST, func(i, j int) bool {
		if snap.REST[i].Method == snap.REST[j].Method {
			return snap.REST[i].Path < snap.REST[j].Path
		}
		return snap.REST[i].Method < snap.REST[j].Method
	})
	return snap
}

func graphqlOpsFromStruct(s *structpb.Struct) []APIOperation {
	if s == nil {
		return nil
	}
	m := s.AsMap()
	names := make([]string, 0, len(m))
	for name := range m {
		if name == "__objectTypes" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]APIOperation, 0, len(names))
	for _, name := range names {
		op := APIOperation{Name: name}
		if fieldMap, ok := m[name].(map[string]interface{}); ok {
			if d, ok := fieldMap["description"].(string); ok {
				op.Description = d
			}
			if t, ok := fieldMap["type"].(string); ok {
				op.Returns = t
			}
		}
		out = append(out, op)
	}
	return out
}

// Empty reports whether no surface is declared.
func (c *Contributions) Empty() bool {
	if c == nil {
		return true
	}
	if c.API != nil && (len(c.API.Queries) > 0 || len(c.API.Mutations) > 0 || len(c.API.REST) > 0 || len(c.API.Events) > 0) {
		return false
	}
	if c.UI != nil && (c.UI.Available || len(c.UI.Routes) > 0 || len(c.UI.Navigation) > 0 || len(c.UI.Settings) > 0) {
		return false
	}
	return len(c.Fields) == 0
}

// Clone deep-copies contributions.
func (c *Contributions) Clone() *Contributions {
	if c == nil {
		return nil
	}
	out := &Contributions{
		API:    mergeAPI(nil, c.API, true),
		UI:     c.UI.clone(),
		Fields: cloneFields(c.Fields),
	}
	return out
}

func (u *UIContribution) clone() *UIContribution {
	if u == nil {
		return nil
	}
	out := *u
	if u.Routes != nil {
		out.Routes = append([]UIRoute{}, u.Routes...)
	}
	if u.Navigation != nil {
		out.Navigation = append([]NavPlacement{}, u.Navigation...)
	}
	if u.Settings != nil {
		out.Settings = append([]SettingsSurface{}, u.Settings...)
	}
	if u.ComponentIDs != nil {
		out.ComponentIDs = append([]string{}, u.ComponentIDs...)
	}
	return &out
}

func mergeAPI(base, overlay *APIContribution, copyOnly bool) *APIContribution {
	if overlay == nil && base == nil {
		return nil
	}
	out := &APIContribution{}
	if base != nil {
		out.Scope = base.Scope
		out.Queries = cloneOps(base.Queries)
		out.Mutations = cloneOps(base.Mutations)
		out.REST = cloneREST(base.REST)
		out.Events = append([]string{}, base.Events...)
	}
	if overlay != nil && !copyOnly {
		if overlay.Scope != "" {
			out.Scope = overlay.Scope
		}
		if len(overlay.Queries) > 0 {
			out.Queries = cloneOps(overlay.Queries)
		}
		if len(overlay.Mutations) > 0 {
			out.Mutations = cloneOps(overlay.Mutations)
		}
		if len(overlay.REST) > 0 {
			out.REST = cloneREST(overlay.REST)
		}
		if overlay.Events != nil {
			out.Events = append([]string{}, overlay.Events...)
		}
	} else if overlay != nil && copyOnly {
		out.Scope = overlay.Scope
		out.Queries = cloneOps(overlay.Queries)
		out.Mutations = cloneOps(overlay.Mutations)
		out.REST = cloneREST(overlay.REST)
		out.Events = append([]string{}, overlay.Events...)
	}
	return out
}

func cloneOps(in []APIOperation) []APIOperation {
	if in == nil {
		return nil
	}
	return append([]APIOperation{}, in...)
}

func cloneREST(in []RESTOperation) []RESTOperation {
	if in == nil {
		return nil
	}
	return append([]RESTOperation{}, in...)
}

func cloneFields(in []FieldContribution) []FieldContribution {
	if in == nil {
		return nil
	}
	return append([]FieldContribution{}, in...)
}

func (s *RuntimeAPISnapshot) empty() bool {
	return s == nil || (len(s.Queries) == 0 && len(s.Mutations) == 0 && len(s.REST) == 0)
}
