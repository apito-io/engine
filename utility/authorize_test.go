package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility/securitytest"
)

func TestAuthorizeModelReadTable(t *testing.T) {
	tests := []struct {
		name    string
		role    *models.Role
		model   string
		wantErr bool
	}{
		{"admin bypass", &models.Role{ID: "admin"}, "post", false},
		{"owner bypass", &models.Role{ID: "owner"}, "post", false},
		{
			"none denies",
			&models.Role{ID: "staff", APIPermissions: map[string]*models.APIPermission{"post": {Read: "none"}}},
			"post", true,
		},
		{
			"missing key denies as none",
			&models.Role{ID: "staff", APIPermissions: map[string]*models.APIPermission{}},
			"post", true,
		},
		{
			"all allows",
			&models.Role{ID: "staff", APIPermissions: map[string]*models.APIPermission{"post": {Read: "all"}}},
			"post", false,
		},
		{
			"auth without project user",
			&models.Role{ID: "staff", IsProjectUser: false, APIPermissions: map[string]*models.APIPermission{"post": {Read: "auth"}}},
			"post", true,
		},
		{
			"auth with project user",
			&models.Role{ID: "staff", IsProjectUser: true, APIPermissions: map[string]*models.APIPermission{"post": {Read: "auth"}}},
			"post", false,
		},
		{
			"own without project user",
			&models.Role{ID: "staff", IsProjectUser: false, APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}}},
			"post", true,
		},
		{
			"own with project user",
			&models.Role{ID: "staff", IsProjectUser: true, APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}}},
			"post", false,
		},
		{"nil role", nil, "post", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AuthorizeModelRead(tt.role, tt.model)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAuthorizeModelUpdateOwnFailsClosed(t *testing.T) {
	role := &models.Role{
		ID:            "passenger",
		IsProjectUser: true,
		APIPermissions: map[string]*models.APIPermission{
			"post": {Update: "own", Delete: "own"},
		},
	}
	if err := AuthorizeModelUpdate(role, "post", securitytest.FixtureUserA, false, securitytest.FixtureUserA); err != nil {
		t.Fatal(err)
	}
	if err := AuthorizeModelUpdate(role, "post", "", false, securitytest.FixtureUserA); err == nil {
		t.Fatal("empty owner must deny")
	}
	if err := AuthorizeModelUpdate(role, "post", securitytest.FixtureUserA, false, ""); err == nil {
		t.Fatal("empty caller must deny")
	}
	if err := AuthorizeModelDelete(role, "post", securitytest.FixtureUserB, false, securitytest.FixtureUserA); err == nil {
		t.Fatal("other owner must deny")
	}
	if err := AuthorizeModelUpdate(role, "post", securitytest.FixtureUserB, true, ""); err == nil {
		t.Fatal("user model without caller must deny")
	}
}

func TestAuthorizeModelOwnRequiresProjectUserRole(t *testing.T) {
	permission := map[string]*models.APIPermission{
		"post": {Update: "own", Delete: "own"},
	}
	tests := []struct {
		name string
		role *models.Role
	}{
		{name: "nil role"},
		{name: "non project user", role: &models.Role{ID: "staff", APIPermissions: permission}},
		{name: "role without project-user identity", role: &models.Role{IsProjectUser: false, APIPermissions: permission}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AuthorizeModelUpdate(tt.role, "post", securitytest.FixtureUserA, false, securitytest.FixtureUserA); err == nil {
				t.Fatal("own update must require an authenticated project-user role")
			}
			if err := AuthorizeModelDelete(tt.role, "post", securitytest.FixtureUserA, false, securitytest.FixtureUserA); err == nil {
				t.Fatal("own delete must require an authenticated project-user role")
			}
		})
	}
}

func TestAuthorizeModelOwnAllowsCanonicalOwnerOnly(t *testing.T) {
	role := &models.Role{
		ID:            "passenger",
		IsProjectUser: true,
		APIPermissions: map[string]*models.APIPermission{
			"post": {Update: "own", Delete: "own"},
		},
	}
	if err := AuthorizeModelUpdate(role, "post", securitytest.FixtureUserA, false, securitytest.FixtureUserA); err != nil {
		t.Fatal("creator/canonical owner should pass:", err)
	}
	if err := AuthorizeModelDelete(role, "post", securitytest.FixtureUserA, false, securitytest.FixtureUserB); err == nil {
		t.Fatal("different canonical owner must deny")
	}
	if err := AuthorizeModelUpdate(role, "post", "", false, securitytest.FixtureUserA); err == nil {
		t.Fatal("missing canonical owner must deny")
	}
}

func TestAuthorizeOwnDocumentRead(t *testing.T) {
	ownRole := &models.Role{
		ID:             "staff",
		IsProjectUser:  true,
		APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}},
	}
	if err := AuthorizeOwnDocumentRead(ownRole, "post", "u1", "u1"); err != nil {
		t.Fatalf("owner should pass: %v", err)
	}
	if err := AuthorizeOwnDocumentRead(ownRole, "post", "u1", "u2"); err == nil {
		t.Fatal("non-owner should fail")
	}
	allRole := &models.Role{
		ID:             "staff",
		APIPermissions: map[string]*models.APIPermission{"post": {Read: "all"}},
	}
	if err := AuthorizeOwnDocumentRead(allRole, "post", "u1", "u2"); err != nil {
		t.Fatalf("non-own read should no-op: %v", err)
	}
	if err := AuthorizeOwnDocumentRead(&models.Role{ID: "admin"}, "post", "u1", "u2"); err != nil {
		t.Fatalf("admin bypass should no-op: %v", err)
	}
}
