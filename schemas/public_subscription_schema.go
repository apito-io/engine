package schemas

import (
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/scaler"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-platform/graphql"
)

// changeEventTypeEnum is the shared CREATED/UPDATED/DELETED enum.
var changeEventTypeEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "ChangeEventType",
	Values: graphql.EnumValueConfigMap{
		"CREATED": &graphql.EnumValueConfig{Value: models.ChangeEventCreated},
		"UPDATED": &graphql.EnumValueConfig{Value: models.ChangeEventUpdated},
		"DELETED": &graphql.EnumValueConfig{Value: models.ChangeEventDeleted},
	},
})

// PublicSubscriptionSchema builds the per-project GraphQL subscription schema
// served over websockets. It auto-generates one `<model>Changed` field per
// model the role may read, plus a generic `broadcast(channel)` field.
//
// The change-event `node` is a JSON scalar (the full document), keeping the
// schema small and DB-engine agnostic — mirroring Supabase postgres_changes.
func PublicSubscriptionSchema(server *resolver.GraphQLServer, cache *models.ApplicationCache) (*graphql.Schema, error) {
	if cache == nil || cache.Project == nil || cache.Project.Schema == nil {
		return nil, fmt.Errorf("subscription schema: project schema is nil")
	}
	if cache.Param == nil || cache.Param.Role == nil {
		return nil, fmt.Errorf("subscription schema: role is nil")
	}

	projectID := cache.Param.ProjectID
	role := cache.Param.Role

	changeEventObj := graphql.NewObject(graphql.ObjectConfig{
		Name: "ModelChangeEvent",
		Fields: graphql.Fields{
			"event":          &graphql.Field{Type: graphql.NewNonNull(changeEventTypeEnum)},
			"id":             &graphql.Field{Type: graphql.ID},
			"model":          &graphql.Field{Type: graphql.String},
			"node":           &graphql.Field{Type: scaler.ScalarJSON},
			"previousValues": &graphql.Field{Type: scaler.ScalarJSON},
		},
	})

	broadcastObj := graphql.NewObject(graphql.ObjectConfig{
		Name: "BroadcastMessage",
		Fields: graphql.Fields{
			"channel": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"event":   &graphql.Field{Type: graphql.String},
			"payload": &graphql.Field{Type: scaler.ScalarJSON},
			"at":      &graphql.Field{Type: graphql.String},
		},
	})

	subFields := graphql.Fields{}

	// Per-model change streams: <model>Changed
	for _, model := range cache.Project.Schema.Models {
		if model == nil || model.Name == "" {
			continue
		}
		if !resolver.ModelReadableForRole(role, model.Name) {
			continue
		}
		fieldName := utility.SingularResourceName(model.Name) + "Changed"
		subFields[fieldName] = &graphql.Field{
			Type: changeEventObj,
			Args: graphql.FieldConfigArgument{
				"events": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(changeEventTypeEnum))},
				"id":     &graphql.ArgumentConfig{Type: graphql.ID},
				"where":  &graphql.ArgumentConfig{Type: scaler.ScalarJSON},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source, nil
			},
			Subscribe: server.ModelChangedSubscribeFn(projectID, model.Name),
		}
	}

	// Generic broadcast channel
	subFields["broadcast"] = &graphql.Field{
		Type: broadcastObj,
		Args: graphql.FieldConfigArgument{
			"channel": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			return p.Source, nil
		},
		Subscribe: server.BroadcastSubscribeFn(projectID),
	}

	query := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"healthcheck": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "ok", nil
				},
			},
		},
	})

	subscription := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Subscription",
		Fields: subFields,
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:        query,
		Subscription: subscription,
		Types: []graphql.Type{
			scaler.ScalarJSON,
			scaler.ScalarJSONArray,
		},
	})
	if err != nil {
		return nil, err
	}
	return &schema, nil
}
