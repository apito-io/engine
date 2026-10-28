package utility

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRESTtoGraphQLResolvesNamesAndWriteVerbs(t *testing.T) {
	schema := &models.ProjectSchema{Models: []*models.ModelType{
		{Name: "note", Fields: []*models.FieldInfo{{Identifier: "title", FieldType: "text"}}},
		{Name: "food_order", Fields: []*models.FieldInfo{{Identifier: "total", FieldType: "number"}}},
	}}

	create := restQuery(t, http.MethodPost, "/notes", `{"title":"hello"}`, schema, "notes", nil)
	require.Equal(t, http.StatusCreated, create.HTTPStatus)
	require.Contains(t, create.Query, "createNote")
	require.Contains(t, create.Query, `title : "hello"`)
	require.NotContains(t, create.Query, "updateNote")

	snake := restQuery(t, http.MethodPost, "/foodOrder", `{"total":4}`, schema, "foodOrder", nil)
	require.Contains(t, snake.Query, "createFoodOrder")
	require.Contains(t, snake.Query, "total : 4")

	snakePlural := restQuery(t, http.MethodPost, "/food_orders", `{"total":5}`, schema, "food_orders", nil)
	require.Contains(t, snakePlural.Query, "createFoodOrder")

	list := restQuery(t, http.MethodGet, "/noteList", "", schema, "noteList", nil)
	require.Equal(t, "noteList", list.QueryName)
	require.Contains(t, list.Query, "query")

	update := restQuery(t, http.MethodPut, "/notes/doc-1", `{"title":"edited"}`, schema, "notes", map[string]string{"_id": "doc-1"})
	require.Equal(t, http.StatusOK, update.HTTPStatus)
	require.Contains(t, update.Query, "updateNote")
	require.Contains(t, update.Query, `_id: "doc-1"`)
	require.NotContains(t, update.Query, "createNote")

	compat := restQuery(t, http.MethodPut, "/note", `{"title":"compat"}`, schema, "note", map[string]string{"_id": "doc-2"})
	require.Contains(t, compat.Query, "updateNote")
	require.Contains(t, compat.Query, `_id: "doc-2"`)
}

func TestRESTtoGraphQLRejectsEmptyAndUnknownWrites(t *testing.T) {
	schema := &models.ProjectSchema{Models: []*models.ModelType{
		{Name: "note", Fields: []*models.FieldInfo{{Identifier: "title", FieldType: "text"}}},
	}}

	_, err := RESTtoGraphQL(restContext(http.MethodPost, "/notes", `{}`, nil), schema, "notes", nil, false)
	require.EqualError(t, err, "Request body is required")

	_, err = RESTtoGraphQL(restContext(http.MethodPost, "/notes", `{"missing":"x"}`, nil), schema, "notes", nil, false)
	require.EqualError(t, err, "request body has no known fields")

	_, err = RESTtoGraphQL(restContext(http.MethodPut, "/notes", `{"title":"x"}`, nil), schema, "notes", nil, false)
	require.EqualError(t, err, "id is required for an update")

	_, err = RESTtoGraphQL(restContext(http.MethodPost, "/missing", `{"title":"x"}`, nil), schema, "missing", nil, false)
	require.EqualError(t, err, "model is required")
}

func restQuery(t *testing.T, method, path, body string, schema *models.ProjectSchema, model string, query map[string]string) *RESTtoGraphResponse {
	t.Helper()
	built, err := RESTtoGraphQL(restContext(method, path, body, query), schema, model, valuesOf(query), false)
	require.NoError(t, err)
	return built
}

func restContext(method, path, body string, query map[string]string) echo.Context {
	target := path
	if len(query) > 0 {
		target += "?"
		first := true
		for key, value := range query {
			if !first {
				target += "&"
			}
			first = false
			target += key + "=" + value
		}
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return echo.New().NewContext(req, httptest.NewRecorder())
}

func valuesOf(query map[string]string) map[string][]string {
	if len(query) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(query))
	for key, value := range query {
		out[key] = []string{value}
	}
	return out
}
