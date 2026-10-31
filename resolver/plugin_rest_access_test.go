package resolver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/types/protobuff"
	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestPluginRESTAccessDefaultsProtected(t *testing.T) {
	route := &protobuff.ThirdPartyRESTApi{Method: "POST", Path: "/sign"}
	if got := pluginRESTAccess("hc-cloudinary-plugin", route); got != pluginRESTAccessProtected {
		t.Fatalf("access=%s", got)
	}
	if got := pluginRESTAccess("hc-cloudinary-plugin", route); got != pluginRESTAccessProtected {
		t.Fatalf("query project_id made route %s", got)
	}
}

func TestPluginRESTAccessExplicitPublic(t *testing.T) {
	schema, err := structpb.NewStruct(map[string]interface{}{
		"_apito": map[string]interface{}{"version": float64(1), "access": "public"},
	})
	if err != nil {
		t.Fatal(err)
	}
	route := &protobuff.ThirdPartyRESTApi{Method: "GET", Path: "/callback", Schema: schema}
	if got := pluginRESTAccess("hc-eps-payment-plugin", route); got != pluginRESTAccessPublic {
		t.Fatalf("access=%s", got)
	}
}

func TestDeclaredRESTAccessDoesNotFlipFromQuery(t *testing.T) {
	id := "hc-cloudinary-plugin"
	pluginService.SetPluginContributions(id, &pluginService.Contributions{
		API: &pluginService.APIContribution{
			REST: []pluginService.RESTOperation{{Method: "POST", Path: "/sign", Access: "protected"}},
		},
	})
	t.Cleanup(func() { pluginService.SetPluginContributions(id, nil) })
	route := &protobuff.ThirdPartyRESTApi{Method: "POST", Path: "/sign"}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/sign?project_id=other", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if got := pluginRESTAccess(id, route); got != pluginRESTAccessProtected {
		t.Fatalf("access=%s", got)
	}
	if err := enforcePluginRESTProjectMatch("proj-a", requestedPluginProjectID(c)); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestEnforcePluginRESTProjectMatch(t *testing.T) {
	if err := enforcePluginRESTProjectMatch("", "p1"); err != errPluginRESTUnauthenticated {
		t.Fatalf("empty session: %v", err)
	}
	if err := enforcePluginRESTProjectMatch("p1", "p2"); err != errPluginRESTProjectMismatch {
		t.Fatalf("mismatch: %v", err)
	}
	if err := enforcePluginRESTProjectMatch("p1", "p1"); err != nil {
		t.Fatal(err)
	}
	if err := enforcePluginRESTProjectMatch("p1", ""); err != nil {
		t.Fatal(err)
	}
}

func TestGuardProtectedPluginRESTRequiresAuthentication(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/plugin/hc-cloudinary-plugin/assets", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	s := &GraphQLServer{}
	if err := s.guardProtectedPluginREST(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAllowPluginPublicREST(t *testing.T) {
	resetPluginPublicRESTLimits()
	now := time.Now()
	if !allowPluginPublicREST("k", now, 2) || !allowPluginPublicREST("k", now, 2) {
		t.Fatal("first calls should pass")
	}
	if allowPluginPublicREST("k", now, 2) {
		t.Fatal("third call should be limited")
	}
}

func TestSanitizePublicPluginResult(t *testing.T) {
	out := sanitizePublicPluginResult(map[string]interface{}{
		"status":     "ok",
		"api_secret": "nope",
		"hash_key":   "nope",
		"nested":     map[string]interface{}{"password": "x", "id": "1"},
	}).(map[string]interface{})
	if _, ok := out["api_secret"]; ok {
		t.Fatal("secret leaked")
	}
	if out["status"] != "ok" {
		t.Fatalf("status=%v", out["status"])
	}
	nested := out["nested"].(map[string]interface{})
	if _, ok := nested["password"]; ok {
		t.Fatal("nested secret leaked")
	}
	if nested["id"] != "1" {
		t.Fatal("id dropped")
	}
}
