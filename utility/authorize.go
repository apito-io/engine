package utility

import (
	"errors"
	"strings"

	"github.com/apito-io/engine/models"
)

// AuthorizeModelRead checks read access for a model.
// none → deny; auth → require IsProjectUser; own → require IsProjectUser (row filter is driver's job); all → ok.
// Unknown scopes fail closed. RoleBypassesDataACL → ok.
func AuthorizeModelRead(role *models.Role, modelName string) error {
	if RoleBypassesDataACL(role) {
		return nil
	}
	perm := EffectivePermission(role, modelName)
	switch perm.Read {
	case "all":
		return nil
	case "none", "":
		return errors.New("read is not permitted")
	case "auth", "own":
		if role == nil || !role.IsProjectUser {
			return errors.New("authentication is required to read a document")
		}
		return nil
	default:
		return errors.New("read is not permitted")
	}
}

// AuthorizeOwnDocumentRead denies when Read==own and createdByID does not match userID.
// No-op when Read is not own or the role bypasses data ACL.
func AuthorizeOwnDocumentRead(role *models.Role, modelName, userID, createdByID string) error {
	if RoleBypassesDataACL(role) {
		return nil
	}
	if EffectivePermission(role, modelName).Read != "own" {
		return nil
	}
	if userID == "" || createdByID == "" || createdByID != userID {
		return errors.New("permission denied: not the document owner")
	}
	return nil
}

// AuthorizeModelCreate checks create access for a model.
// none → deny; auth → require IsProjectUser; all → ok. own/custom_logic/unknown → deny.
func AuthorizeModelCreate(role *models.Role, modelName string) error {
	if RoleBypassesDataACL(role) {
		return nil
	}
	perm := EffectivePermission(role, modelName)
	switch perm.Create {
	case "all":
		return nil
	case "none", "":
		return errors.New("creation is not permitted")
	case "auth":
		if role == nil || !role.IsProjectUser {
			return errors.New("authentication is required to Create a Document")
		}
		return nil
	default:
		// own, custom_logic, and any unknown scope fail closed
		return errors.New("creation is not permitted")
	}
}

// AuthorizeModelUpdate checks update access.
// For own: user model → paramUserID must equal doc ID (pass doc.ID as docCreatedByUserID);
// otherwise created_by must match (caller should only pass createdBy when creator IsProjectUser).
func AuthorizeModelUpdate(role *models.Role, modelName string, docCreatedByUserID string, docIsUserModel bool, paramUserID string) error {
	if RoleBypassesDataACL(role) {
		return nil
	}
	perm := EffectivePermission(role, modelName)
	switch perm.Update {
	case "all":
		return nil
	case "none", "":
		return errors.New("Update is not permitted")
	case "auth":
		if role == nil || !role.IsProjectUser {
			return errors.New("Authentication is required to Update a Document")
		}
		return nil
	case "own":
		return authorizeOwnDocument(role, docCreatedByUserID, docIsUserModel, paramUserID, "edit")
	default:
		return errors.New("Update is not permitted")
	}
}

// AuthorizeModelDelete checks delete access with the same own semantics as update.
func AuthorizeModelDelete(role *models.Role, modelName string, docCreatedByUserID string, docIsUserModel bool, paramUserID string) error {
	if RoleBypassesDataACL(role) {
		return nil
	}
	perm := EffectivePermission(role, modelName)
	switch perm.Delete {
	case "all":
		return nil
	case "none", "":
		return errors.New("Delete is not permitted")
	case "auth":
		if role == nil || !role.IsProjectUser {
			return errors.New("Authentication is required to Delete a Document")
		}
		return nil
	case "own":
		return authorizeOwnDocument(role, docCreatedByUserID, docIsUserModel, paramUserID, "delete")
	default:
		return errors.New("Delete is not permitted")
	}
}

// authorizeOwnDocument fails closed. A caller must be an authenticated project
// user before an own-scoped permission can authorize a mutation. A missing
// caller or canonical owner is a denial for every model, including user
// documents. The persisted IsProjectUser flag on the document is not an
// ownership signal, but the caller's authenticated role is.
func authorizeOwnDocument(role *models.Role, docCreatedByUserID string, docIsUserModel bool, paramUserID string, action string) error {
	msg := "You are not authorized to " + action + " this document"
	if role == nil || !role.IsProjectUser {
		return errors.New(msg)
	}
	// User-model ownership is represented by the document id by the resolver.
	// Keep the explicit branch here so a future caller cannot accidentally pass
	// creator metadata for a user row and bypass the self-only rule.
	if docIsUserModel && strings.TrimSpace(docCreatedByUserID) == "" {
		return errors.New(msg)
	}
	callerID := strings.TrimSpace(paramUserID)
	ownerID := strings.TrimSpace(docCreatedByUserID)
	if callerID == "" || ownerID == "" || callerID != ownerID {
		return errors.New(msg)
	}
	return nil
}
