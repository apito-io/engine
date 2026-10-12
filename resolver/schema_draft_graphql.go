package resolver

import (
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

func (s *GraphQLServer) RegisterSchemaDraftGraphQL() {
	if s == nil || s.SchemaDraft == nil {
		return
	}
	api := s.SchemaDraft

	statusType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SchemaVersioningStatus",
		Fields: graphql.Fields{
			"enabled":            &graphql.Field{Type: graphql.Boolean},
			"active_version":     &graphql.Field{Type: graphql.Int},
			"has_draft":          &graphql.Field{Type: graphql.Boolean},
			"changeset_id":       &graphql.Field{Type: graphql.String},
			"changeset_status":   &graphql.Field{Type: graphql.String},
			"pending_operations": &graphql.Field{Type: graphql.Int},
			"publish_total":      &graphql.Field{Type: graphql.Int},
			"publish_done":       &graphql.Field{Type: graphql.Int},
			"error":              &graphql.Field{Type: graphql.String},
			"capabilities": &graphql.Field{Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "SchemaDraftCapabilities",
				Fields: graphql.Fields{
					"history":       &graphql.Field{Type: graphql.Boolean},
					"collaboration": &graphql.Field{Type: graphql.Boolean},
					"revert":        &graphql.Field{Type: graphql.Boolean},
					"repair":        &graphql.Field{Type: graphql.Boolean},
					"remote_sync":   &graphql.Field{Type: graphql.Boolean},
					"async_publish": &graphql.Field{Type: graphql.Boolean},
				},
			})},
		},
	})
	eventType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SchemaChangeEventItem",
		Fields: graphql.Fields{
			"event_type":   &graphql.Field{Type: graphql.String},
			"actor_id":     &graphql.Field{Type: graphql.String},
			"created_at":   &graphql.Field{Type: graphql.String},
			"payload_json": &graphql.Field{Type: graphql.String},
		},
	})
	recordType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SchemaChangeExecutionRecord",
		Fields: graphql.Fields{
			"id":                  &graphql.Field{Type: graphql.String},
			"project_id":          &graphql.Field{Type: graphql.String},
			"changeset_id":        &graphql.Field{Type: graphql.String},
			"schema_version_id":   &graphql.Field{Type: graphql.String},
			"schema_operation_id": &graphql.Field{Type: graphql.String},
			"operation_id":        &graphql.Field{Type: graphql.String},
			"sequence":            &graphql.Field{Type: graphql.Int},
			"scope_kind":          &graphql.Field{Type: graphql.String},
			"scope_key":           &graphql.Field{Type: graphql.String},
			"target_kind":         &graphql.Field{Type: graphql.String},
			"target_name":         &graphql.Field{Type: graphql.String},
			"action_key":          &graphql.Field{Type: graphql.String},
			"impact":              &graphql.Field{Type: graphql.String},
			"requires_ddl":        &graphql.Field{Type: graphql.Boolean},
			"retryable":           &graphql.Field{Type: graphql.Boolean},
			"system_message":      &graphql.Field{Type: graphql.String},
			"project_message":     &graphql.Field{Type: graphql.String},
			"local_status":        &graphql.Field{Type: graphql.String},
			"remote_status":       &graphql.Field{Type: graphql.String},
			"status":              &graphql.Field{Type: graphql.String},
			"error":               &graphql.Field{Type: graphql.String},
			"attempt_count":       &graphql.Field{Type: graphql.Int},
			"created_at":          &graphql.Field{Type: graphql.String},
			"updated_at":          &graphql.Field{Type: graphql.String},
		},
	})
	approveType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ApproveSchemaChangesResponse",
		Fields: graphql.Fields{
			"version":      &graphql.Field{Type: graphql.Int},
			"changeset_id": &graphql.Field{Type: graphql.String},
			"message":      &graphql.Field{Type: graphql.String},
			"status":       &graphql.Field{Type: graphql.String},
			"total":        &graphql.Field{Type: graphql.Int},
			"done":         &graphql.Field{Type: graphql.Int},
		},
	})

	s.SystemQueriesChan <- &graphql.Fields{
		"schemaVersioningStatus": &graphql.Field{
			Type: statusType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				project, _, err := s.schemaDraftProject(p)
				if err != nil {
					return nil, err
				}
				return api.Status(p.Context, project)
			},
		},
		"schemaDiff": &graphql.Field{
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"changeset_id": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				project, _, err := s.schemaDraftProject(p)
				if err != nil {
					return nil, err
				}
				csID, _ := p.Args["changeset_id"].(string)
				return api.Diff(p.Context, project, csID)
			},
		},
		"schemaPreview": &graphql.Field{
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"source":  &graphql.ArgumentConfig{Type: graphql.String},
				"version": &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				project, _, err := s.schemaDraftProject(p)
				if err != nil {
					return nil, err
				}
				source, _ := p.Args["source"].(string)
				version := GetArgInt(p.Args, "version", 0)
				return api.Preview(p.Context, project, source, version)
			},
		},
		"schemaChangeEvents": &graphql.Field{
			Type: graphql.NewList(eventType),
			Args: graphql.FieldConfigArgument{
				"changeset_id": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				project, _, err := s.schemaDraftProject(p)
				if err != nil {
					return nil, err
				}
				csID, _ := p.Args["changeset_id"].(string)
				return api.Events(p.Context, project, csID)
			},
		},
		"schemaChangeExecutionRecords": &graphql.Field{
			Type: graphql.NewList(recordType),
			Args: graphql.FieldConfigArgument{
				"changeset_id": &graphql.ArgumentConfig{Type: graphql.String},
				"version":      &graphql.ArgumentConfig{Type: graphql.Int},
				"limit":        &graphql.ArgumentConfig{Type: graphql.Int},
				"offset":       &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				project, _, err := s.schemaDraftProject(p)
				if err != nil {
					return nil, err
				}
				csID, _ := p.Args["changeset_id"].(string)
				return api.Records(
					p.Context,
					project,
					csID,
					GetArgInt(p.Args, "version", 0),
					GetArgInt(p.Args, "limit", 500),
					GetArgInt(p.Args, "offset", 0),
				)
			},
		},
	}

	s.SystemMutationsChan <- &graphql.Fields{
		"approveSchemaChanges": &graphql.Field{
			Type: approveType,
			Args: graphql.FieldConfigArgument{
				"changeset_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"message":      &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				project, actor, err := s.schemaDraftProject(p)
				if err != nil {
					return nil, err
				}
				csID, _ := p.Args["changeset_id"].(string)
				msg, _ := p.Args["message"].(string)
				return api.Approve(p.Context, project, csID, msg, actor)
			},
		},
		"discardSchemaDraft": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"changeset_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				project, _, err := s.schemaDraftProject(p)
				if err != nil {
					return nil, err
				}
				csID, _ := p.Args["changeset_id"].(string)
				return api.Discard(p.Context, project, csID)
			},
		},
	}
}

func (s *GraphQLServer) schemaDraftProject(p graphql.ResolveParams) (*models.Project, string, error) {
	router, _ := p.Context.Value("router").(echo.Context)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, "", err
	}
	if cache == nil || cache.Project == nil {
		return nil, "", echo.NewHTTPError(400, "project is required")
	}
	actor := ""
	if cache.Param != nil {
		actor = cache.Param.UserID
	}
	return cache.Project, actor, nil
}
