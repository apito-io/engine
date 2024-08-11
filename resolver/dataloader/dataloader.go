package dataloader

import (
	"context"
	"errors"

	"github.com/apito-io/buffers/interfaces"
	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/graph-gophers/dataloader"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/tailor-inc/graphql"
)

type SystemDataloader struct {
	Cfg             *models.Config
	systemDriver    interfaces.SystemDBInterface
	Logger          *logrus.Logger
	DataloaderCache dataloader.Cache
}

func GetSystemDataloader(systemDriver interfaces.SystemDBInterface) (*SystemDataloader, error) {

	c := &dataloader.NoCache{}

	return &SystemDataloader{
		systemDriver:    systemDriver,
		DataloaderCache: c,
	}, nil
}

var handleError = func(err error) []*dataloader.Result {
	var results []*dataloader.Result
	var result dataloader.Result
	result.Error = err
	results = append(results, &result)
	return results
}

func (s *SystemDataloader) UserLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

	/*	var (
			v = ctx.Value
			//cache = v("cache").(*shared.ApplicationCache).Dataloaders
			router = v("router").(echo.Context)
		)

		var projectID string
		_projectID := router.Get("project")
		if _projectID != nil {
			projectID = _projectID.(string)
		}*/

	var results []*dataloader.Result

	var _ids []string
	for _, id := range keys {
		_ids = append(_ids, id.String())
	}

	param := &shared.CommonSystemParams{
		///ProjectId:                       projectID,
		IsEntireCollectionSearchRequest: true,
		SystemCollectionName:            "users",
		DocumentIDs:                     _ids,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"_ids":  _ids,
				"limit": 0,
				//"conn"
			},
		},
	}

	res, err := s.systemDriver.SearchUsers(ctx, param)
	if err != nil {
		return handleError(err)
	}

	users := res.Results

	for _, key := range keys {
		if user := utility.FilterUserArray(&users, key.String()); user != nil {
			results = append(results, &dataloader.Result{
				Data:  user,
				Error: nil,
			})
		} else {
			results = append(results, &dataloader.Result{
				Data:  nil,
				Error: errors.New("error on fetching user"),
			})
		}
	}
	return results
}

func (s *SystemDataloader) ProjectLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

	/*	var (
			v = ctx.Value
			//cache = v("cache").(*shared.ApplicationCache).Dataloaders
			router = v("router").(echo.Context)
		)

		var projectID string
		_projectID := router.Get("project")
		if _projectID != nil {
			projectID = _projectID.(string)
		}*/

	var results []*dataloader.Result

	var _ids []string
	for _, id := range keys {
		_ids = append(_ids, id.String())
	}

	param := &shared.CommonSystemParams{
		///ProjectId:                       projectID,
		IsEntireCollectionSearchRequest: true,
		SystemCollectionName:            "projects",
		DocumentIDs:                     _ids,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"_ids":  _ids,
				"limit": 0,
				//"conn"
			},
		},
	}

	res, err := s.systemDriver.ListProjects(ctx, param)
	if err != nil {
		return handleError(err)
	}

	projects := res.Results

	for _, key := range keys {
		if project := utility.FilterProjectArray(&projects, key.String()); project != nil {
			results = append(results, &dataloader.Result{
				Data:  project,
				Error: nil,
			})
		} else {
			results = append(results, &dataloader.Result{
				Data:  nil,
				Error: errors.New("error on fetching project"),
			})
		}
	}
	return results
}

func (s *SystemDataloader) buildCommonSystemParam(i echo.Context) (*shared.CommonSystemParams, error) {

	param := shared.CommonSystemParams{}

	projectID := i.Get("project")
	if projectID != nil {
		param.ProjectId = projectID.(string)
	}

	userId := i.Get("user")
	if userId != nil {
		param.UserId = userId.(string)
	}

	email := i.Get("email")
	if email != nil {
		param.Email = email.(string)
	}

	return &param, nil
}
