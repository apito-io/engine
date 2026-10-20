package utility

import (
	"reflect"
	"testing"

	"github.com/apito-io/engine/utility/securitytest"
)

func TestSecurityFixturesAreRedactedAndDeterministic(t *testing.T) {
	fixture := securitytest.DefaultSecurityFixture()
	if err := securitytest.ValidateSecurityFixture(fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.UserA != securitytest.FixtureUserA ||
		fixture.TenantB != securitytest.FixtureTenantB ||
		fixture.WorkerOwnedRow == fixture.WorkerNoOwnerRow {
		t.Fatal("default fixture changed unexpectedly")
	}
	if len(securitytest.SecurityFixtureCredentials) != 3 {
		t.Fatalf("credential fixture count=%d", len(securitytest.SecurityFixtureCredentials))
	}
	if securitytest.RedactSecret("ak_live_secret") != "[redacted]" {
		t.Fatal("secrets must be redacted")
	}
	if securitytest.RedactSecret("") != "" {
		t.Fatal("empty values stay empty")
	}
	err := securitytest.AssertSecurityDecision(securitytest.SecurityDecision{
		Status:       403,
		ErrorCode:    "TENANT_SCOPE_MISMATCH",
		UserID:       securitytest.FixtureUserA,
		TenantID:     securitytest.FixtureTenantA,
		Credential:   securitytest.CredentialPlaneAppUser,
		RequestCount: 0,
	}, securitytest.SecurityDecision{
		Status:       403,
		ErrorCode:    "TENANT_SCOPE_MISMATCH",
		UserID:       securitytest.FixtureUserA,
		TenantID:     securitytest.FixtureTenantA,
		Credential:   securitytest.CredentialPlaneAppUser,
		RequestCount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := securitytest.AssertSecurityDecision(securitytest.SecurityDecision{FallbackCalls: 1}, securitytest.SecurityDecision{}); err == nil {
		t.Fatal("privileged fallback must fail the assertion")
	}
}

func TestSecurityFixturesUseRealIdentifierShape(t *testing.T) {
	for name, value := range map[string]string{
		"user_a": securitytest.FixtureUserA, "user_b": securitytest.FixtureUserB,
		"tenant_a": securitytest.FixtureTenantA, "tenant_b": securitytest.FixtureTenantB,
		"worker": securitytest.FixtureWorkerActor,
	} {
		if len(value) != 26 {
			t.Fatalf("%s length=%d; want ULID length", name, len(value))
		}
	}
}

func TestSecretBearingHeadersOnlyReturnNames(t *testing.T) {
	headers := map[string]string{
		"Authorization":     "Bearer ak_test_only",
		"X-Apito-Tenant-ID": securitytest.FixtureTenantA,
		"X-Trace":           "safe",
	}
	got := securitytest.SecretBearingHeaderNames(headers)
	if want := []string{"Authorization"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("secret headers=%v want %v", got, want)
	}
}

func TestObserveSecurityRequestsCapturesFallbacksWithoutSecrets(t *testing.T) {
	got := securitytest.ObserveSecurityRequests(
		securitytest.SecurityDecision{
			Status:     403,
			ErrorCode:  "BFF_ACTION_FORBIDDEN",
			UserID:     securitytest.FixtureUserA,
			TenantID:   securitytest.FixtureTenantA,
			Credential: securitytest.CredentialPlaneAppUser,
		},
		[]securitytest.SecurityRequestEvidence{
			{CredentialPlane: securitytest.CredentialPlaneAppUser},
			{CredentialPlane: securitytest.CredentialPlaneOperator, PrivilegedFallback: true},
		},
	)
	if got.RequestCount != 2 || got.FallbackCalls != 1 {
		t.Fatalf("request evidence=%+v", got)
	}
	if got.SecretLogged || len(got.SecretHeaders) != 0 {
		t.Fatalf("secret evidence leaked=%+v", got)
	}
}

func TestAssertSecurityPhaseDecisionLabelsFailures(t *testing.T) {
	err := securitytest.AssertSecurityPhaseDecision(
		"phase-0-baseline",
		securitytest.SecurityDecision{Status: 401},
		securitytest.SecurityDecision{Status: 403},
	)
	if err == nil || err.Error() != "phase-0-baseline: security status mismatch" {
		t.Fatalf("unexpected phase error: %v", err)
	}
}
