package resolver

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-platform/graphql"
)

func (s *GraphQLServer) SystemUserMetaLoader(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

	handleError := func(err error) []*dataloader.Result {
		var results []*dataloader.Result
		var result dataloader.Result
		result.Error = err
		results = append(results, &result)
		return results
	}

	var (
		v        = ctx.Value
		relation = v("relation").(map[string]interface{})
	)

	var systemUserIds []string
	var projectUserIds []string
	for _, key := range keys {
		meta := key.(*models.ResolverKey).GetMeta()
		if meta.LastModifiedBy != nil {
			if meta.LastModifiedBy.ProjectUser {
				projectUserIds = append(projectUserIds, meta.LastModifiedBy.ID)
			} else {
				systemUserIds = append(systemUserIds, meta.LastModifiedBy.ID)
			}
		}

		if meta.CreatedBy != nil {
			if meta.CreatedBy.ProjectUser {
				projectUserIds = append(projectUserIds, meta.CreatedBy.ID)
			} else {
				systemUserIds = append(systemUserIds, meta.CreatedBy.ID)
			}
		}
	}

	var resp models.SearchResponse[models.SystemUser]
	if len(systemUserIds) > 0 {
		param := &models.CommonSystemParams{
			///ProjectId:                       projectID,
			IsEntireCollectionSearchRequest: true,
			SystemCollectionName:            "users",
			Role: &models.Role{
				ID:      "admin",
				IsAdmin: true,
			},
			//DocumentIDs: systemUserIds,
			ResolveParams: &graphql.ResolveParams{
				Args: map[string]interface{}{
					"_ids":  systemUserIds,
					"limit": 10,
					//"conn"
				},
			},
		}
		_resp, err := s.SystemDriver.SearchSystemUsers(context.Background(), param)
		if err != nil {
			return handleError(err)
		}
		resp.GroupedResults = _resp.GroupedResults
	}

	projectID := relation["project_id"].(string)

	var projectUsers map[string]*types.DefaultDocumentStructure
	if len(projectUserIds) > 0 {
		driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
		if err != nil {
			return handleError(err)
		}
		projectUsers, err = driver.GetProjectUsers(ctx, projectID, projectUserIds)
		if err != nil {
			return handleError(err)
		}
	}

	var results []*dataloader.Result

	for _, key := range keys {
		meta := key.(*models.ResolverKey).GetMeta()
		if meta.CreatedBy != nil {
			if user, ok := projectUsers[meta.CreatedBy.ID]; ok && meta.CreatedBy.ProjectUser {
				if name, ok := user.Data["first_name"].(string); ok {
					meta.CreatedBy.FirstName = name
				} else {
					meta.CreatedBy.Email = user.Data["email"].(string)
				}
				if avatar, ok := user.Data["avatar"].(map[string]interface{}); ok && avatar["url"] != nil {
					meta.CreatedBy.Avatar = avatar["url"].(string)
				}
			} else {
				if _user, ok := resp.GroupedResults[meta.CreatedBy.ID]; ok && _user != nil {
					user := _user[0]
					meta.CreatedBy.FirstName = user.FirstName
					meta.CreatedBy.Avatar = user.Avatar
				}
			}
		}

		if meta.LastModifiedBy != nil {
			if user, ok := projectUsers[meta.LastModifiedBy.ID]; ok && meta.LastModifiedBy.ProjectUser {
				if name, ok := user.Data["first_name"].(string); ok {
					meta.LastModifiedBy.FirstName = name
				} else {
					meta.LastModifiedBy.Email = user.Data["email"].(string)
				}
				if avatar, ok := user.Data["avatar"].(map[string]interface{}); ok && avatar["url"] != nil {
					meta.LastModifiedBy.Avatar = avatar["url"].(string)
				}
			} else {
				if _user, ok := resp.GroupedResults[meta.CreatedBy.ID]; ok && _user != nil {
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
