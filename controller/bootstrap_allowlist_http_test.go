package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestSystemGraphQLCustomClientCannotUseBootstrapKeyForPrivilegedFields(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("auth_plane", "project_api_key")
			c.Set("role", "passenger")
			return next(c)
		}
	})
	e.POST("/system/graphql", (&GraphCtrl{}).SystemGraphQL)

	server := httptest.NewServer(e)
	defer server.Close()

	for _, query := range []string{
		`mutation { createTenant(name: "attacker") { id } }`,
		`query { alias: loginUser(phone: "1", secret: "x") { token } }`,
		`query { __schema { types { name } } }`,
	} {
		encodedQuery, err := json.Marshal(query)
		require.NoError(t, err)
		body := bytes.NewBufferString(`{"query":` + string(encodedQuery) + `}`)
		req, err := http.NewRequest(http.MethodPost, server.URL+"/system/graphql", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		require.NoError(t, readErr)
		require.NoError(t, closeErr)
		require.Equal(t, http.StatusForbidden, resp.StatusCode, string(responseBody))
		require.Contains(t, string(responseBody), "SYSTEM_OPERATION_FORBIDDEN_FOR_CLIENT_KEY")
	}
}
