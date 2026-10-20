package utility

import (
	"errors"
	"reflect"
	"strings"

	"github.com/apito-io/engine/models"
)

// ErrRelationTargetForbidden is returned when a project user links or unlinks
// another user's owner edge.
var ErrRelationTargetForbidden = errors.New("RELATION_TARGET_FORBIDDEN")

// ValidateProjectUserRelationEdges denies cross-user owner/user connects and
// any owner disconnect for a non-admin project user. Audit creator metadata is
// not rewritten here.
func ValidateProjectUserRelationEdges(role *models.Role, userID string, connections, disconnects map[string]interface{}) error {
	if role == nil || !role.IsProjectUser || RoleBypassesDataACL(role) {
		return nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrRelationTargetForbidden
	}
	if err := validateOwnerEdges(userID, connections, false); err != nil {
		return err
	}
	return validateOwnerEdges(userID, disconnects, true)
}

func validateOwnerEdges(userID string, edges map[string]interface{}, disconnect bool) error {
	for key, value := range edges {
		if !isOwnerEdge(key) {
			continue
		}
		if disconnect {
			return ErrRelationTargetForbidden
		}
		ids := relationIDs(value)
		if len(ids) == 0 {
			return ErrRelationTargetForbidden
		}
		for _, id := range ids {
			if id != userID {
				return ErrRelationTargetForbidden
			}
		}
	}
	return nil
}

// ValidateProjectUserRelationParams repeats the owner-edge check after the
// public payload has been converted into driver instructions. This is the
// authoritative boundary shared by SQL and non-SQL drivers; callers cannot
// bypass the policy by constructing ConnectDisconnectParam values directly.
func ValidateProjectUserRelationParams(
	role *models.Role,
	userID string,
	params []*models.ConnectDisconnectParam,
	disconnect bool,
) error {
	if role == nil || !role.IsProjectUser || RoleBypassesDataACL(role) {
		return nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrRelationTargetForbidden
	}
	for _, param := range params {
		if param == nil {
			return ErrRelationTargetForbidden
		}
		if param.TargetIsUserModel {
			if disconnect || len(param.ActionIDs) == 0 {
				return ErrRelationTargetForbidden
			}
			for _, id := range param.ActionIDs {
				if strings.TrimSpace(id) != userID {
					return ErrRelationTargetForbidden
				}
			}
		}
		if param.TargetRequiresOwnership && !param.TargetOwnershipIsVerified() {
			return ErrRelationTargetForbidden
		}
		if !isOwnerEdge(param.RequestedRelationField) {
			continue
		}
		if disconnect {
			return ErrRelationTargetForbidden
		}
		if len(param.ActionIDs) == 0 {
			return ErrRelationTargetForbidden
		}
		for _, id := range param.ActionIDs {
			if strings.TrimSpace(id) != userID {
				return ErrRelationTargetForbidden
			}
		}
	}
	return nil
}

func isUserModelName(modelName string) bool {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
	case "user", "users":
		return true
	default:
		return false
	}
}

// ValidateOwnerFieldMutation prevents an ordinary project user from adding,
// changing, or removing an owner relation during an update. Transfers must
// use an audited service command; public CRUD cannot turn a creator-owned row
// into someone else's row.
func ValidateOwnerFieldMutation(
	role *models.Role,
	userID string,
	before,
	after map[string]interface{},
) error {
	if role == nil || !role.IsProjectUser || RoleBypassesDataACL(role) {
		return nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrRelationTargetForbidden
	}
	for _, key := range []string{"owner_id", "owner_uid", "user_id", "owner", "user"} {
		beforeValue, beforeOK := before[key]
		afterValue, afterOK := after[key]
		if !afterOK {
			continue
		}
		if !beforeOK || !reflect.DeepEqual(beforeValue, afterValue) {
			return ErrRelationTargetForbidden
		}
	}
	if ownerID, ownerUID := relationIDs(after["owner_id"]), relationIDs(after["owner_uid"]); len(ownerID) > 0 && len(ownerUID) > 0 && ownerID[0] != ownerUID[0] {
		return ErrRelationTargetForbidden
	}
	return nil
}

// ValidateOwnerFieldOnCreate prevents an ordinary project user from assigning
// ownership to another principal while creating a document. Unlike the
// update validator, there is no prior owner to compare against: every
// supplied owner-shaped field must resolve only to the caller, and malformed
// or conflicting values fail closed.
func ValidateOwnerFieldOnCreate(
	role *models.Role,
	userID string,
	payload map[string]interface{},
) error {
	if role == nil || !role.IsProjectUser || RoleBypassesDataACL(role) {
		return nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrRelationTargetForbidden
	}

	var ownerIDs map[string]string
	for _, key := range []string{"owner_id", "owner_uid", "user_id", "owner", "user"} {
		value, present := payload[key]
		if !present {
			continue
		}
		ids := relationIDs(value)
		if len(ids) != 1 || ids[0] != userID {
			return ErrRelationTargetForbidden
		}
		if ownerIDs == nil {
			ownerIDs = make(map[string]string)
		}
		ownerIDs[key] = ids[0]
	}
	if ownerIDs["owner_id"] != "" &&
		ownerIDs["owner_uid"] != "" &&
		ownerIDs["owner_id"] != ownerIDs["owner_uid"] {
		return ErrRelationTargetForbidden
	}
	return nil
}

func isOwnerEdge(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "owner_id", "owner_uid", "user_id", "owner", "user":
		return true
	default:
		return false
	}
}

func relationIDs(value interface{}) []string {
	switch v := value.(type) {
	case string:
		if id := strings.TrimSpace(v); id != "" {
			return []string{id}
		}
	case []string:
		out := make([]string, 0, len(v))
		for _, id := range v {
			if id = strings.TrimSpace(id); id != "" {
				out = append(out, id)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if ids := relationIDs(item); len(ids) > 0 {
				out = append(out, ids...)
			}
		}
		return out
	case map[string]interface{}:
		// Accept the shapes emitted by generated clients and reject unknown
		// owner payloads in validateOwnerEdges instead of silently treating
		// them as an empty edge list.
		for _, key := range []string{"id", "_id", "user_id", "owner_id", "owner_uid"} {
			if id := relationIDs(v[key]); len(id) > 0 {
				return id
			}
		}
		for _, key := range []string{"connect", "ids", "data"} {
			if ids := relationIDs(v[key]); len(ids) > 0 {
				return ids
			}
		}
	case map[string]string:
		for _, key := range []string{"id", "_id", "user_id", "owner_id", "owner_uid"} {
			if id := strings.TrimSpace(v[key]); id != "" {
				return []string{id}
			}
		}
	}
	return nil
}
