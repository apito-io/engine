package services

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestIsStaffProjectUserKey(t *testing.T) {
	if IsStaffProjectUserKey(nil) {
		t.Fatal("nil claims must fail")
	}
	if IsStaffProjectUserKey(&models.TokenClaims{Role: "login", UserID: "u1", IsProjectUser: false}) {
		t.Fatal("login bootstrap must fail")
	}
	if IsStaffProjectUserKey(&models.TokenClaims{Role: "login_bootstrap", IsProjectUser: true, TokenType: "user"}) {
		t.Fatal("login_bootstrap must fail even with project-user flags")
	}
	if !IsStaffProjectUserKey(&models.TokenClaims{Role: "manager", IsProjectUser: true, TokenType: "tenant"}) {
		t.Fatal("manager tenant token must pass")
	}
	if !IsStaffProjectUserKey(&models.TokenClaims{Role: "website", TokenType: "tenant"}) {
		t.Fatal("website tenant token must pass")
	}
	if IsStaffProjectUserKey(&models.TokenClaims{Role: "public", UserID: "admin"}) {
		t.Fatal("non-project-user public key must fail")
	}
}

func TestIsSecuredAppDataPath(t *testing.T) {
	if !IsSecuredAppDataPath("/secured/graphql") {
		t.Fatal("secured graphql")
	}
	if IsSecuredAppDataPath("/system/graphql") {
		t.Fatal("system graphql must stay open for login keys")
	}
}
