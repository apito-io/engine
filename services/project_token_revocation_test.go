package services

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestProjectUserSessionRevocationFailsClosedWithoutStore(t *testing.T) {
	manager := &ProjectKeyManager{}
	revoked, err := manager.projectUserSessionRevoked(
		context.Background(),
		"project",
		"user",
		[]string{"iat:10"},
	)
	if revoked {
		t.Fatal("unavailable revocation store must not be treated as an active session")
	}
	if !errors.Is(err, ErrProjectUserRevocationUnavailable) {
		t.Fatalf("error=%v; want ErrProjectUserRevocationUnavailable", err)
	}
}

func TestProjectUserSessionRevocationFailsClosedWithoutIssuedAt(t *testing.T) {
	store := &memoryProjectUserRevocationStore{values: map[string]int64{}}
	m1 := &ProjectKeyManager{revocationStore: store}
	m2 := &ProjectKeyManager{revocationStore: store}
	revoked, err := m1.projectUserSessionRevoked(context.Background(), "project", "user", []string{"iat:10"})
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("no revocation is recorded")
	}
	if err := m1.RevokeProjectUserSessions("project", "user"); err != nil {
		t.Fatal(err)
	}
	revoked, err = m2.projectUserSessionRevoked(context.Background(), "project", "user", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("token without iat must fail closed after revocation")
	}
	revoked, err = m2.projectUserSessionRevoked(context.Background(), "project", "other", nil)
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("revocation must stay scoped to the user")
	}
}

type memoryProjectUserRevocationStore struct {
	mu     sync.Mutex
	values map[string]int64
}

func (s *memoryProjectUserRevocationStore) GetProjectUserSessionRevokedAfter(
	_ context.Context,
	projectID, userID string,
) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[projectID+"\x00"+userID], nil
}

func (s *memoryProjectUserRevocationStore) SetProjectUserSessionRevokedAfter(
	_ context.Context,
	projectID, userID string,
	revokedAfter int64,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := projectID + "\x00" + userID
	if revokedAfter > s.values[key] {
		s.values[key] = revokedAfter
	}
	return nil
}
