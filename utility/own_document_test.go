package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/stretchr/testify/require"
)

func TestOwnDocumentOwnerIDCanonicalOwnership(t *testing.T) {
	owner, userModel := OwnDocumentOwnerID(&types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{
			"owner_id":  "user-a",
			"owner_uid": "user-a",
		},
		Meta: &types.MetaField{CreatedBy: &types.SystemUser{ID: "audit-actor"}},
	})
	require.Equal(t, "user-a", owner)
	require.False(t, userModel)

	owner, _ = OwnDocumentOwnerID(&types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{
			"owner_id":  "user-a",
			"owner_uid": "user-b",
		},
		Meta: &types.MetaField{CreatedBy: &types.SystemUser{ID: "user-a"}},
	})
	require.Empty(t, owner, "conflicting canonical owner fields must not fall back to audit metadata")
}

func TestOwnDocumentOwnerIDOnlyFallsBackWhenCanonicalFieldsAreAbsent(t *testing.T) {
	doc := &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": ""},
		Meta: &types.MetaField{CreatedBy: &types.SystemUser{ID: "user-a"}},
	}
	owner, _ := OwnDocumentOwnerID(doc)
	require.Equal(t, "user-a", owner, "empty legacy ownership is absent")

	doc.Data = map[string]interface{}{"owner_id": 42}
	owner, _ = OwnDocumentOwnerID(doc)
	require.Empty(t, owner, "non-string canonical ownership is invalid, not absent")

	doc.Data = nil
	owner, _ = OwnDocumentOwnerID(doc)
	require.Equal(t, "user-a", owner)
}

func TestOwnDocumentBelongsToUserIsSelfOnlyForUserModels(t *testing.T) {
	user := &types.DefaultDocumentStructure{ID: "user-a", Type: "users"}
	require.True(t, OwnDocumentBelongsToUser(user, "users", "user-a"))
	require.False(t, OwnDocumentBelongsToUser(user, "users", "user-b"))
	require.False(t, OwnDocumentBelongsToUser(user, "users", ""))
}

func TestOwnReadUserIDDistinguishesMissingCallerFromInactiveFilter(t *testing.T) {
	param := &models.CommonSystemParams{
		Model: &models.ModelType{Name: "post"},
		Role: &models.Role{
			ID:             "author",
			IsProjectUser:  true,
			APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}},
		},
	}
	userID, active := OwnReadUserID(param)
	require.True(t, active)
	require.Empty(t, userID)

	param.UserID = "user-a"
	userID, active = OwnReadUserID(param)
	require.True(t, active)
	require.Equal(t, "user-a", userID)

	param.Role = &models.Role{ID: "admin", IsAdmin: true}
	userID, active = OwnReadUserID(param)
	require.False(t, active)
	require.Empty(t, userID)

	param.Role = &models.Role{
		ID:             "untrusted",
		APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}},
	}
	userID, active = OwnReadUserID(param)
	require.True(t, active)
	require.Empty(t, userID)
}
