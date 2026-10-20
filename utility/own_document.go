package utility

import (
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
)

// CanonicalOwnerID returns the canonical owner stored on a document.
//
// The second result distinguishes an absent owner field from an invalid or
// conflicting owner field. Once a non-empty canonical field is present,
// callers must not fall back to audit metadata: a malformed or conflicting
// value is a denial rather than an opportunity to guess. Empty strings are
// treated as absent to match the SQL drivers' NULL/empty legacy semantics.
func CanonicalOwnerID(data map[string]interface{}) (string, bool) {
	if len(data) == 0 {
		return "", false
	}

	ownerID, hasOwnerID := canonicalStringField(data, "owner_id")
	ownerUID, hasOwnerUID := canonicalStringField(data, "owner_uid")
	if !hasOwnerID && !hasOwnerUID {
		return "", false
	}
	if hasOwnerID && hasOwnerUID && ownerID != ownerUID {
		return "", true
	}
	if hasOwnerID {
		return ownerID, true
	}
	return ownerUID, true
}

func canonicalStringField(data map[string]interface{}, key string) (string, bool) {
	value, present := data[key]
	if !present {
		return "", false
	}
	owner, ok := value.(string)
	if !ok {
		return "", true
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return "", false
	}
	return owner, true
}

// OwnDocumentOwnerID resolves the owner used by own-scoped authorization.
// User documents are self-owned by their persisted document id. Other
// documents use canonical owner fields and only use created_by as a legacy
// fallback when no canonical field exists.
func OwnDocumentOwnerID(doc *types.DefaultDocumentStructure) (ownerID string, isUserModel bool) {
	if doc == nil {
		return "", false
	}
	if isUserModelName(doc.Type) {
		return strings.TrimSpace(doc.ID), true
	}
	if owner, present := CanonicalOwnerID(doc.Data); present {
		return owner, false
	}
	if doc.Meta != nil && doc.Meta.CreatedBy != nil {
		return strings.TrimSpace(doc.Meta.CreatedBy.ID), false
	}
	return "", false
}

// OwnDocumentOwnerIDForModel resolves ownership when storage omitted the
// document type field. The schema model name is trusted server metadata, not
// client input.
func OwnDocumentOwnerIDForModel(doc *types.DefaultDocumentStructure, modelName string) (string, bool) {
	if isUserModelName(modelName) {
		if doc == nil {
			return "", true
		}
		return strings.TrimSpace(doc.ID), true
	}
	return OwnDocumentOwnerID(doc)
}

// OwnDocumentBelongsToUser applies the complete fail-closed own-read rule.
func OwnDocumentBelongsToUser(doc *types.DefaultDocumentStructure, modelName, userID string) bool {
	caller := strings.TrimSpace(userID)
	if caller == "" {
		return false
	}
	owner, _ := OwnDocumentOwnerIDForModel(doc, modelName)
	return owner != "" && owner == caller
}

// OwnReadUserID reports whether the request has an own-scoped read permission.
// The second result remains true when the caller id is empty so drivers can
// distinguish "own filtering is inactive" from "own filtering must deny all."
func OwnReadUserID(param *models.CommonSystemParams) (string, bool) {
	if param == nil || param.Model == nil || RoleBypassesDataACL(param.Role) {
		return "", false
	}
	permission := EffectivePermission(param.Role, param.Model.Name)
	if permission == nil || permission.Read != "own" {
		return "", false
	}
	if param.Role == nil || !param.Role.IsProjectUser {
		return "", true
	}
	return strings.TrimSpace(param.UserID), true
}
