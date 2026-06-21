package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	_const "github.com/apito-io/engine/const"
	"golang.org/x/crypto/bcrypt"
	"gitlab.com/apito.io/open_driver/project/bbolt"
	"gitlab.com/apito.io/open_driver/project/mongo"
	"gitlab.com/apito.io/open_driver/project"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/utility"
	"github.com/getsentry/sentry-go"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

type AuthController struct {
	Cfg           *models.Config
	graphQLServer *resolver.GraphQLServer
}

func GetAuthController(cfg *models.Config, commonFn *resolver.GraphQLServer) *AuthController {
	return &AuthController{
		Cfg:           cfg,
		graphQLServer: commonFn,
	}
}

func fetchFromCookies(r *http.Request, name string) (string, error) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", ae.TokenIsRequired
	}
	return c.Value, nil
}

func (a *AuthController) normalJSONResponse(resp http.ResponseWriter, code int, msg interface{}) {
	js, _ := json.Marshal(msg)
	resp.WriteHeader(code)
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(js)
}

/*func (a *AuthController) setFacebookToken() http.Handler {
	fn := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		token, err := fb_oauth2.TokenFromContext(ctx)
		if err != nil {
			a.normalJSONErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		// Save the token
		facebookUser, err := facebook.UserFromContext(ctx)
		cred := &pb.CredentialRequest{
			Facebook: &pb.FacebookCred{
				Id:    facebookUser.ID,
				Email: facebookUser.Email,
				Token: token.AccessToken, // #todo save other info to
			},
		}

		accountsCtx, _ := context.WithTimeout(context.Background(), a.cfg.GRPCServiceTimeout)

		msg, err := a.profileService.Register(accountsCtx, cred)
		if err != nil {
			a.normalJSONErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		a.normalJSONResponse(w, http.StatusOK, msg)

	}
	return http.HandlerFunc(fn)
}*/

func (a *AuthController) Journey(c echo.Context) error {

	// Load configuration from environment variables and .env file
	var cfg models.Config
	err := cleanenv.ReadConfig(".env", &cfg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"code":    http.StatusBadRequest,
			"message": err.Error(),
			"stage":   "init",
		})
	}

	// If .env file doesn't exist, try to read from environment variables only
	err = cleanenv.ReadEnv(&cfg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"code":    http.StatusBadRequest,
			"message": err.Error(),
			"stage":   "init",
		})
	}

	if cfg.SystemDatabaseEngine == "" && cfg.SystemDBHost == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"code":    http.StatusOK,
			"message": "Journey",
			"stage":   "welcome",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"message": "Applicatin is Configured",
		"stage":   "welcome",
	})
}

type DatabaseRequest struct {
	File     string      `json:"file"`
	Type     string      `json:"type"`
	Host     string      `json:"host"`
	Port     interface{} `json:"port"`
	Database string      `json:"database"`
	Username string      `json:"username"`
	Password string      `json:"password"`
	SSLMode  string      `json:"ssl_mode"`
}

// DatabaseCheckPort coerces JSON port values for database check requests.
func DatabaseCheckPort(v interface{}, defaultPort string) string {
	switch x := v.(type) {
	case nil:
		return defaultPort
	case float64:
		if x == 0 {
			return defaultPort
		}
		return strconv.FormatInt(int64(x), 10)
	case int:
		if x == 0 {
			return defaultPort
		}
		return strconv.Itoa(x)
	case string:
		if x == "" {
			return defaultPort
		}
		return x
	default:
		return defaultPort
	}
}

func (a *AuthController) DatabaseCheck(c echo.Context) error {
	var req DatabaseRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	return DatabaseCheckCore(a, c, &req)
}

