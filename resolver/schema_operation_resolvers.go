package resolver

import (
	"errors"
	"fmt"
	"sync"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	schemasvc "github.com/apito-io/engine/services/schema"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

var (
	schemaOperationObject     *graphql.Object
	schemaOperationObjectOnce sync.Once
)

func schemaOperationGraphQLObject() *graphql.Object {
	schemaOperationObjectOnce.Do(func() {
		schemaOperationObject = graphql.NewObject(graphql.ObjectConfig{
			Name: "SchemaOperation",
			Fields: graphql.Fields{
				"id":              &graphql.Field{Type: graphql.String},
				"project_id":      &graphql.Field{Type: graphql.String},
				"operation_type":  &graphql.Field{Type: graphql.String},
				"status":          &graphql.Field{Type: graphql.String},
				"request_json":    &graphql.Field{Type: graphql.String},
				"steps_json":      &graphql.Field{Type: graphql.String},
				"error":           &graphql.Field{Type: graphql.String},
				"attempt_count":   &graphql.Field{Type: graphql.Int},
				"created_at":      &graphql.Field{Type: graphql.String},
				"updated_at":      &graphql.Field{Type: graphql.String},
			},
		})
	})
	return schemaOperationObject
}

func (s *GraphQLServer) SearchSchemaOperationsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	v := p.Context.Value
	router := v("router").(echo.Context)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	store := schemasvc.StoreFromSystemDriver(s.SystemDriver)
	if store == nil {
		return []*models.SchemaOperation{}, nil
	}
	var statuses []string
	if raw, ok := p.Args["statuses"].([]interface{}); ok {
		for _, item := range raw {
			if st, ok := item.(string); ok && st != "" {
				statuses = append(statuses, st)
			}
		}
	}
	if len(statuses) == 0 {
		statuses = []string{
			models.SchemaOpStatusApplying,
			models.SchemaOpStatusFailed,
			models.SchemaOpStatusNeedsRepair,
		}
	}
	limit := 50
	if val, ok := p.Args["limit"].(int); ok && val > 0 {
		limit = val
	}
	ops, err := store.ListSchemaOperationsByStatus(cache.Ctx, cache.Project.ID, statuses, limit)
	if err != nil {
		return nil, err
	}
	return ops, nil
}

func (s *GraphQLServer) RetrySchemaOperationResolverFn(p graphql.ResolveParams) (interface{}, error) {
	v := p.Context.Value
	router := v("router").(echo.Context)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	store := schemasvc.StoreFromSystemDriver(s.SystemDriver)
	if store == nil {
		return nil, errors.New("schema operation ledger not supported by system database")
	}
	var opID string
	if val, ok := p.Args["operation_id"].(string); ok && val != "" {
		opID = val
	} else {
		return nil, errors.New("operation_id is required")
	}
	op, err := store.GetSchemaOperation(cache.Ctx, opID)
	if err != nil {
		return nil, err
	}
	if op == nil || op.ProjectID != cache.Project.ID {
		return nil, fmt.Errorf("schema operation not found for project")
	}
	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}
	project := cache.Project
	updated, err := schemasvc.ReconcileOneOperation(s.schemaHooks(), schemasvc.ReconcileInput{
		Ctx:        cache.Ctx,
		Op:         op,
		BaseDriver: driver,
		ApplyDDL: func(d interfaces.ProjectDBInterface) error {
			return fmt.Errorf("retry schema operation: manual repair required for operation type %s", op.OperationType)
		},
		PersistSystem: func() error {
			return s.SystemDriver.TouchProjectUpdatedAt(cache.Ctx, project.ID)
		},
		RefreshCache: func() error {
			_, err := s.refreshProjectAndReCache(cache.Ctx, project.ID)
			return err
		},
	})
	if err != nil {
		return updated, err
	}
	return updated, nil
}
