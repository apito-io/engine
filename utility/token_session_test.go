package utility

import (
	"testing"

	"github.com/apito-io/engine/utility/securitytest"
)

func TestProjectUserSessionRevocation(t *testing.T) {
	scopes := AppendIssuedAtScope([]string{"tenant:" + securitytest.FixtureTenantA}, 100)
	if IssuedAtFromScopes(scopes) != 100 {
		t.Fatal("issued-at scope missing")
	}
	if ProjectUserSessionRevoked(scopes, 0) {
		t.Fatal("no revocation")
	}
	if !ProjectUserSessionRevoked(scopes, 101) {
		t.Fatal("older token must be revoked")
	}
	if !ProjectUserSessionRevoked(scopes, 100) {
		t.Fatal("token issued at the revocation second must be revoked")
	}
	if !ProjectUserSessionRevoked([]string{"tenant:" + securitytest.FixtureTenantA}, 50) {
		t.Fatal("missing iat cannot survive revocation")
	}
}
