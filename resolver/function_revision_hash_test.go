package resolver

import (
	"context"
	"testing"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

type stubLifecycleStore struct {
	revs map[string]*models.FunctionRevision
}

func (s *stubLifecycleStore) SaveRevision(context.Context, *models.FunctionRevision) error {
	return nil
}
func (s *stubLifecycleStore) GetRevision(_ context.Context, _, revisionID string) (*models.FunctionRevision, error) {
	if s == nil || s.revs == nil {
		return nil, nil
	}
	return s.revs[revisionID], nil
}
func (s *stubLifecycleStore) ListRevisions(context.Context, string, string, int) ([]*models.FunctionRevision, error) {
	return nil, nil
}
func (s *stubLifecycleStore) SaveBuild(context.Context, *models.FunctionBuild) error {
	return nil
}
func (s *stubLifecycleStore) SaveDeployment(context.Context, *models.FunctionDeployment) error {
	return nil
}
func (s *stubLifecycleStore) ListDeployments(context.Context, string, string, int) ([]*models.FunctionDeployment, error) {
	return nil, nil
}
func (s *stubLifecycleStore) MarkDeploymentsSuperseded(context.Context, string, string, string) error {
	return nil
}
func (s *stubLifecycleStore) SaveInvocation(context.Context, *models.FunctionInvocation) error {
	return nil
}

var _ interfaces.FunctionLifecycleStore = (*stubLifecycleStore)(nil)

func TestEnrichActiveRevisionHashes(t *testing.T) {
	store := &stubLifecycleStore{
		revs: map[string]*models.FunctionRevision{
			"rev-1": {ID: "rev-1", ArtifactHash: "abc123"},
		},
	}
	fns := []*models.ApitoFunction{
		{Name: "published", ActiveRevisionID: "rev-1"},
		{Name: "draft-only"},
		{Name: "missing-rev", ActiveRevisionID: "rev-missing"},
	}
	enrichActiveRevisionHashes(context.Background(), store, "proj-1", fns)
	if fns[0].ActiveRevisionHash != "abc123" {
		t.Fatalf("published hash = %q, want abc123", fns[0].ActiveRevisionHash)
	}
	if fns[1].ActiveRevisionHash != "" {
		t.Fatalf("draft-only hash = %q, want empty", fns[1].ActiveRevisionHash)
	}
	if fns[2].ActiveRevisionHash != "" {
		t.Fatalf("missing-rev hash = %q, want empty", fns[2].ActiveRevisionHash)
	}
}

func TestEnrichActiveRevisionHashesNilStore(t *testing.T) {
	fns := []*models.ApitoFunction{{Name: "x", ActiveRevisionID: "rev-1"}}
	enrichActiveRevisionHashes(context.Background(), nil, "proj-1", fns)
	if fns[0].ActiveRevisionHash != "" {
		t.Fatalf("expected empty hash with nil store")
	}
}
