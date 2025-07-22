package controller

import (
	"fmt"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"net/http"
	"time"
)

func (a *authCtrl) GenerateAPIKey(c echo.Context) error {

	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	var tenantID string
	if _, ok := req["tenant_id"]; ok {
		tenantID = req["tenant_id"].(string)
	}

	userID := c.Get("user")
	if userID == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Nope, Can't Do it..! User!",
			Code:    http.StatusBadRequest,
		})
	}

	projectID := c.Get("project_id")
	if projectID == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "You have to switch to a project in order for this to work",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	t := services.GetBrankaToken(a.Cfg, a.graphQLServer.SystemDriver)

	apiKey, err := t.GenerateAPIKey(ctx, userID.(string), projectID.(string), tenantID, "api_key", time.Now().Add(time.Hour*24*30).Unix())
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Token: *apiKey,
		Code:  http.StatusOK,
	})
}

func (a *authCtrl) SyncProject(c echo.Context) error {

	type SyncProjectRequest struct {
		Token   string          `json:"token"`
		Project *models.Project `json:"project"`
	}

	var req SyncProjectRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Bad boy, Jason ...",
			Code:    http.StatusBadRequest,
		})
	}

	if req.Token == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "token is missing",
			Code:    http.StatusBadRequest,
		})
	}

	if req.Project == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "project is missing",
			Code:    http.StatusBadRequest,
		})
	}

	if req.Project.Schema == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "project does not contain any model. Nothing to sync",
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

	t := services.GetBrankaToken(a.Cfg, a.graphQLServer.SystemDriver)

	decodedToken, err := t.Validate(ctx, req.Token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	if decodedToken.UserID != userId {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "token is invalid",
			Code:    http.StatusBadRequest,
		})
	}

	if req.Project.SyncedProperty != nil {
		// check
	} else {
		req.Project.SyncedProperty = &models.SyncProject{
			ProjectID:       req.Project.ID,
			SyncedTokenUsed: req.Token,
			LocalProjectID:  req.Project.ID,
			LastSyncedAt:    utility.GetCurrentTime(),
		}
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body: req.Project.SyncedProperty,
		Code: http.StatusOK,
	})
}

func (a *authCtrl) ProjectSwitchV2(c echo.Context) error {
	var req *models.ProjectCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Bad boy, Jason ...",
			Code:    http.StatusBadRequest,
		})
	}

	if req.ID == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "What are you switching?",
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

	resp, err := a.graphQLServer.SystemDriver.CheckProjectWithRoles(ctx, userId.(string), req.ID)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	// project switched so empty the cache
	//a.gqlServer.Param.ProjectId = project.ID

	_project := resp.Project

	// refresh the token
	tokens, err := a.graphQLServer.AuthService.ExchangeAndRefreshToken(ctx, resp)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusBadRequest,
		})
	}

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", tokens.IDToken, false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "accessToken", tokens.AccessToken, true, false))

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "enter_project_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano()/1e6), false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "project_id", _project.ID, false, false))

	// update project cache
	err = a.graphQLServer.ProjectCache.Expire(ctx, _project.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if _project.Driver != nil && _project.Driver.Host == "" {
		// inject the default config
		_project.Driver.Host = a.Cfg.DefaultProjectDBHost
		_project.Driver.Port = a.Cfg.DefaultProjectDBPort
		_project.Driver.User = a.Cfg.DefaultProjectDBUser
		_project.Driver.Password = a.Cfg.DefaultProjectDBPassword
		_project.Driver.Database = a.Cfg.DefaultProjectDBName
	}

	/*projectDriver, err := project.GetProjectDriver(_project.Driver)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Oh,Boy .." + err.Error(),
			Code:    http.StatusBadRequest,
		})
	}*/

	//a.graphQLServer.GraphQLExecutor.SetProjectDriver(ctx, projectDriver)

	//time.Sleep(time.Second * 2)

	_project.Schema = nil
	_project.MicroServicePort = ""

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: "Project Switched",
		Body:    _project,
		Code:    http.StatusOK,
	})
}

func (a *authCtrl) LogoutV2(c echo.Context) error {

	ctx := c.Request().Context()

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Nope, Can't Do it..! User!",
			Code:    http.StatusBadRequest,
		})
	}

	// update user current project id
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	user.CurrentProjectID = ""
	user.ReadOnlyProject = false
	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	tokenString, err := fetchFromCookies(c.Request(), "accessToken")
	if err != nil {
		if err.Error() == "no token" {
			url := c.Request().URL
			url.Path = "/auth/v2/login"
			return c.Redirect(http.StatusTemporaryRedirect, "/auth/v2/login")
		}
	}

	err = a.graphQLServer.AuthService.Logout(ctx, tokenString)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	// handle possible exceptions

	// reset the http Only Cookies
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", "", true, true))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "accessToken", "", true, true))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "email", "", false, true))

	url := c.Request().URL
	url.Path = "/login"

	return c.Redirect(http.StatusTemporaryRedirect, url.String())
}

func (a *authCtrl) ChangePasswordV2(c echo.Context) error {

	ctx := c.Request().Context()

	var passChangeReq *models.PassChangeRequest
	if err := c.Bind(&passChangeReq); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
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

	// update user current project id
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	user, err = a.graphQLServer.AuthService.ChangePassword(ctx, user, passChangeReq.OldPassword, passChangeReq.NewPassword)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"message": "password has been changed",
	})
}

