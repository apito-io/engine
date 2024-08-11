package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/utility"
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"golang.org/x/oauth2"
)

type authCtrl struct {
	Cfg           *models.Config
	graphQLServer *resolver.GraphQLServer

	googleOauthConfig *oauth2.Config
	githubOauthConfig *oauth2.Config
}

func GetAuthController(cfg *models.Config, commonFn *resolver.GraphQLServer) *authCtrl {
	/*cognito, err := services.NewApitoTokenService(cfg, gqlServer.SystemDriver)
	if err != nil {
		return nil
	}*/

	return &authCtrl{
		Cfg: cfg,
		//authService:  cognito,
		graphQLServer: commonFn,
	}
}

func (a *authCtrl) ProjectSwitchV2(c echo.Context) error {
	var req *protobuff.ProjectCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Badboy, Jason ...",
			Code:    http.StatusBadRequest,
		})
	}

	if req.Id == "" {
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

	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	param := &shared.CommonSystemParams{
		UserId: userId.(string),
		//ProjectId:            projectID.(string),
		SystemCollectionName:            "projects",
		IsEntireCollectionSearchRequest: true,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"where": map[string]interface{}{
					"owner_id": map[string]interface{}{
						"eq": userId.(string),
					},
				},
				"limit": 0, // no limit
			},
		},
	}

	// query projects
	resp, err := a.graphQLServer.SystemDriver.ListProjects(ctx, param)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	var project *protobuff.Project
	for _, p := range resp.Results {
		if p.Id == req.Id {
			project = p
			break
		}
	}

	user.CurrentProjectId = project.Id

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	param.SystemCollectionName = "users"
	param.ResolveParams = &graphql.ResolveParams{
		Args: map[string]interface{}{
			"_id":   project.OwnerId,
			"limit": 1,
		},
	}

	// project switched so empty the cache
	//a.gqlServer.Param.ProjectId = project.Id

	// refresh the token
	tokens, err := a.graphQLServer.AuthService.ExchangeAndRefreshToken(ctx, user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", tokens.IDToken, false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "accessToken", tokens.AccessToken, true, false))

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "enter_project_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano()/1e6), false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "project_id", project.Id, false, false))

	// update project cache
	err = a.graphQLServer.ProjectCache.Expire(ctx, project.Id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	project.Schema = nil

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: "Project Switched",
		Body:    project,
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
		return c.JSON(http.StatusForbidden, models.HttpResponse{
			Code:  http.StatusForbidden,
			Error: err.Error(),
		})
	}

	user.CurrentProjectId = ""
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
		return c.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:  http.StatusBadRequest,
			Error: err.Error(),
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

	var passChangeReq *protobuff.PassChangeRequest
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

	var loginRequest *protobuff.LoginRequest
	if err := c.Bind(&loginRequest); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if loginRequest.Username == "" || loginRequest.Secret == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "either username or secret is missing",
			Code:    http.StatusBadRequest,
		})
	}
	ctx := c.Request().Context()

	user, err := a.graphQLServer.SystemDriver.GetSystemUserByUsername(ctx, loginRequest.Username)
	if err != nil {
		return c.JSON(http.StatusForbidden, models.HttpResponse{
			Code:  http.StatusForbidden,
			Error: err.Error(),
		})
	}

	if user == nil || user.Id == "" {
		return c.JSON(http.StatusForbidden, models.HttpResponse{
			Code:  http.StatusForbidden,
			Error: "user not found",
		})
	}

	project, err := a.graphQLServer.SystemDriver.GetProject(ctx, user.CurrentProjectId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:  http.StatusBadRequest,
			Error: err.Error(),
		})
	}

	if project == nil {
		return c.JSON(http.StatusInternalServerError, models.HttpResponse{
			Code:  http.StatusInternalServerError,
			Error: "no project data found",
		})
	}

	tokens, err := a.graphQLServer.AuthService.Login(ctx, loginRequest, user)
	if err != nil {
		return c.JSON(http.StatusForbidden, models.HttpResponse{
			Code:  http.StatusForbidden,
			Error: err.Error(),
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

	var registerRequest *protobuff.RegisterRequest
	if err := c.Bind(&registerRequest); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	// validate input
	if registerRequest.User.Username == "" || registerRequest.User.Email == "" || registerRequest.User.Secret == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "either Username, Email or Secret is missing",
			Code:    http.StatusBadRequest,
		})
	}

	user, err := a.graphQLServer.SystemDriver.GetSystemUserByUsername(ctx, registerRequest.User.Username)
	if err != nil {
		return c.JSON(http.StatusForbidden, models.HttpResponse{
			Code:  http.StatusForbidden,
			Error: err.Error(),
		})
	}

	if user != nil {
		return c.JSON(http.StatusForbidden, models.HttpResponse{
			Code:  http.StatusForbidden,
			Error: "this email is already used. please use another one",
		})
	}

	user, err = a.graphQLServer.AuthService.Signup(ctx, registerRequest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:  http.StatusBadRequest,
			Error: err.Error(),
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

	var registerRequest *protobuff.RegisterRequest
	if err := c.Bind(&registerRequest); err != nil {
		// if json bind error then 400 bad request
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Error: err.Error(),
			Code:  http.StatusBadRequest,
		})
	}

	// validate input
	if registerRequest.User.Secret == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Error: "Cant do",
			Code:  http.StatusBadRequest,
		})
	}

	err := a.graphQLServer.AuthService.ConfirmSignup(ctx, registerRequest)
	// handle possible exceptions
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Error: err.Error(),
			Code:  http.StatusBadRequest,
		})
	}

	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "logged_in_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano() / 1e6), false))
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"message": "Email verified",
	})
}

func (a *authCtrl) ForgetPasswordRequestV2(c echo.Context) error {

	var registerRequest *protobuff.RegisterRequest
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
			Error: "Demo account password can not be recovered. Stop messing around.",
			Code:  http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	err := a.graphQLServer.AuthService.ForgetPasswordRequest(ctx, registerRequest)
	// handle possible exceptions
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Error: err.Error(),
			Code:  http.StatusBadRequest,
		})
	}

	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "logged_in_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano() / 1e6), false))
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"message": "Check Email for a Verification Code",
	})
}

func (a *authCtrl) ForgetPasswordConfirmedV2(c echo.Context) error {

	var registerRequest *protobuff.RegisterRequest
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
			Error: "New Password is Missing",
			Code:  http.StatusBadRequest,
		})
	}

	err := a.graphQLServer.AuthService.ConfirmForgetPassword(nil, registerRequest)
	// handle possible exceptions
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Error: err.Error(),
			Code:  http.StatusBadRequest,
		})
	}

	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "logged_in_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano() / 1e6), false))
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":  http.StatusOK,
		"error": "Check Email for a Verification Code",
	})
}

func captureInternalServerError(err error) error {
	sentry.CaptureException(err)
	sentry.Flush(time.Second * 2)
	return err
}

func (a *authCtrl) errorHandler(router echo.Context, response *models.HttpResponse) {
	router.JSON(int(response.Code), response)
}

func fetchFromCookies(r *http.Request, name string) (string, error) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", ae.TokenIsRequired
	}
	return c.Value, nil
}
