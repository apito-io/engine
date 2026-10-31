package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
)

const (
	realtimeWhereMaxDepth = 6
	realtimeWhereMaxKeys  = 32
)

var realtimeOperators = map[string]struct{}{
	"eq": {}, "ne": {}, "gt": {}, "gte": {}, "lt": {}, "lte": {},
	"contains": {}, "in": {}, "not_in": {},
}

type realtimeAuth struct {
	Role   *models.Role
	UserID string
	ConnID string
}

type realtimeAuthKey struct{}
type realtimeSuppressKey struct{}

// WithRealtimeAuth attaches the subscriber role used to enforce own-row reads.
func WithRealtimeAuth(ctx context.Context, role *models.Role, userID, connID string) context.Context {
	return context.WithValue(ctx, realtimeAuthKey{}, realtimeAuth{Role: role, UserID: userID, ConnID: connID})
}

// WithRealtimeSuppressed marks bulk or migration work that must not emit live events.
func WithRealtimeSuppressed(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithValue(ctx, realtimeSuppressKey{}, true)
}

func realtimeSuppressed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	suppressed, _ := ctx.Value(realtimeSuppressKey{}).(bool)
	return suppressed
}

func realtimeAuthFrom(ctx context.Context) realtimeAuth {
	if ctx == nil {
		return realtimeAuth{}
	}
	auth, _ := ctx.Value(realtimeAuthKey{}).(realtimeAuth)
	return auth
}

func validateWhere(where interface{}) error {
	if where == nil {
		return nil
	}
	keys := 0
	return walkWhere(where, 0, &keys)
}

func walkWhere(value interface{}, depth int, keys *int) error {
	if depth > realtimeWhereMaxDepth {
		return fmt.Errorf("subscription where is too deep")
	}
	m, ok := asMap(value)
	if !ok {
		return fmt.Errorf("subscription where must be an object")
	}
	for key, child := range m {
		*keys++
		if *keys > realtimeWhereMaxKeys {
			return fmt.Errorf("subscription where is too large")
		}
		switch key {
		case "relation", "_key":
			return fmt.Errorf("subscription where does not support %s", key)
		case "AND", "OR":
			if err := walkWhere(child, depth+1, keys); err != nil {
				return err
			}
			continue
		}
		childMap, ok := asMap(child)
		if !ok {
			return fmt.Errorf("subscription where field %s is not an operator object", key)
		}
		if len(childMap) == 0 {
			return fmt.Errorf("subscription where field %s is empty", key)
		}
		operatorLike := false
		nested := false
		for op := range childMap {
			if _, ok := realtimeOperators[op]; ok {
				operatorLike = true
				continue
			}
			nested = true
		}
		if operatorLike && nested {
			return fmt.Errorf("subscription where field %s mixes operators and nested fields", key)
		}
		if operatorLike {
			continue
		}
		if err := walkWhere(childMap, depth+1, keys); err != nil {
			return err
		}
	}
	return nil
}

func eventMatchesWhere(evt models.ModelChangeEvent, where interface{}) bool {
	if where == nil {
		return true
	}
	if m, ok := asMap(where); ok && len(m) == 0 {
		return true
	}
	if evt.Node != nil && matchWhere(evt.Node, where) {
		return true
	}
	if evt.PreviousValues != nil && matchWhere(evt.PreviousValues, where) {
		return true
	}
	return false
}

func matchWhere(doc interface{}, where interface{}) bool {
	m, ok := asMap(where)
	if !ok {
		return false
	}
	for key, child := range m {
		switch key {
		case "AND":
			if !matchWhere(doc, child) {
				return false
			}
		case "OR":
			if matchAny(doc, child) {
				return true
			}
			return false
		default:
			if !matchField(doc, key, child) {
				return false
			}
		}
	}
	return true
}

func matchAny(doc, where interface{}) bool {
	m, ok := asMap(where)
	if !ok || len(m) == 0 {
		return false
	}
	for key, child := range m {
		one := map[string]interface{}{key: child}
		if matchWhere(doc, one) {
			return true
		}
	}
	return false
}

