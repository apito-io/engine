package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

func TestCookiePlaneIgnoresExplicitBearer(t *testing.T) {
	svc := &ApitoTokenService{apiKeyManager: &ProjectKeyManager{}}
	e := echo.New()
	e.POST("/secured/graphql", svc.ApitoTokenHandler(func(c echo.Context) error {
		t.Fatal("cookie plane must stop before the content handler")
		return nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/secured/graphql", strings.NewReader(`{"query":"{ noteList { id } }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Use-Cookies", "true")
	req.Header.Set("Authorization", "Bearer ak_role")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid Cookie header") {
		t.Fatalf("bearer was not ignored: %s", rec.Body.String())
	}
}

func TestExplicitBearerSkipsCookies(t *testing.T) {
	auth := &planeAuth{}
	svc := &ApitoTokenService{authService: auth}
	e := echo.New()
	e.POST("/system/graphql", svc.ApitoTokenHandler(func(c echo.Context) error {
		if c.Get("auth_plane") != "id_token_bearer" {
			t.Fatalf("auth plane %v", c.Get("auth_plane"))
		}
		return c.NoContent(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/system/graphql", strings.NewReader(`{"query":"{ projects { id } }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Use-Cookies", "false")
	req.Header.Set("Authorization", "Bearer console-session-jwt")
	req.AddCookie(&http.Cookie{Name: "userToken", Value: "cookie-user"})
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: "cookie-access"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if auth.seen != "console-session-jwt" {
		t.Fatalf("verified %q", auth.seen)
	}
}

func TestExplicitProjectKeyDoesNotUseCookies(t *testing.T) {
	svc := &ApitoTokenService{apiKeyManager: &ProjectKeyManager{}}
	e := echo.New()
	e.POST("/secured/graphql", svc.ApitoTokenHandler(func(c echo.Context) error {
		t.Fatal("invalid project key must not reach the handler")
		return nil
	}))

	req := httptest.NewRequest(http.MethodPost, "/secured/graphql", strings.NewReader(`{"query":"{ noteList { id } }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Use-Cookies", "false")
	req.Header.Set("Authorization", "Bearer ak_role")
	req.AddCookie(&http.Cookie{Name: "userToken", Value: "cookie-user"})
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: "cookie-access"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Cookie") {
		t.Fatalf("cookie plane handled an explicit key: %s", rec.Body.String())
	}
}

type planeAuth struct {
	seen string
}

func (p *planeAuth) Login(context.Context, *models.LoginRequest, *models.SystemUser, *models.ProjectWithRoles) (*models.JWTTokens, error) {
	return nil, nil
}
func (p *planeAuth) Signup(context.Context, *models.RegisterRequest) (*models.SystemUser, error) {
	return nil, nil
}
func (p *planeAuth) ConfirmSignup(context.Context, *models.RegisterRequest) error { return nil }
func (p *planeAuth) ForgetPasswordRequest(context.Context, *models.RegisterRequest) error {
	return nil
}
func (p *planeAuth) ConfirmForgetPassword(context.Context, *models.RegisterRequest) error {
	return nil
}
func (p *planeAuth) ChangePassword(context.Context, *models.SystemUser, string, string) (*models.SystemUser, error) {
	return nil, nil
}
func (p *planeAuth) Logout(context.Context, string) error { return nil }
func (p *planeAuth) VerifyIDToken(_ context.Context, token string) (*models.TokenClaims, error) {
	p.seen = token
	return &models.TokenClaims{UserID: "user-1", Role: "admin", ProjectID: "project-1"}, nil
}
func (p *planeAuth) VerifyAccessToken(context.Context, string) error { return nil }
func (p *planeAuth) ExchangeAndRefreshToken(context.Context, *models.ProjectWithRoles) (*models.JWTTokens, error) {
	return nil, nil
}
