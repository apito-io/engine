package controller

import (
	"encoding/json"
	"github.com/apito-io/engine/database/system/driver/mongodb"
	"github.com/apito-io/engine/database/system/driver/sql"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/getsentry/sentry-go"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/labstack/echo/v4"

	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
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

	// Scopes define the level of access you are requesting from the user
	var googleOauthConfig = &oauth2.Config{
		ClientID:     cfg.GoogleOauthClientID,
		ClientSecret: cfg.GoogleOauthClientSecret,
		RedirectURL:  cfg.GoogleOauthRedirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile", "openid"},
		Endpoint:     google.Endpoint,
	}

	// Replace these with your GitHub OAuth app's Client ID and Client Secret
	var githubOauthConfig = &oauth2.Config{
		ClientID:     cfg.GithubOauthClientID,
		ClientSecret: cfg.GithubOauthClientSecret,
		RedirectURL:  cfg.GithubOauthRedirectURL,
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint,
	}

	return &authCtrl{
		Cfg: cfg,
		//authService:  cognito,
		graphQLServer:     commonFn,
		googleOauthConfig: googleOauthConfig,
		githubOauthConfig: githubOauthConfig,
	}
}

func fetchFromCookies(r *http.Request, name string) (string, error) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", ae.TokenIsRequired
	}
	return c.Value, nil
}

func (a *authCtrl) normalJSONResponse(resp http.ResponseWriter, code int, msg interface{}) {
	js, _ := json.Marshal(msg)
	resp.WriteHeader(code)
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(js)
}

/*func (a *authCtrl) setFacebookToken() http.Handler {
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

func (a *authCtrl) Journey(c echo.Context) error {

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
		"stage":   "done",
	})
}

type DatabaseRequest struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *authCtrl) DatabaseTest(c echo.Context) error {

	var req DatabaseRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	switch req.Type {
	case "mongodb":
		driver, err := mongodb.GetMongoDriver(&models.DriverCredentials{
			Engine:   req.Type,
			Host:     req.Host,
			Port:     req.Port,
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

	case "postgresql", "mysql", "sqlite", "sqlServer", "mariadb":
		driver, err := sql.GetSystemSQLDriver(&models.DriverCredentials{
			Engine:   req.Type,
			Host:     req.Host,
			Port:     req.Port,
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
	default:
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Invalid database type",
			Code:    http.StatusBadRequest,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"message": "DemoProjectSwitch",
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
