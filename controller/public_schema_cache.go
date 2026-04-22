package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"

	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

// preConnectionShape holds GraphQL maps built before relation fields and resolvers are attached.
// It is safe to cache when the fingerprint matches (same project schema view + role,
// unless ROLE_AGNOSTIC_SCHEMA_CACHE is on, in which case the fingerprint omits role).
type preConnectionShape struct {
	commonFields                     map[string]graphql.Fields
	aggregateFields                  map[string]graphql.Fields
	whereArgs                        map[string]graphql.InputObjectConfigFieldMap
	sortParam                        map[string]graphql.InputObjectConfigFieldMap
	connectionParamArgs              map[string]*graphql.InputObject
	whereRelationArgs                map[string]*graphql.InputObject
	createMutationFieldsArguments    map[string]graphql.InputObjectConfigFieldMap
	updateMutationFieldsArguments    map[string]graphql.InputObjectConfigFieldMap
	connectionFields                 map[string]graphql.InputObjectConfigFieldMap
	commonMutationFieldsConfigArgs   map[string]graphql.FieldConfigArgument
}

const maxCompiledSchemaCacheEntries = 1000

var compiledSchemaLRU = struct {
	mu      sync.Mutex
	order   []string
	entries map[string]*preConnectionShape
}{
	entries: make(map[string]*preConnectionShape),
}

func fingerprintPreConnection(
	project *models.Project,
	role *models.Role,
	incomingRequest []*models.IncomingRequest,
) string {
	h := sha256.New()
	_, _ = h.Write([]byte(project.ID))
	if project.Schema != nil {
		type m struct {
			Name        string   `json:"n"`
			FieldIDs    []string `json:"f"`
			Connections []string `json:"c"`
		}
		var rows []m
		for _, mod := range project.Schema.Models {
			if mod == nil {
				continue
			}
			var fids []string
			for _, fi := range mod.Fields {
				if fi != nil {
					fids = append(fids, fi.Identifier)
				}
			}
			sort.Strings(fids)
			var cstr []string
			for _, ct := range mod.Connections {
				if ct == nil {
					continue
				}
				cstr = append(cstr, ct.Model+":"+ct.Relation+":"+ct.KnownAs)
			}
			sort.Strings(cstr)
			rows = append(rows, m{Name: mod.Name, FieldIDs: fids, Connections: cstr})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
		b, _ := json.Marshal(rows)
		_, _ = h.Write(b)
	}
	if role != nil {
		roleBlob, _ := json.Marshal(struct {
			IsAdmin         bool                            `json:"a"`
			LogicExec       []string                        `json:"l"`
			APIPermKeys     []string                        `json:"k"`
			APIPermValues   map[string]*models.APIPermission `json:"p"`
		}{
			IsAdmin:   role.IsAdmin,
			LogicExec: role.LogicExecutions,
			APIPermKeys: func() []string {
				var ks []string
				for k := range role.APIPermissions {
					ks = append(ks, k)
				}
				sort.Strings(ks)
				return ks
			}(),
			APIPermValues: role.APIPermissions,
		})
		_, _ = h.Write(roleBlob)
	}
	if len(incomingRequest) > 0 {
		ir, _ := json.Marshal(incomingRequest)
		_, _ = h.Write(ir)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func getPreConnectionFromCache(key string) (*preConnectionShape, bool) {
	compiledSchemaLRU.mu.Lock()
	defer compiledSchemaLRU.mu.Unlock()
	shape, ok := compiledSchemaLRU.entries[key]
	return shape, ok
}

func putPreConnectionInCache(key string, shape *preConnectionShape) {
	compiledSchemaLRU.mu.Lock()
	defer compiledSchemaLRU.mu.Unlock()

	if _, exists := compiledSchemaLRU.entries[key]; exists {
		compiledSchemaLRU.entries[key] = shape
		return
	}
	if len(compiledSchemaLRU.order) >= maxCompiledSchemaCacheEntries {
		evict := compiledSchemaLRU.order[0]
		compiledSchemaLRU.order = compiledSchemaLRU.order[1:]
		delete(compiledSchemaLRU.entries, evict)
	}
	compiledSchemaLRU.order = append(compiledSchemaLRU.order, key)
	compiledSchemaLRU.entries[key] = shape
}

func checkCtxDone(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func clonePreConnectionShape(s *preConnectionShape) *preConnectionShape {
	if s == nil {
		return nil
	}
	agg := make(map[string]graphql.Fields, len(s.aggregateFields))
	for k, v := range s.aggregateFields {
		agg[k] = cloneGraphQLFields(v)
	}
	return &preConnectionShape{
		commonFields:                   cloneCommonFieldsByModel(s.commonFields),
		aggregateFields:                agg,
		whereArgs:                      cloneStringToInputObjectFieldMap(s.whereArgs),
		sortParam:                      cloneStringToInputObjectFieldMap(s.sortParam),
		connectionParamArgs:            cloneStringToInputObjectPtr(s.connectionParamArgs),
		whereRelationArgs:              cloneStringToInputObjectPtr(s.whereRelationArgs),
		createMutationFieldsArguments:  cloneStringToInputObjectFieldMap(s.createMutationFieldsArguments),
		updateMutationFieldsArguments:  cloneStringToInputObjectFieldMap(s.updateMutationFieldsArguments),
		connectionFields:               cloneStringToInputObjectFieldMap(s.connectionFields),
		commonMutationFieldsConfigArgs: cloneFieldConfigArgumentMap(s.commonMutationFieldsConfigArgs),
	}
}
