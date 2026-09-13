package resolver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
)

func TestBuildSystemParamPreservesProjectUserFlag(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/secured/graphql", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	c.Set("role", "passenger")
	c.Set("user", "user_1")
	c.Set("project", "proj_1")
	c.Set("is_project_user", true)

	s := &GraphQLServer{Cfg: &models.Config{}}
	project := &models.Project{
		Roles: map[string]*models.Role{
			"passenger": {
				APIPermissions: map[string]*models.APIPermission{
					"user_profile": {Read: "own", Create: "auth", Update: "own", Delete: "own"},
				},
			},
		},
	}

	param, err := s.BuildSystemParam(c, project)
	if err != nil {
		t.Fatal(err)
	}
	if !param.Role.IsProjectUser {
		t.Fatal("token IsProjectUser must survive role template merge")
	}
	if project.Roles["passenger"].IsProjectUser {
		t.Fatal("must not mutate cached role template")
	}
	if err := utility.AuthorizeModelRead(param.Role, "user_profile"); err != nil {
		t.Fatalf("passenger project user should read own user_profile: %v", err)
	}
}