func (a *authCtrl) LoginV2(c echo.Context) error {

	var loginRequest *models.LoginRequest
	if err := c.Bind(&loginRequest); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if loginRequest.Email == "" || loginRequest.Secret == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Either email or password is empty",
			Code:    http.StatusBadRequest,
		})
	}
	ctx := c.Request().Context()

	/*
		userId := c.Get("user")
		if userId == nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "Nope, Can't Do it..! User!",
				Code:    http.StatusBadRequest,
			})
		}

		projectID := c.Get("project_id")
		if projectID == nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "Nope, Can't Do it..! Project!",
				Code:    http.StatusBadRequest,
			})
		}
	*/

	param := &models.CommonSystemParams{
		//UserId:    userId.(string),
		//ProjectId: projectID.(string),
		IsEntireCollectionSearchRequest: true,
		SystemCollectionName:            "users",
		Role: &models.Role{
			ID:      "admin",
			IsAdmin: true,
		},
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"where": map[string]interface{}{
					"email": map[string]interface{}{
						"eq": loginRequest.Email,
					},
				},
				"limit": 1,
			},
		},
	}

	users, err := a.graphQLServer.SystemDriver.SearchUsers(ctx, param)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	if len(users.Results) == 0 {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "user not found",
			Code:    http.StatusForbidden,
		})
	}

	user := users.Results[0]

	tokens, err := a.graphQLServer.AuthService.Login(ctx, loginRequest, user, nil)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	//state := uuid.New().String()
	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "state", state, true))

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", tokens.IDToken, false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "accessToken", tokens.AccessToken, true, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "email", loginRequest.Email, false, false))

	user.RefreshToken = tokens.RefreshToken
	user.AccessToken = tokens.AccessToken
	user.LastLoggedIn = utility.GetCurrentTime()

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	user.Secret = ""

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":  http.StatusOK,
		"token": tokens.IDToken,
		//"refresh_token": *tokens.AuthenticationResult.RefreshToken,
		"message": "Authenticated",
	})
}

func (a *authCtrl) RegisterV2(c echo.Context) error {

	ctx := c.Request().Context()

	var registerRequest *models.RegisterRequest
	if err := c.Bind(&registerRequest); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if registerRequest.User.Email == "" || registerRequest.User.Secret == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Cant do",
			Code:    http.StatusBadRequest,
		})
	}

	/*
		userId := c.Get("user")
		if userId == nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "Nope, Can't Do it..! User!",
				Code:    http.StatusBadRequest,
			})
		}

		projectID := c.Get("project_id")
		if projectID == nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "Nope, Can't Do it..! Project!",
				Code:    http.StatusBadRequest,
			})
		}
	*/

	param := &models.CommonSystemParams{
		//UserId:    userId.(string),
		//ProjectId: projectID.(string),
		SystemCollectionName: "users",
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"where": map[string]interface{}{
					"email": map[string]interface{}{
						"eq": registerRequest.User.Email,
					},
				},
			},
		},
	}

	users, err := a.graphQLServer.SystemDriver.SearchUsers(ctx, param)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	if len(users.Results) > 0 {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "this email is already used. please use another one",
			Code:    http.StatusForbidden,
		})
	}

	user, err := a.graphQLServer.AuthService.Signup(ctx, registerRequest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	user, err = a.graphQLServer.SystemDriver.CreateSystemUser(ctx, user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	user.Secret = ""
	user.AccessToken = ""
	user.RefreshToken = ""
	user.LastLoggedIn = ""

	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "logged_in_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano() / 1e6), false))
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"body":    user,
		"message": "Check Email for a Verification Code",
	})
}

func (a *authCtrl) VerifyV2(c echo.Context) error {

	ctx := c.Request().Context()

	var registerRequest *models.RegisterRequest
	if err := c.Bind(&registerRequest); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if registerRequest.User.Secret == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Cant do",
			Code:    http.StatusBadRequest,
		})
	}

	err := a.graphQLServer.AuthService.ConfirmSignup(ctx, registerRequest)
	// handle possible exceptions
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "logged_in_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano() / 1e6), false))
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"message": "Email verified",
	})
}

func (a *authCtrl) ForgetPasswordRequestV2(c echo.Context) error {

	var registerRequest *models.RegisterRequest
	if err := c.Bind(&registerRequest); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if registerRequest.User.Email == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Cant do",
			Code:    http.StatusBadRequest,
		})
	}

	if registerRequest.User.Email == "demo@apito.io" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Demo account password can not be recovered. Stop messing around.",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	err := a.graphQLServer.AuthService.ForgetPasswordRequest(ctx, registerRequest)
	// handle possible exceptions
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "logged_in_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano() / 1e6), false))
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"message": "Check Email for a Verification Code",
	})
}

func (a *authCtrl) ForgetPasswordConfirmedV2(c echo.Context) error {

	var registerRequest *models.RegisterRequest
	if err := c.Bind(&registerRequest); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if registerRequest.User.Email == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Email is Missing",
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if registerRequest.VerificationCode == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Cant do",
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if registerRequest.User.Secret == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "New Password is Missing",
			Code:    http.StatusBadRequest,
		})
	}

	err := a.graphQLServer.AuthService.ConfirmForgetPassword(nil, registerRequest)
	// handle possible exceptions
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "logged_in_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano() / 1e6), false))
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"message": "Check Email for a Verification Code",
	})
}
