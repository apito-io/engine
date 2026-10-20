package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/apito-io/engine/mcp"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

// MountRemoteMCP registers the URL-only MCP OAuth and Streamable HTTP routes
// on the existing engine router.
func (a *AuthController) MountRemoteMCP(e *echo.Echo, authorize echo.MiddlewareFunc) error {
	_, err := mcp.Mount(e, mcp.Options{
		Config:    a.Cfg,
		Authorize: authorize,
		Mint: func(ctx context.Context, userID string, req *models.CreateAccessTokenRequest) (string, error) {
			svc := a.accessTokenSvc()
			if svc == nil {
				return "", echo.NewHTTPError(http.StatusInternalServerError, "access token service unavailable")
			}
			raw, _, err := svc.Mint(ctx, userID, req)
			return raw, err
		},
		System: echoSystem{echo: e},
	})
	return err
}

type echoSystem struct {
	echo *echo.Echo
}

func (s echoSystem) GraphQL(ctx context.Context, token, projectID, tenantID, query string, vars map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return nil, err
	}
	status, raw, err := s.do(ctx, token, http.MethodPost, "/system/graphql", projectID, tenantID, body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return raw, echo.NewHTTPError(status, string(raw))
	}
	return raw, nil
}

func (s echoSystem) REST(ctx context.Context, token, method, path string, body []byte) (int, json.RawMessage, error) {
	return s.do(ctx, token, method, path, "", "", body)
}

func (s echoSystem) do(ctx context.Context, token, method, path, projectID, tenantID string, body []byte) (int, json.RawMessage, error) {
	meta, hasMeta := mcp.RESTMetaFrom(ctx)
	if projectID == "" && hasMeta {
		projectID = meta.ProjectID
	}
	if tenantID == "" && hasMeta {
		tenantID = meta.TenantID
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	// Headless apt_ calls must not fall through to the console cookie path.
	req.Header.Set("X-Use-Cookies", "false")
	if hasMeta {
		for key, value := range meta.Headers {
			req.Header.Set(key, value)
		}
	}
	if projectID != "" {
		req.Header.Set(models.ApitoProjectIDHeader, projectID)
	}
	if tenantID != "" {
		req.Header.Set(models.ApitoTenantIDHeader, tenantID)
	}
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	raw, err := io.ReadAll(rec.Body)
	return rec.Code, raw, err
}
