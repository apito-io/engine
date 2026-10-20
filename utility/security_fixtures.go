//go:build security_fixtures

// This legacy file is retained only for source-history compatibility. New
// tests must import utility/securitytest; runtime builds never include it.
package utility

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/oklog/ulid/v2"
)

// Security fixtures are deterministic identifiers for adversarial tests.
// They are not credentials and must never be logged as secrets.
const (
	// These are valid, deterministic ULIDs so the same fixtures can be passed
	// through the real token and document paths without test-only identifier
	// parsing branches.
	FixtureUserA            = "01J00000000000000000000001"
	FixtureUserB            = "01J00000000000000000000002"
	FixtureTenantA          = "01J00000000000000000000003"
	FixtureTenantB          = "01J00000000000000000000004"
	FixtureWorkerActor      = "01J00000000000000000000005"
	FixtureProjectGeneral   = "security_general"
	FixtureProjectShared    = "security_shared"
	FixtureProjectIso       = "security_isolated"
	FixtureWorkerOwnedRow   = "01J00000000000000000000006"
	FixtureWorkerNoOwnerRow = "01J00000000000000000000007"

	CredentialPlaneBootstrap = "project_api_key_bootstrap"
	CredentialPlaneAppUser   = "project_api_key_user"
	CredentialPlaneOperator  = "access_token"
)

// SecurityFixture is the common deterministic topology used by adversarial
// security tests. It deliberately contains labels and identifiers only; it
// never contains a token, password, cookie, or API key.
type SecurityFixture struct {
	GeneralProject   string
	SharedProject    string
	IsolatedProject  string
	UserA            string
	UserB            string
	TenantA          string
	TenantB          string
	WorkerActor      string
	WorkerOwnedRow   string
	WorkerNoOwnerRow string
}

// DefaultSecurityFixture returns a value rather than exposing mutable package
// state. Callers may safely add test-specific rows without changing another
// test's fixture.
func DefaultSecurityFixture() SecurityFixture {
	return SecurityFixture{
		GeneralProject:   FixtureProjectGeneral,
		SharedProject:    FixtureProjectShared,
		IsolatedProject:  FixtureProjectIso,
		UserA:            FixtureUserA,
		UserB:            FixtureUserB,
		TenantA:          FixtureTenantA,
		TenantB:          FixtureTenantB,
		WorkerActor:      FixtureWorkerActor,
		WorkerOwnedRow:   FixtureWorkerOwnedRow,
		WorkerNoOwnerRow: FixtureWorkerNoOwnerRow,
	}
}

// FixtureCredential describes a credential plane without carrying its secret.
// Tests should use the real secret only at the transport boundary and should
// record this descriptor in evidence instead.
type FixtureCredential struct {
	Name  string
	Plane string
}

var SecurityFixtureCredentials = [...]FixtureCredential{
	{Name: "login-bootstrap", Plane: CredentialPlaneBootstrap},
	{Name: "app-user-session", Plane: CredentialPlaneAppUser},
	{Name: "operator-access-token", Plane: CredentialPlaneOperator},
}

// SecurityRequestEvidence is safe to retain in test output: header values are
// never stored, only normalized header names that were observed to contain
// secret material.
type SecurityRequestEvidence struct {
	CredentialPlane       string
	PrivilegedFallback    bool
	SecretBearingHeaders  []string
	SecretBearingLogEntry bool
}

// SecurityDecision is the redacted result a security test asserts.
type SecurityDecision struct {
	Status        int
	ErrorCode     string
	UserID        string
	TenantID      string
	Credential    string
	RequestCount  int
	FallbackCalls int
	SecretHeaders []string
	SecretLogged  bool
}

// ObserveSecurityRequests converts request evidence into a redacted decision.
// It is intentionally independent of a particular HTTP client so Workers,
// Go handlers, and Flutter test doubles can use the same assertion contract.
func ObserveSecurityRequests(
	base SecurityDecision,
	requests []SecurityRequestEvidence,
) SecurityDecision {
	out := base
	out.RequestCount = len(requests)
	out.FallbackCalls = 0
	out.SecretHeaders = nil
	for _, request := range requests {
		if request.PrivilegedFallback {
			out.FallbackCalls++
		}
		if request.SecretBearingLogEntry {
			out.SecretLogged = true
		}
		out.SecretHeaders = append(out.SecretHeaders, request.SecretBearingHeaders...)
	}
	out.SecretHeaders = uniqueSorted(out.SecretHeaders)
	return out
}

