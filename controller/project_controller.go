package controller

import (
	"net/http"
	"strings"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/databasedriver/project"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

func contains(arr []string, str string) bool {
	for _, k := range arr {
		if k == str {
			return true
		}
	}
	return false
}

func (a *authCtrl) ProjectCreation(c echo.Context) error {

	var req *protobuff.Project
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Badboy, Jason ...",
			Code:    http.StatusBadRequest,
		})
	}

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Nope, Can't Do it..! User!",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	projectId := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(req.Id, " ", "_")))

	if req.Driver == nil {
		req.Driver = &protobuff.DriverCredentials{
			Engine:   a.Cfg.ProjectDatabaseEngine,
			Host:     a.Cfg.ProjectDatabaseDBConfig.Host,
			Port:     a.Cfg.ProjectDatabaseDBConfig.Port,
			User:     a.Cfg.ProjectDatabaseDBConfig.User,
			Password: a.Cfg.ProjectDatabaseDBConfig.Password,
			Database: a.Cfg.ProjectDatabaseDBConfig.Database,
		}
	}

	projectDriver, err := project.GetProjectDriver(req.Driver)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Oh,Boy .." + err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	a.graphQLServer.GraphQLExecutor.SetProjectDriver(ctx, projectDriver)

	ust := c.Get("ust")
	if ust == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Nope, Can't Do it..! User!",
			Code:    http.StatusBadRequest,
		})
	}

	req.XKey = req.Id
	req.OwnerId = userId.(string)

	projectIdCreated, err := a.graphQLServer.GraphQLExecutor.GetProjectDriver(ctx).AddCollection(ctx, projectId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	// update the engine of the project
	if projectIdCreated != nil && req.Driver != nil {
		req.Driver.Database = *projectIdCreated
	}

	_project, err := a.graphQLServer.SystemDriver.CreateProject(ctx, req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if user.CurrentProjectId == "" {
		user.CurrentProjectId = _project.Id
	}

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	// reverify again
	/*tokens, err := utility.NewRefreshTokenAuthenticator(a.Cfg, user.RefreshToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
		return
	}*/

	//token := tokens["id_token"].(string)
	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "userToken", token, true))

	if _project.ProjectTemplate != "" {
		// transfer the project
		err = a.graphQLServer.GraphQLExecutor.GetProjectDriver(ctx).TransferProject(ctx, userId.(string), _project.ProjectTemplate, _project.Id)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}

		/*err = a.graphQLServer.S3.TransferBucket(projectDetails.FromExample, projectId)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}*/
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body: _project,
		//Token: token,
		Code: http.StatusOK,
	})
}

func (a *authCtrl) ProjectNameCheck(c echo.Context) error {

	var req *protobuff.ProjectCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Badboy, Jason ...",
			Code:    http.StatusBadRequest,
		})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "project Name is required",
			Code:    http.StatusBadRequest,
		})
	}

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Nope, Can't Do it..! User!",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	err := a.graphQLServer.SystemDriver.CheckProjectName(ctx, req.Name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		//Token:  token,
		Body: req.Name,
		Code: http.StatusOK,
	})
}

func (a *authCtrl) ProjectList(c echo.Context) error {

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Nope, Can't Do it..! User!",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	param := &shared.CommonSystemParams{
		UserId: userId.(string),
		//ProjectId: projectID.(string),
		SystemCollectionName:            "projects",
		IsEntireCollectionSearchRequest: true,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"where": map[string]interface{}{
					"owner_id": map[string]interface{}{
						"eq": userId.(string),
					},
				},
				"limit": 100, //  #todo fixed for now will do pagination later
			},
		},
	}

	resp, err := a.graphQLServer.SystemDriver.ListProjects(ctx, param)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	projects := resp.Results

	var result []*protobuff.Project
	for _, p := range projects {
		_project := &protobuff.Project{
			Id:          p.Id,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
		}
		if p.Driver != nil {
			_project.Driver = &protobuff.DriverCredentials{Engine: p.Driver.Engine}
		}
		result = append(result, _project)
	}
	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body: result,
		Code: http.StatusOK,
	})
}

func (a *authCtrl) GetProfile(c echo.Context) error {

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "you need to be logged in to access this route",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	//subs, err :=  a.gqlServer.SystemDriver.ListSubscription("38c6bef1-a693-41e5-ad9b-a4e6a7e6dc24")
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	user.Secret = ""
	user.RefreshToken = ""
	user.AccessToken = ""

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body: user,
		Code: http.StatusOK,
	})
}

func (a *authCtrl) UpdateProfile(c echo.Context) error {

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "you need to be logged in to access this route",
			Code:    http.StatusBadRequest,
		})
	}

	var req *protobuff.SystemUser
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Badboy, Jason ...",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	//subs, err :=  a.gqlServer.SystemDriver.ListSubscription("38c6bef1-a693-41e5-ad9b-a4e6a7e6dc24")
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}

	if req.LastName != "" {
		user.LastName = req.LastName
	}

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body:    user,
		Message: "Profile Updated",
		Code:    http.StatusOK,
	})
}
