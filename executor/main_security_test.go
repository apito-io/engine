package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/apito-io/engine/database"
	"github.com/apito-io/engine/models"
)

func TestGetProjectDriver_DeniedRoutingDoesNotFallBackToBase(t *testing.T) {
	expected := errors.New("tenant scoped database credentials are unavailable")
	cfg := &models.Config{
		ConnectionRoutingHook: func(context.Context, string) models.ConnectionRoutingResult {
			return models.ConnectionRoutingResult{
				Decision: models.ConnectionRouteDenied,
				Err:      expected,
			}
		},
	}
	executor := &GraphQLExecutor{
		connectionManager: database.NewConnectionManager(cfg, 1000, nil),
	}
	ctx := context.WithValue(context.Background(), "project_id", "project-a")

	_, err := executor.GetProjectDriver(ctx)
	if !errors.Is(err, expected) {
		t.Fatalf("expected routing denial, got %v", err)
	}
}

func TestGetProjectDriver_UnknownRoutingDecisionFailsClosed(t *testing.T) {
	cfg := &models.Config{
		ConnectionRoutingHook: func(context.Context, string) models.ConnectionRoutingResult {
			return models.ConnectionRoutingResult{Decision: models.ConnectionRoutingDecision(255)}
		},
	}
	executor := &GraphQLExecutor{
		connectionManager: database.NewConnectionManager(cfg, 1000, nil),
	}
	ctx := context.WithValue(context.Background(), "project_id", "project-a")

	_, err := executor.GetProjectDriver(ctx)
	if err == nil {
		t.Fatal("expected unknown routing decision to fail closed")
	}
}
