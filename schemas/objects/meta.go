package objects

import (
	"context"
	"fmt"

	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/engine/models"
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
	},
})

func BuildMetaObject(ctx context.Context, projectId string, needsResolver bool) *graphql.Field {
	fields := &graphql.Field{
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
				},
				"last_modified_by": &graphql.Field{
					Name: "object_last_modified_by",
					Type: userType,
				},
			},
		}),
		Description: fmt.Sprintf("Meta Fields"),
	}
	if needsResolver {
		fields.Resolve = func(p graphql.ResolveParams) (interface{}, error) {
			// get title from source
			source := p.Source.(*shared.DefaultDocumentStructure)
			var (
				v        = p.Context.Value
				_loaders = v("cache").(*shared.ApplicationCache).Dataloaders
				key      = models.NewResolverKey(source.Id, source.Meta)
				lid      = "meta_loader"
			)

			typeRelation := context.WithValue(ctx, "relation", map[string]string{
				"project_id": projectId,
				"relation":   lid,
				//"relation_type": "has_one", // no need always one
			})
			tx, closeContext := onecontext.Merge(p.Context, typeRelation)
			defer closeContext()

			thunk := _loaders[lid].Load(tx, key)
			return func() (interface{}, error) {
				return thunk()
			}, nil
		}
	}
	return fields
}
