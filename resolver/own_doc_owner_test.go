package resolver

import (
	"testing"

	"github.com/apito-io/engine/utility/securitytest"
	"github.com/apito-io/types"
)

func TestOwnDocOwnerIDDoesNotRequireProjectUserFlag(t *testing.T) {
	doc := &types.DefaultDocumentStructure{
		Type: "post",
		Meta: &types.MetaField{CreatedBy: &types.SystemUser{ID: securitytest.FixtureUserA}},
	}
	owner, isUser := ownDocOwnerID(doc)
	if isUser || owner != securitytest.FixtureUserA {
		t.Fatalf("creator owner %q user=%v", owner, isUser)
	}
}

func TestOwnDocOwnerIDPrefersCanonicalOwner(t *testing.T) {
	doc := &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": securitytest.FixtureUserA},
		Meta: &types.MetaField{CreatedBy: &types.SystemUser{ID: securitytest.FixtureWorkerActor}},
	}
	owner, _ := ownDocOwnerID(doc)
	if owner != securitytest.FixtureUserA {
		t.Fatalf("canonical owner %q", owner)
	}
	doc.Data["owner_uid"] = securitytest.FixtureUserB
	owner, _ = ownDocOwnerID(doc)
	if owner != "" {
		t.Fatalf("conflicting owners must fail closed, got %q", owner)
	}
}

func TestOwnDocOwnerIDUserDocument(t *testing.T) {
	owner, isUser := ownDocOwnerID(&types.DefaultDocumentStructure{ID: securitytest.FixtureUserA, Type: "users"})
	if !isUser || owner != securitytest.FixtureUserA {
		t.Fatalf("user owner %q %v", owner, isUser)
	}
}
