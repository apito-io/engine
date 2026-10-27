package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services/schemadraft"
	"github.com/labstack/echo/v4"
)

func TestStampLocalSchemaReplayCopiesOntoRequest(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/system/graphql", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	ctx := context.WithValue(req.Context(), "router", c)
	ctx = stampLocalSchemaReplay(ctx)
	if ctx.Value(localSchemaReplayKey{}) != true {
		t.Fatal("replay flag missing on the resolver context")
	}
	if c.Request().Context().Value(localSchemaReplayKey{}) != true {
		t.Fatal("replay flag missing on the HTTP request context")
	}
}

func TestTryStageSkipsWhenReplayContext(t *testing.T) {
	schemadraft.RootDir = t.TempDir()
	t.Cleanup(func() { schemadraft.RootDir = "" })

	s := &GraphQLServer{Cfg: &models.Config{SchemaVersioningEnabled: true}}
	project := &models.Project{ID: "proj"}
	called := false
	replay := context.WithValue(context.Background(), localSchemaReplayKey{}, true)
	got, err := s.tryStageSchemaMutation(
		&models.ApplicationCache{Ctx: context.Background()},
		replay,
		project,
		models.SchemaOpTypeCreateModel,
		map[string]interface{}{"name": "note"},
		func() (interface{}, error) {
			called = true
			return "applied", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called || got != "applied" {
		t.Fatalf("replay must apply the model, got %#v called=%v", got, called)
	}
	draft, err := schemadraft.Load(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Ops) != 0 {
		t.Fatalf("replay restaged %d ops", len(draft.Ops))
	}
}

func TestTryStageUsesCacheReplayFlag(t *testing.T) {
	schemadraft.RootDir = t.TempDir()
	t.Cleanup(func() { schemadraft.RootDir = "" })

	s := &GraphQLServer{Cfg: &models.Config{SchemaVersioningEnabled: true}}
	called := false
	cache := &models.ApplicationCache{Ctx: context.WithValue(context.Background(), localSchemaReplayKey{}, true)}
	if _, err := s.tryStageSchemaMutation(cache, context.Background(), &models.Project{ID: "proj"}, models.SchemaOpTypeCreateModel, map[string]interface{}{"name": "note"}, func() (interface{}, error) {
		called = true
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("cache replay flag must skip staging")
	}
}

func TestSchemaDraftRestagedDetectsNewOps(t *testing.T) {
	before := &localSchemaDraft{Ops: []localSchemaOp{{ID: "a"}}}
	same := &localSchemaDraft{Ops: []localSchemaOp{{ID: "a"}}}
	if schemaDraftRestaged(before, same) {
		t.Fatal("same ops are not a restage")
	}
	grown := &localSchemaDraft{Ops: []localSchemaOp{{ID: "a"}, {ID: "b"}}}
	if !schemaDraftRestaged(before, grown) {
		t.Fatal("a new op id is a restage")
	}
}
