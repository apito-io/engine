package services

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/apito-io/buffers/interfaces"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type ApitoTokenService struct {
	cfg           *models.Config
	systemDB      interfaces.SystemDBInterface
	blankaService *BrankaToken
	authService   AuthServiceInterface
	dbWriteLock   sync.Mutex
}

func NewApitoTokenService(cfg *models.Config, auth AuthServiceInterface, driver interfaces.SystemDBInterface) (*ApitoTokenService, error) {

	return &ApitoTokenService{
		cfg:           cfg,
		systemDB:      driver,
		blankaService: GetBrankaToken(cfg, driver),
		authService:   auth,
		dbWriteLock:   sync.Mutex{},
	}, nil
}

// CustomResponseWriter is a wrapper around the standard http.ResponseWriter
// that captures the status code and response body.
type CustomResponseWriter struct {
	http.ResponseWriter
	Body   *bytes.Buffer
	Status int
}

// Header returns the header map that will be sent by WriteHeader.
func (w *CustomResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// WriteHeader captures the status code and calls the underlying WriteHeader.
func (w *CustomResponseWriter) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write captures the response body and calls the underlying Write.
func (w *CustomResponseWriter) Write(b []byte) (int, error) {
	w.Body.Write(b)
	return w.ResponseWriter.Write(b)
}

// getFunctionName returns the name of the function being executed
func getFunctionName(i interface{}) string {
	val := runtime.FuncForPC(reflect.ValueOf(i).Pointer())
	return val.Name()
}

func (t *ApitoTokenService) ApitoTokenHandlr(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {

		var token *string
		var err error

		requestPath := ctx.Request().URL.Path
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, map[string]interface{}{"message": "invalid graphql path or mis constructed url"})
		}

		useTokenFlag := ctx.Request().Header.Get("X-Use-Cookies")
		if ((requestPath == "/secured/graphql" || requestPath == "/secured/graphql/v2") || strings.HasPrefix(requestPath, "/secured/rest/")) && useTokenFlag == "" {
			token, err = tokenFromBearer(ctx.Request())
			if err != nil {
				return ctx.JSON(http.StatusUnauthorized, map[string]interface{}{"message": "invalid auth header or auth header missing"})
			}
			verifiedToken, err := t.blankaService.VerifyBlankaApiToken(ctx, *token)
			if err != nil {
				return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
			}
			fmt.Println("verifiedToken", verifiedToken)

		} else if useTokenFlag == "false" {
			token, err = tokenFromBearer(ctx.Request())
			if err != nil {
				return ctx.JSON(http.StatusUnauthorized, map[string]interface{}{"message": "invalid auth header or auth header missing"})
			}
			tokenClaims, err := t.authService.VerifyIDToken(ctx.Request().Context(), *token)
			if err != nil {
				return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
			}

			err = utility.SetTokenClaimsToRouter(ctx, tokenClaims)
			if err != nil {
				return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
			}

		} else {
			_ctx := ctx.Request().Context()

			tokens, err := tokenFromCookies(ctx.Request())
			if err != nil {
				return ctx.JSON(http.StatusUnauthorized, map[string]interface{}{"message": "invalid Cookie header. Reload the page"})
			}
			err = t.authService.VerifyAccessToken(_ctx, tokens.AccessToken)
			if err != nil {
				return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
			}

			tokenClaims, err := t.authService.VerifyIDToken(_ctx, tokens.IDToken)
			if err != nil {
				if errors.Is(err, ae.LOGIN_CONFICT) {
					// reset the http Only Cookies
					http.SetCookie(ctx.Response(), utility.SetTokenCookie(t.cfg, "userToken", "", true, true))
					http.SetCookie(ctx.Response(), utility.SetTokenCookie(t.cfg, "accessToken", "", true, true))
					http.SetCookie(ctx.Response(), utility.SetTokenCookie(t.cfg, "email", "", false, true))

					/*url := ctx.Request().URL
					url.Path = "/login"
					params := url.Query()
					params.Add("message", "logged in from another device or browser. Please login again.")
					url.RawQuery = params.Encode()
					return ctx.Redirect(http.StatusTemporaryRedirect, url.String())
					*/

					return ctx.JSON(http.StatusConflict, map[string]interface{}{"message": ae.LOGIN_CONFICT.Error()})
				} else {
					return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
				}
			}

			err = utility.SetTokenClaimsToRouter(ctx, tokenClaims)
			if err != nil {
				return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
			}
		}

		// Check if the request is an upgrade to WebSocket
		if websocket.IsWebSocketUpgrade(ctx.Request()) {
			// Bypass the custom response writer for WebSocket connections
			// If you dont do this then the graphql websocket connection will fail
			return next(ctx)
		}

		// pass the request
		err = next(ctx)
		return err
	}
}

func tokenFromCookies(r *http.Request) (*models.JWTTokens, error) {
	if r.URL.RawQuery == "v=2" {
		tokenString := r.Header.Get("Authorization")
		tokenType := "Bearer"
		// Missing Token
		if tokenString == "" {
			return nil, errors.New("token is missing")
		}

		// Check for tempered token , check with signing method RSA
		if strings.HasPrefix(tokenString, tokenType) {
			tokenString = strings.TrimSpace(strings.TrimPrefix(tokenString, tokenType))
			if tokenString != "" {
				return &models.JWTTokens{AccessToken: tokenString}, nil
			} else {
				return nil, errors.New("invalid token Claims ! WTF ")
			}
		}
	}

	var tokens models.JWTTokens
	c, err := r.Cookie("accessToken")
	if err != nil {
		return nil, errors.New("no token")
	}
	tokens.AccessToken = c.Value

	c, err = r.Cookie("userToken")
	if err != nil {
		return nil, errors.New("no token")
	}
	tokens.IDToken = c.Value

	return &tokens, nil
}

func tokenFromBearer(r *http.Request) (*string, error) {
	token := r.Header.Get("Authorization")
	tokenType := "Bearer"
	// Missing Token
	if token == "" {
		return nil, errors.New("unauthorized Request. API Key is missing")
	}

	var tokenString string
	// Check for tempered token , check with signing method RSA
	if strings.HasPrefix(token, tokenType) {
		tokenString = strings.TrimSpace(strings.TrimPrefix(token, tokenType))
		if tokenString == "" {
			return nil, errors.New("invalid API Request")
		}
		return &tokenString, nil
	}
	return nil, errors.New("invalid token Type")
}
