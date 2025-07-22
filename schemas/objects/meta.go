package objects

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/tailor-inc/graphql"
	"github.com/teivah/onecontext"
)

var userType = graphql.NewObject(graphql.ObjectConfig{
	Name: "UserMetaFields",
	Fields: graphql.Fields{
		"id": &graphql.Field{
			Type: graphql.String,
		},
		"name": &graphql.Field{
			Type: graphql.String,
		},
		"avatar": &graphql.Field{
			Type: graphql.String,
		},
		"role": &graphql.Field{
			Type: graphql.String,
		},
	},
})

func BuildMetaObject(ctx context.Context, projectId string) *graphql.Field {
	return &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "MetaFields",
			Fields: graphql.Fields{
				"created_at": &graphql.Field{
					Type: graphql.String,
				},
				"updated_at": &graphql.Field{
					Type: graphql.String,
				},
				"status": &graphql.Field{
					Type: graphql.Boolean,
				},
				"created_by": &graphql.Field{
					Name: "object_created_by",
					Type: userType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// get title from source
						source := p.Source.(*models.MetaField)
						var (
							v        = p.Context.Value
							_loaders = v("cache").(*models.ApplicationCache).Dataloaders
							key      = models.NewResolverKey(source.SourceID, source)
							lid      = "system_user_loader"
						)

						typeRelation := context.WithValue(ctx, "relation", map[string]interface{}{
							"project_id":  projectId,
							"relation":    lid,
							"system_user": true, // this could be tanent user loader in the future , not using this now but will in future | doc.Meta.CreatedBy.ProjectUser is related
							//"relation_type": "has_one", // no need always one
						})
						tx, closeContext := onecontext.Merge(p.Context, typeRelation)
						defer closeContext()

						thunk := _loaders[lid].Load(tx, key)
						return func() (interface{}, error) {
							return thunk()
						}, nil
					},
				},
				"last_modified_by": &graphql.Field{
					Name: "object_last_modified_by",
					Type: userType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// get title from source
						source := p.Source.(*models.MetaField)
						var (
							v        = p.Context.Value
							_loaders = v("cache").(*models.ApplicationCache).Dataloaders
							key      = models.NewResolverKey(source.SourceID, source)
							lid      = "system_user_loader"
						)

						typeRelation := context.WithValue(ctx, "relation", map[string]interface{}{
							"project_id":  projectId,
							"relation":    lid,
							"system_user": true, // this could be tanent user loader in the future , not using this now but will in future | doc.Meta.CreatedBy.ProjectUser is related
							//"relation_type": "has_one", // no need always one
						})
						tx, closeContext := onecontext.Merge(p.Context, typeRelation)
						defer closeContext()

						thunk := _loaders[lid].Load(tx, key)
						return func() (interface{}, error) {
							return thunk()
						}, nil
					},
				},
				"resource_id": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Description: fmt.Sprintf("Meta Fields"),
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			// get title from source
			source := p.Source.(*types.DefaultDocumentStructure)
			source.Meta.SourceID = source.ID
			return source.Meta, nil
		},
	}
}
