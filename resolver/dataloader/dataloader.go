package dataloader

import (
	"context"
	"errors"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/graph-gophers/dataloader"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/tailor-inc/graphql"
)

type SystemDataloader struct {
	Cfg             *models.Config
	systemDriver    interfaces.ApitoSystemDB
	Logger          *logrus.Logger
	DataloaderCache dataloader.Cache
}

var handleError = func(err error) []*dataloader.Result {
	var results []*dataloader.Result
	var result dataloader.Result
	result.Error = err
	results = append(results, &result)
	return results
}

func GetSystemDataloader(systemDriver interfaces.ApitoSystemDB) (*SystemDataloader, error) {

	c := &dataloader.NoCache{}

	return &SystemDataloader{
		systemDriver:    systemDriver,
		DataloaderCache: c,
	}, nil
}

func (s *SystemDataloader) OrganizationsTeamsLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

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

	param := &models.CommonSystemParams{
		///ProjectId:                       projectID,
		IsEntireCollectionSearchRequest: true,
		SystemCollectionName:            "users",
		Role: &models.Role{
			ID:      "admin",
			IsAdmin: true,
		},
		DocumentIDs: _ids,
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

func (s *SystemDataloader) OrganizationsUsersLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

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

	param := &models.CommonSystemParams{
		///ProjectId:                       projectID,
		IsEntireCollectionSearchRequest: true,
		SystemCollectionName:            "users",
		Role: &models.Role{
			ID:      "admin",
			IsAdmin: true,
		},
		DocumentIDs: _ids,
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

func (s *SystemDataloader) OrganizationsLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

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

	param := &models.CommonSystemParams{
		///ProjectId:                       projectID,
		IsEntireCollectionSearchRequest: true,
		SystemCollectionName:            "users",
		Role: &models.Role{
			ID:      "admin",
			IsAdmin: true,
		},
		DocumentIDs: _ids,
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

	param := &models.CommonSystemParams{
		///ProjectId:                       projectID,
		IsEntireCollectionSearchRequest: true,
		SystemCollectionName:            "users",
		Role: &models.Role{
			ID:      "admin",
			IsAdmin: true,
		},
		DocumentIDs: _ids,
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

	param := &models.CommonSystemParams{
		///ProjectId:                       projectID,
		IsEntireCollectionSearchRequest: true,
		SystemCollectionName:            "projects",
		Role: &models.Role{
			ID:      "admin",
			IsAdmin: true,
		},
		DocumentIDs: _ids,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"_ids":  _ids,
				"limit": 0,
				//"conn"
			},
		},
	}

	res, err := s.systemDriver.SearchProjects(ctx, param)
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

func (s *SystemDataloader) buildCommonSystemParam(i echo.Context) (*models.CommonSystemParams, error) {

	param := models.CommonSystemParams{}

	projectID := i.Get("project")
	if projectID != nil {
		param.ProjectID = projectID.(string)
	}

	role := i.Get("role")
	if role == nil || role == "" {
		return nil, errors.New("role is required for this operation")
	}
	param.Role = &models.Role{ID: role.(string)}

	projectPlan := i.Get("plan")
	if projectPlan != nil {
		param.Plan = projectPlan.(string)
	}

	userId := i.Get("user")
	if userId != nil {
		param.UserID = userId.(string)
	}

	email := i.Get("email")
	if email != nil {
		param.Email = email.(string)
	}

	readOnly := i.Get("read_only")
	if readOnly != nil {
		param.Role.ReadOnlyProject = readOnly.(bool)
	}

	isProjectUser := i.Get("is_project_user")
	if isProjectUser != nil {
		param.Role.IsProjectUser = isProjectUser.(bool)
	}

	if param.Role.ID == "admin" {
		param.Role.IsAdmin = true

	} else {
		if param.Role.ID == "demo" {
			param.Role.SystemGenerated = true
			param.Role.IsAdmin = false
		} else if param.Role.ID == "team" {
			param.Role.SystemGenerated = true
			param.Role.IsAdmin = false
		}
	}
	return &param, nil
}