// AssertSecurityDecision checks the authorization result, selected identity,
// credential plane, request count, and the absence of escalation or secret
// leakage. All fields are exact comparisons; an omitted expected request is
// therefore not silently treated as acceptable.
func AssertSecurityDecision(got SecurityDecision, want SecurityDecision) error {
	if got.Status != want.Status {
		return errors.New("security status mismatch")
	}
	if got.ErrorCode != want.ErrorCode {
		return errors.New("security error code mismatch")
	}
	if got.UserID != want.UserID || got.TenantID != want.TenantID {
		return errors.New("security identity mismatch")
	}
	if got.Credential != want.Credential {
		return errors.New("security credential plane mismatch")
	}
	if got.RequestCount != want.RequestCount {
		return errors.New("security request count mismatch")
	}
	if got.FallbackCalls != want.FallbackCalls {
		return errors.New("security fallback count mismatch")
	}
	if !sameStrings(got.SecretHeaders, want.SecretHeaders) {
		return errors.New("security secret-bearing headers mismatch")
	}
	if got.SecretLogged != want.SecretLogged {
		return errors.New("security secret log mismatch")
	}
	if got.FallbackCalls != 0 || len(got.SecretHeaders) != 0 || got.SecretLogged {
		return errors.New("privileged fallback or secret log")
	}
	return nil
}

// AssertSecurityPhaseDecision gives every phase a stable, labeled assertion
// boundary. Keeping the label in the error makes CI output attributable
// without logging request credentials or payloads.
func AssertSecurityPhaseDecision(
	phase string,
	got SecurityDecision,
	want SecurityDecision,
) error {
	phase = strings.TrimSpace(phase)
	if phase == "" {
		return errors.New("security phase is required")
	}
	if err := AssertSecurityDecision(got, want); err != nil {
		return fmt.Errorf("%s: %w", phase, err)
	}
	return nil
}

// RedactSecret returns a non-reversible marker for logs and test output.
func RedactSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return "[redacted]"
}

// SecretBearingHeaderNames reports only header names, never values. It catches
// both conventional credential headers and accidental token-shaped values.
func SecretBearingHeaderNames(headers map[string]string) []string {
	names := make([]string, 0)
	for name, value := range headers {
		lowerName := strings.ToLower(strings.TrimSpace(name))
		lowerValue := strings.ToLower(strings.TrimSpace(value))
		if lowerName == "authorization" ||
			lowerName == "cookie" ||
			lowerName == "set-cookie" ||
			lowerName == "x-apito-key" ||
			lowerName == "x-apito-access-key" ||
			strings.Contains(lowerValue, "ak_") ||
			strings.Contains(lowerValue, "apt_") ||
			strings.Contains(lowerValue, "bearer ") {
			names = append(names, name)
		}
	}
	return uniqueSorted(names)
}

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool {
	a = uniqueSorted(a)
	b = uniqueSorted(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ValidateSecurityFixture checks the invariants that make the shared fixture
// safe to use in real authorization paths.
func ValidateSecurityFixture(f SecurityFixture) error {
	ids := map[string]string{
		"user_a":              f.UserA,
		"user_b":              f.UserB,
		"tenant_a":            f.TenantA,
		"tenant_b":            f.TenantB,
		"worker_actor":        f.WorkerActor,
		"worker_owned_row":    f.WorkerOwnedRow,
		"worker_no_owner_row": f.WorkerNoOwnerRow,
	}
	for name, value := range ids {
		if _, err := ulid.Parse(value); err != nil {
			return fmt.Errorf("%s is not a valid ULID: %w", name, err)
		}
	}
	if f.UserA == f.UserB || f.TenantA == f.TenantB {
		return errors.New("fixture principals must be distinct")
	}
	if strings.TrimSpace(f.GeneralProject) == "" ||
		strings.TrimSpace(f.SharedProject) == "" ||
		strings.TrimSpace(f.IsolatedProject) == "" {
		return errors.New("fixture project labels are required")
	}
	return nil
}
