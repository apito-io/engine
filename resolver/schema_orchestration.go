package resolver

import (
	"context"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	schemasvc "github.com/apito-io/engine/services/schema"
)

const skipConnectionRoutingContextKey = "skip_connection_routing"

func (s *GraphQLServer) schemaHooks() schemasvc.Hooks {
	h := schemasvc.Hooks{
		SchemaIterate: s.Cfg.SchemaIterateHook,
		Store:         schemasvc.StoreFromSystemDriver(s.SystemDriver),
	}
	if s.Cfg != nil && s.Cfg.PostSchemaChangeHook != nil {
		hook := s.Cfg.PostSchemaChangeHook
		h.AfterCommit = func(in schemasvc.RunInput) {
			ctx := in.Ctx
			if in.PhysicalDDLRequired {
				ctx = schemasvc.WithPhysicalDDLRequired(ctx, true)
			}
			hook(ctx, in.BaseDriver, in.Project)
		}
	}
	return h
}

// applySchemaDDLToBaseAndTenants runs apply on the base project driver then each tenant driver via SchemaIterateHook.
func (s *GraphQLServer) applySchemaDDLToBaseAndTenants(
	ctx context.Context,
	project *models.Project,
	baseDriver interfaces.ProjectDBInterface,
	apply func(driver interfaces.ProjectDBInterface) error,
) error {
	if err := apply(baseDriver); err != nil {
		return err
	}
	if s.Cfg.SchemaIterateHook == nil {
		return nil
	}
	return s.Cfg.SchemaIterateHook(ctx, project, func(tctx context.Context, drv interface{}) error {
		td, ok := drv.(interfaces.ProjectDBInterface)
		if !ok || td == nil {
			return nil
		}
		return apply(td)
	})
}

func (s *GraphQLServer) runSchemaChange(ctx context.Context, in schemasvc.RunInput) error {
	return schemasvc.Run(s.schemaHooks(), in)
}

// RunSchemaChange executes the schema change orchestrator (exported for pro schema versioning publish).
func (s *GraphQLServer) RunSchemaChange(ctx context.Context, in schemasvc.RunInput) error {
	return s.runSchemaChange(ctx, in)
}

// GetSchemaBaseProjectDriverIfNeeded returns the base project driver for schema DDL when needed.
func (s *GraphQLServer) GetSchemaBaseProjectDriverIfNeeded(ctx context.Context, project *models.Project) (interfaces.ProjectDBInterface, bool, error) {
	return s.getSchemaBaseProjectDriverIfNeeded(ctx, project)
}

func schemaBaseDriverContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipConnectionRoutingContextKey, true)
}

func (s *GraphQLServer) getSchemaBaseProjectDriver(ctx context.Context) (interfaces.ProjectDBInterface, error) {
	return s.GraphQLExecutor.GetProjectDriver(schemaBaseDriverContext(ctx))
}

func (s *GraphQLServer) skipSchemaBaseDDL(ctx context.Context, project *models.Project) bool {
	return s != nil && s.Cfg != nil && s.Cfg.SkipSchemaBaseDDLHook != nil && s.Cfg.SkipSchemaBaseDDLHook(ctx, project)
}

func (s *GraphQLServer) getSchemaBaseProjectDriverIfNeeded(ctx context.Context, project *models.Project) (interfaces.ProjectDBInterface, bool, error) {
	if s.skipSchemaBaseDDL(ctx, project) {
		return nil, true, nil
	}
	drv, err := s.getSchemaBaseProjectDriver(ctx)
	return drv, false, err
}