func matchField(doc interface{}, key string, predicate interface{}) bool {
	pred, ok := asMap(predicate)
	if !ok {
		return false
	}
	operatorLike := true
	for op := range pred {
		if _, ok := realtimeOperators[op]; !ok {
			operatorLike = false
			break
		}
	}
	if !operatorLike {
		value, found := lookupField(doc, key)
		if !found {
			return false
		}
		return matchWhere(value, pred)
	}
	value, found := lookupField(doc, key)
	for op, expected := range pred {
		if !compareOp(op, value, found, expected) {
			return false
		}
	}
	return true
}

func lookupField(doc interface{}, key string) (interface{}, bool) {
	m, ok := asMap(doc)
	if !ok {
		return nil, false
	}
	if value, ok := m[key]; ok {
		return value, true
	}
	if data, ok := asMap(m["data"]); ok {
		if value, ok := data[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func compareOp(op string, value interface{}, found bool, expected interface{}) bool {
	switch op {
	case "eq":
		return found && scalarEqual(value, expected)
	case "ne":
		return !found || !scalarEqual(value, expected)
	case "contains":
		left, lok := value.(string)
		right, rok := expected.(string)
		return lok && rok && strings.Contains(left, right)
	case "in":
		return found && inList(value, expected)
	case "not_in":
		return !found || !inList(value, expected)
	case "gt", "gte", "lt", "lte":
		if !found {
			return false
		}
		return compareOrdered(op, value, expected)
	default:
		return false
	}
}

func inList(value, expected interface{}) bool {
	switch list := expected.(type) {
	case []interface{}:
		for _, item := range list {
			if scalarEqual(value, item) {
				return true
			}
		}
	case []string:
		text := fmt.Sprint(value)
		for _, item := range list {
			if text == item {
				return true
			}
		}
	}
	return false
}

func compareOrdered(op string, value, expected interface{}) bool {
	left, lok := asFloat(value)
	right, rok := asFloat(expected)
	if lok && rok {
		switch op {
		case "gt":
			return left > right
		case "gte":
			return left >= right
		case "lt":
			return left < right
		case "lte":
			return left <= right
		}
	}
	ls, lok := value.(string)
	rs, rok := expected.(string)
	if !lok || !rok {
		return false
	}
	switch op {
	case "gt":
		return ls > rs
	case "gte":
		return ls >= rs
	case "lt":
		return ls < rs
	case "lte":
		return ls <= rs
	}
	return false
}

func asFloat(value interface{}) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func scalarEqual(left, right interface{}) bool {
	if lf, lok := asFloat(left); lok {
		if rf, rok := asFloat(right); rok {
			return lf == rf
		}
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func asMap(value interface{}) (map[string]interface{}, bool) {
	switch m := value.(type) {
	case map[string]interface{}:
		return m, true
	case map[string]string:
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out, true
	default:
		return nil, false
	}
}

func eventReadable(ctx context.Context, model string, evt models.ModelChangeEvent) bool {
	auth := realtimeAuthFrom(ctx)
	if auth.Role == nil {
		return true
	}
	if utility.AuthorizeModelRead(auth.Role, model) != nil {
		noteRealtimeAuthDeny()
		return false
	}
	if utility.EffectivePermission(auth.Role, model).Read != "own" {
		return true
	}
	if documentReadable(auth, model, evt.Node) || documentReadable(auth, model, evt.PreviousValues) {
		return true
	}
	noteRealtimeAuthDeny()
	return false
}

func documentReadable(auth realtimeAuth, model string, node interface{}) bool {
	if node == nil {
		return false
	}
	doc := nodeAsDocument(node, model)
	owner, _ := utility.OwnDocumentOwnerIDForModel(doc, model)
	return utility.AuthorizeOwnDocumentRead(auth.Role, model, auth.UserID, owner) == nil
}

func nodeAsDocument(node interface{}, model string) *types.DefaultDocumentStructure {
	raw, err := json.Marshal(node)
	if err != nil {
		return &types.DefaultDocumentStructure{Type: model}
	}
	var doc types.DefaultDocumentStructure
	if err := json.Unmarshal(raw, &doc); err != nil {
		return &types.DefaultDocumentStructure{Type: model}
	}
	if doc.Type == "" {
		doc.Type = model
	}
	return &doc
}
