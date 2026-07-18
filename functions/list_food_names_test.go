package functions

import (
	"context"
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
)

func TestRequireCapabilityFoodRead(t *testing.T) {
	check := RequireCapability([]string{"data.read:food"})
	err := check(context.Background(), &DataGatewayCall{
		Op:      "getList",
		Payload: map[string]interface{}{"model": "food"},
	})
	if err != nil {
		t.Fatalf("expected allow: %v", err)
	}
	err = check(context.Background(), &DataGatewayCall{
		Op:      "getList",
		Payload: map[string]interface{}{"model": "order"},
	})
	if err == nil || !strings.Contains(err.Error(), "capability denied") {
		t.Fatalf("expected denial, got %v", err)
	}
}

func TestDeployThenActiveSourceIgnoresDraftEdit(t *testing.T) {
	store, err := NewFilesystemArtifactStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fn := &models.ApitoFunction{
		ProjectID:    "rosna_v2_jpn6o",
		Name:         "listFoodNames",
		Source:       "export default async () => ({ foods: [] })",
		Language:     "typescript",
		Capabilities: []string{"data.read:food"},
	}
	rev, _, _, err := DeployDraft(context.Background(), store, fn, []byte(fn.Source), "e2e")
	if err != nil {
		t.Fatal(err)
	}
	fn.Source = "export default async () => ({ foods: ['DRAFT_ONLY'] })"
	got := ResolveActiveSource(context.Background(), store, fn)
	if strings.Contains(got, "DRAFT_ONLY") {
		t.Fatal("live source must not include unsaved draft edit")
	}
	if fn.ActiveRevisionID != rev.ID {
		t.Fatalf("active revision %q != %q", fn.ActiveRevisionID, rev.ID)
	}
}