// DatabaseCheckCore runs open-core database connectivity checks (Mongo, SQL family, coredb). Pro wraps this for hosted providers.
func DatabaseCheckCore(a *AuthController, c echo.Context, req *DatabaseRequest) error {
	dbType := strings.ToLower(strings.TrimSpace(req.Type))

	switch dbType {
	case _const.MongoDBDriver:
		driver, err := mongo.GetProjectMongoDriver(a.Cfg, &models.DriverCredentials{
			Engine:   _const.MongoDBDriver,
			Host:     req.Host,
			Port:     DatabaseCheckPort(req.Port, "27017"),
			Database: req.Database,
			User:     req.Username,
			Password: req.Password,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
			})
		}
		err = driver.Ping()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
			})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"code":    http.StatusOK,
			"message": "Database connected successfully",
		})

	case _const.PostgreSQLDriver, _const.MySQLDriver, _const.SQLiteDriver, _const.SQLServerDriver, _const.MariaDBDriver:
		defPort := ""
		switch dbType {
		case _const.PostgreSQLDriver:
			defPort = "5432"
		case _const.MySQLDriver, _const.MariaDBDriver:
			defPort = "3306"
		}
		driver, err := project.GetProjectSQLDriver(a.Cfg, &models.DriverCredentials{
			File:     req.File,
			Engine:   dbType,
			Host:     req.Host,
			Port:     DatabaseCheckPort(req.Port, defPort),
			Database: req.Database,
			User:     req.Username,
			Password: req.Password,
			SSLMode:  req.SSLMode,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
			})
		}

		pinger, ok := driver.(interface{ Ping() error })
		if !ok {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: "driver does not support connectivity check",
				Code:    http.StatusInternalServerError,
			})
		}
		err = pinger.Ping()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
			})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"code":    http.StatusOK,
			"message": "Database connected successfully",
		})
	case _const.CoreDB:
		driver, err := bbolt.GetBBoltDriver(a.Cfg, &models.DriverCredentials{
			Engine: _const.CoreDB,
			File:   req.File,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
			})
		}
		err = driver.Ping()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
			})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"code":    http.StatusOK,
			"message": "Database connected successfully",
		})
	default:
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Invalid database type",
			Code:    http.StatusBadRequest,
		})
	}
}

func captureInternalServerError(err error) error {
	sentry.CaptureException(err)
	sentry.Flush(time.Second * 2)
	return err
}

func (a *AuthController) errorHandler(router echo.Context, response *models.HttpResponse) {
	router.JSON(int(response.Code), response)
}

func (a *AuthController) ProjectSwitchV2(c echo.Context) error {
	var req *models.ProjectCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
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
			Message: "user is missing in the token payload",
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

	if resp.User == nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "user is missing in the project with roles",
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

	// Register enriched credentials on the process-wide connection manager (same as project create).
	if _project.Driver != nil && a.graphQLServer != nil && a.graphQLServer.GraphQLExecutor != nil {
		d := *_project.Driver
		if strings.TrimSpace(d.ProjectID) == "" {
			d.ProjectID = _project.ID
		}
		ctxP := context.WithValue(ctx, "project_id", _project.ID)
		_ = a.graphQLServer.GraphQLExecutor.SetProjectDriverCredential(ctxP, &d)
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

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: "Project Switched",
		Body:    _project,
		Code:    http.StatusOK,
	})
}

func (a *AuthController) LogoutV2(c echo.Context) error {

	ctx := c.Request().Context()

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "user is missing in the token payload",
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

func (a *AuthController) ChangePasswordV2(c echo.Context) error {

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
			Message: "user is missing in the token payload",
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

// AdminResetPasswordV2 handles POST /admin/reset-password. Requires APITO_ADMIN_RESET_SECRET in request body and config.
func (a *AuthController) AdminResetPasswordV2(c echo.Context) error {
	var req models.AdminResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	if req.Email == "" || req.NewPassword == "" || req.AdminSecret == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "email, new_password and admin_secret are required",
			Code:    http.StatusBadRequest,
		})
	}
	if a.Cfg.AdminResetSecret == "" || req.AdminSecret != a.Cfg.AdminResetSecret {
		return c.JSON(http.StatusUnauthorized, &models.HttpResponse{
			Message: "invalid or missing admin secret",
			Code:    http.StatusUnauthorized,
		})
	}
	ctx := c.Request().Context()
	user, err := a.graphQLServer.SystemDriver.GetSystemUserByEmail(ctx, req.Email)
	if err != nil {
		return c.JSON(http.StatusNotFound, &models.HttpResponse{
			Message: "user not found",
			Code:    http.StatusNotFound,
		})
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "failed to hash password",
			Code:    http.StatusInternalServerError,
		})
	}
	user.Secret = string(hash)
	if err := a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true); err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusInternalServerError,
		})
	}
	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: "password reset successfully",
		Code:    http.StatusOK,
	})
}

func (a *AuthController) LoginV2(c echo.Context) error {

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
				Message: "user is missing in the token payload",
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

	users, err := a.graphQLServer.SystemDriver.SearchSystemUsers(ctx, param)
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

	//state := utility.NewID()
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

func (a *AuthController) RegisterV2(c echo.Context) error {

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
				Message: "user is missing in the token payload",
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

	users, err := a.graphQLServer.SystemDriver.SearchSystemUsers(ctx, param)
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

func (a *AuthController) VerifyV2(c echo.Context) error {

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

func (a *AuthController) ForgetPasswordRequestV2(c echo.Context) error {

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

func (a *AuthController) ForgetPasswordConfirmedV2(c echo.Context) error {

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
