package schemadraft

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/apito-io/engine/models"
)

func TestLoadOldDraftFile(t *testing.T) {
	RootDir = t.TempDir()
	t.Cleanup(func() { RootDir = "" })
	raw := []byte(`{"project_id":"apito_website","ops":[{"id":"op1","type":"add_field","args":{"model_name":"food","field_label":"Text","field_type":"text"},"created_at":"staged"}]}`)
	if err := os.WriteFile(filepath.Join(RootDir, "apito_website.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	draft, err := Load("apito_website")
	if err != nil {
		t.Fatal(err)
	}
	if draft.ChangesetID != "local-apito_website" {
		t.Fatalf("changeset id %q", draft.ChangesetID)
	}
	if len(draft.Ops) != 1 || draft.Ops[0].Type != "add_field" {
		t.Fatalf("ops %+v", draft.Ops)
	}
}

func TestReplayAddFieldAndDiff(t *testing.T) {
	live := &models.ProjectSchema{Models: []*models.ModelType{{
		Name: "food",
	}}}
	ops := []Op{{
		ID:   "1",
		Type: "add_field",
		Args: map[string]interface{}{"model_name": "food", "field_label": "Date", "field_type": "date"},
	}}
	draft, err := Replay(live, ops)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Models[0].Fields) != 1 || draft.Models[0].Fields[0].Identifier == "" {
		t.Fatalf("fields %+v", draft.Models[0].Fields)
	}
	diff := ComputeDiff(live, draft)
	if len(diff.ModelChanges) != 1 || len(diff.ModelChanges[0].FieldsAdded) != 1 {
		t.Fatalf("diff %+v", diff)
	}
	rows := Plan("p", "local-p", ops)
	if len(rows) != 2 || rows[0]["operation_id"] != "1" || rows[0]["action_key"] != "add_field" {
		t.Fatalf("plan %+v", rows)
	}
}

func TestReplayRenameDeleteRelation(t *testing.T) {
	live := &models.ProjectSchema{Models: []*models.ModelType{
		{Name: "food", Fields: []*models.FieldInfo{{Identifier: "title", Label: "Title", FieldType: "text"}}},
		{Name: "dish"},
	}}
	ops := []Op{
		{ID: "r", Type: "rename_model", Args: map[string]interface{}{"model_name": "dish", "new_name": "plate"}},
		{ID: "d", Type: "model_field_operation", Args: map[string]interface{}{"model_name": "food", "field_name": "title", "type": "delete"}},
		{ID: "c", Type: "create_connection", Args: map[string]interface{}{"from": "food", "to": "plate", "known_as": "plate", "forward_connection_type": "has_many", "reverse_connection_type": "has_one"}},
	}
	draft, err := Replay(live, ops)
	if err != nil {
		t.Fatal(err)
	}
	if FindModel(draft, "plate") == nil || FindModel(draft, "dish") != nil {
		t.Fatalf("models %+v", draft.Models)
	}
	food := FindModel(draft, "food")
	if FindFieldByIdentifier(food.Fields, "title") != nil {
		t.Fatal("title still present")
	}
	if len(food.Connections) != 1 || food.Connections[0].Model != "plate" {
		t.Fatalf("connections %+v", food.Connections)
	}
}

func TestApproveKeepsDraftOnFailure(t *testing.T) {
	RootDir = t.TempDir()
	t.Cleanup(func() { RootDir = "" })
	if _, err := Append("p1", Op{ID: "a", Type: "create_model", Args: map[string]interface{}{"name": "food"}}); err != nil {
		t.Fatal(err)
	}
	local := NewLocal(func(_ context.Context, _ *models.Project, _ []Op) error {
		return os.ErrInvalid
	})
	project := &models.Project{ID: "p1", Schema: &models.ProjectSchema{}}
	if _, err := local.Approve(t.Context(), project, ChangesetID("p1"), "", ""); err == nil {
		t.Fatal("expected publish error")
	}
	draft, err := Load("p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Ops) != 1 {
		t.Fatalf("draft dropped: %+v", draft)
	}
	ok, err := local.Discard(t.Context(), project, ChangesetID("p1"))
	if err != nil || !ok {
		t.Fatalf("discard %v %v", ok, err)
	}
	draft, err = Load("p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Ops) != 0 {
		t.Fatalf("discard left ops %+v", draft.Ops)
	}
}

func TestSynchronousPublishClearsDraft(t *testing.T) {
	RootDir = t.TempDir()
	t.Cleanup(func() { RootDir = "" })
	if _, err := Append("p2", Op{ID: "a", Type: "create_model", Args: map[string]interface{}{"name": "food"}}); err != nil {
		t.Fatal(err)
	}
	var published int
	local := NewLocal(func(_ context.Context, _ *models.Project, ops []Op) error {
		published = len(ops)
		return nil
	})
	project := &models.Project{ID: "p2", Schema: &models.ProjectSchema{}}
	res, err := local.Approve(t.Context(), project, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if published != 1 || res["status"] != "published" {
		t.Fatalf("result %+v published %d", res, published)
	}
	draft, err := Load("p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Ops) != 0 {
		t.Fatalf("file still has ops %+v", draft.Ops)
	}
}
