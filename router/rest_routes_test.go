package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestSecuredRESTRoutesCreateAndUpdateByID(t *testing.T) {
	e := echo.New()
	RegisterSecuredRESTRoutes(e.Group("/secured"), func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"method": c.Request().Method,
			"model":  c.Param("model"),
			"id":     c.Param("id"),
		})
	})

	createRec := requestREST(t, e, http.MethodPost, "/secured/rest/proj/notes", `{"title":"a"}`)
	require.Equal(t, http.StatusOK, createRec.Code)
	createBody := decodeRouteBody(t, createRec)
	require.Equal(t, http.MethodPost, createBody["method"])
	require.Equal(t, "notes", createBody["model"])
	require.Empty(t, createBody["id"])

	updateRec := requestREST(t, e, http.MethodPut, "/secured/rest/proj/food_order/doc-1", `{"total":2}`)
	require.Equal(t, http.StatusOK, updateRec.Code)
	updateBody := decodeRouteBody(t, updateRec)
	require.Equal(t, http.MethodPut, updateBody["method"])
	require.Equal(t, "food_order", updateBody["model"])
	require.Equal(t, "doc-1", updateBody["id"])
}

func requestREST(t *testing.T, e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeRouteBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}
