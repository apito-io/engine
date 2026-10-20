// Package securitytest contains redacted, deterministic fixtures and assertion
// helpers for security tests. It is intentionally separate from the runtime
// utility package so production code cannot accidentally depend on test data.
package securitytest

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/oklog/ulid/v2"
)

// These are valid, deterministic ULIDs for exercising real authorization
// paths. They are identifiers only; this package never contains credentials.
const (
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
// security tests.
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

// DefaultSecurityFixture returns a fresh value so individual tests can extend
// it without mutating shared state.
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
type FixtureCredential struct {
	Name  string
	Plane string
}

var SecurityFixtureCredentials = [...]FixtureCredential{
	{Name: "login-bootstrap", Plane: CredentialPlaneBootstrap},
	{Name: "app-user-session", Plane: CredentialPlaneAppUser},
	{Name: "operator-access-token", Plane: CredentialPlaneOperator},
}

// SecurityRequestEvidence is safe to retain in test output: it stores only
// normalized names of headers that contained secret material.
type SecurityRequestEvidence struct {
	CredentialPlane       string
	PrivilegedFallback    bool
	SecretBearingHeaders  []string
	SecretBearingLogEntry bool
}

// SecurityDecision is a redacted result for a security assertion.
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

// AssertSecurityDecision checks the full security outcome, including the
// absence of privileged fallbacks and secret leakage.
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

// AssertSecurityPhaseDecision gives each remediation phase a stable assertion
// boundary without putting credentials in failure output.
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

// RedactSecret returns a non-reversible marker suitable for test logs.
func RedactSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "[redacted]"
}

// SecretBearingHeaderNames reports header names, never header values.
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

// ValidateSecurityFixture checks the invariants needed by the shared fixture.
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
