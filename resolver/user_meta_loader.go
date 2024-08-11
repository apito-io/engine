package resolver

import (
	"context"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/engine/models"
	"github.com/graph-gophers/dataloader"
)

func (s *GraphQLServer) MetaLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

	handleError := func(err error) []*dataloader.Result {
		var results []*dataloader.Result
		var result dataloader.Result
		result.Error = err
		results = append(results, &result)
		return results
	}

	var (
		v        = ctx.Value
		relation = v("relation").(map[string]string)
	)

	var systemUserIds []string
	var projectUserIds []string
	for _, key := range keys {
		meta := key.(*models.ResolverKey).GetMeta()
		if meta.LastModifiedBy != nil {
			if meta.LastModifiedBy.ProjectUser {
				projectUserIds = append(projectUserIds, meta.LastModifiedBy.Id)
			} else {
				systemUserIds = append(systemUserIds, meta.LastModifiedBy.Id)
			}
		}

		if meta.CreatedBy != nil {
			if meta.CreatedBy.ProjectUser {
				projectUserIds = append(projectUserIds, meta.CreatedBy.Id)
			} else {
				systemUserIds = append(systemUserIds, meta.CreatedBy.Id)
			}
		}
	}

	var resp shared.SearchResponse[protobuff.SystemUser]
	var err error
	if len(systemUserIds) > 0 {
		_resp, err := s.SystemDriver.SearchUsers(ctx, &shared.CommonSystemParams{})
		if err != nil {
			return handleError(err)
		}
		resp.GroupedResults = _resp.GroupedResults
	}

	var projectUsers map[string]*shared.DefaultDocumentStructure
	if len(projectUserIds) > 0 {
		projectUsers, err = s.GraphQLExecutor.GetProjectDriver(ctx).GetProjectUsers(ctx, relation["project_id"], projectUserIds)
		if err != nil {
			return handleError(err)
		}
	}

	var results []*dataloader.Result

	for _, key := range keys {
		meta := key.(*models.ResolverKey).GetMeta()
		if meta.CreatedBy != nil {
			if user, ok := projectUsers[meta.CreatedBy.Id]; ok && meta.CreatedBy.ProjectUser {
				if name, ok := user.Data["first_name"].(string); ok {
					meta.CreatedBy.FirstName = name
				} else {
					meta.CreatedBy.Email = user.Data["email"].(string)
				}
				if avatar, ok := user.Data["avatar"].(map[string]interface{}); ok && avatar["url"] != nil {
					meta.CreatedBy.Avatar = avatar["url"].(string)
				}
			} else {
				if _user, ok := resp.GroupedResults[meta.CreatedBy.Id]; ok && _user != nil {
					user := _user[0]
					meta.CreatedBy.FirstName = user.FirstName
					meta.CreatedBy.Avatar = user.Avatar
				}
			}
		}

		if meta.LastModifiedBy != nil {
			if user, ok := projectUsers[meta.LastModifiedBy.Id]; ok && meta.LastModifiedBy.ProjectUser {
				if name, ok := user.Data["first_name"].(string); ok {
					meta.LastModifiedBy.FirstName = name
				} else {
					meta.LastModifiedBy.Email = user.Data["email"].(string)
				}
				if avatar, ok := user.Data["avatar"].(map[string]interface{}); ok && avatar["url"] != nil {
					meta.LastModifiedBy.Avatar = avatar["url"].(string)
				}
			} else {
				if _user, ok := resp.GroupedResults[meta.CreatedBy.Id]; ok && _user != nil {
					user := _user[0]
					meta.CreatedBy.FirstName = user.FirstName
					meta.CreatedBy.Avatar = user.Avatar
				}
			}
		}

		results = append(results, &dataloader.Result{
			Data:  meta, // because it has only one
			Error: nil,
		})
	}

	return results
}
