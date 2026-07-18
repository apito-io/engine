package functions

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
)

func TestDeployDraftAndResolveActiveSource(t *testing.T) {
	store, err := NewFilesystemArtifactStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	draft := "export default async () => ({ ok: true })"
	fn := &models.ApitoFunction{
		ProjectID: "proj1",
		Name:      "hello",
		Source:    draft,
		Language:  "typescript",
	}
	rev, build, dep, err := DeployDraft(context.Background(), store, fn, []byte(draft), "tester")
	if err != nil {
		t.Fatal(err)
	}
	if rev == nil || build == nil || dep == nil {
		t.Fatal("expected revision, build, deployment")
	}
	if fn.ActiveRevisionID != rev.ID {
		t.Fatalf("active revision not set: got %q want %q", fn.ActiveRevisionID, rev.ID)
	}
	if got := ResolveActiveSource(context.Background(), store, fn); got != draft {
		t.Fatalf("active source mismatch: %q", got)
	}

	// Draft edit should not change live active source.
	fn.Source = "export default async () => ({ draft: true })"
	if got := ResolveActiveSource(context.Background(), store, fn); got != draft {
		t.Fatalf("live source should stay deployed: %q", got)
	}

	rev2, _, _, err := DeployDraft(context.Background(), store, fn, []byte(fn.Source), "tester")
	if err != nil {
		t.Fatal(err)
	}
	rb := RollbackDeployment(fn, rev, "tester")
	if rb == nil || fn.ActiveRevisionID != rev.ID {
		t.Fatalf("rollback failed: %#v active=%q", rb, fn.ActiveRevisionID)
	}
	if got := ResolveActiveSource(context.Background(), store, fn); got != draft {
		t.Fatalf("after rollback want original draft artifact, got %q", got)
	}
	_ = rev2
}

func TestResolveActiveSourceFallsBackToDraft(t *testing.T) {
	fn := &models.ApitoFunction{Source: "draft-only"}
	if got := ResolveActiveSource(context.Background(), nil, fn); got != "draft-only" {
		t.Fatalf("got %q", got)
	}
}
