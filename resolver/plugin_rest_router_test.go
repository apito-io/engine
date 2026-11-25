package resolver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

// newPluginRESTTestEcho mounts r the same way BuildGraphQL does.
func newPluginRESTTestEcho(r *pluginRESTRouter) *echo.Echo {
	e := echo.New()
	e.Group("plugin").Any("/*", r.dispatch)
	return e
}

func servePluginREST(e *echo.Echo, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func textHandler(body string) echo.HandlerFunc {
	return func(c echo.Context) error { return c.String(http.StatusOK, body) }
}

func TestPluginRESTRouterDispatchesByMethodAndPath(t *testing.T) {
	r := newPluginRESTRouter()
	e := newPluginRESTTestEcho(r)

	if ok, _ := r.register("hc-a", "GET", "/hc-a/items", textHandler("list")); !ok {
		t.Fatal("GET refused")
	}
	// Same path, different method: previously dropped by the path-only dedupe.
	if ok, _ := r.register("hc-a", "post", "/hc-a/items", textHandler("create")); !ok {
		t.Fatal("POST on same path refused")
	}

	if rec := servePluginREST(e, http.MethodGet, "/plugin/hc-a/items"); rec.Code != http.StatusOK || rec.Body.String() != "list" {
		t.Fatalf("GET: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec := servePluginREST(e, http.MethodPost, "/plugin/hc-a/items"); rec.Code != http.StatusOK || rec.Body.String() != "create" {
		t.Fatalf("POST: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec := servePluginREST(e, http.MethodGet, "/plugin/hc-a/missing"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing route: code=%d", rec.Code)
	}
	if rec := servePluginREST(e, http.MethodDelete, "/plugin/hc-a/items"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method: code=%d", rec.Code)
	}

	routes := r.routes()
	if len(routes) != 2 || routes[0].Method != "GET" || routes[1].Method != "POST" || routes[0].PluginID != "hc-a" {
		t.Fatalf("routes=%+v", routes)
	}
}

// A total miss leaves Echo's Find without touching the handler; dispatch must
// not fall back to the outer catch-all (which recursed until stack overflow).
func TestPluginRESTRouterMissDoesNotRecurse(t *testing.T) {
	empty := newPluginRESTTestEcho(newPluginRESTRouter())
	if rec := servePluginREST(empty, http.MethodGet, "/plugin/anything"); rec.Code != http.StatusNotFound {
		t.Fatalf("empty router: code=%d", rec.Code)
	}

	r := newPluginRESTRouter()
	r.register("hc-a", "GET", "/hc-a/items", textHandler("list"))
	e := newPluginRESTTestEcho(r)
	if rec := servePluginREST(e, http.MethodGet, "/plugin/zzz/unrelated"); rec.Code != http.StatusNotFound {
		t.Fatalf("unrelated path: code=%d", rec.Code)
	}
}

func TestPluginRESTRouterOwnership(t *testing.T) {
	r := newPluginRESTRouter()
	e := newPluginRESTTestEcho(r)

	r.register("hc-a", "GET", "/shared", textHandler("v1"))

	// Hot reload by the owner swaps in the fresh handler.
	if ok, _ := r.register("hc-a", "GET", "/shared", textHandler("v2")); !ok {
		t.Fatal("owner re-register refused")
	}
	if rec := servePluginREST(e, http.MethodGet, "/plugin/shared"); rec.Body.String() != "v2" {
		t.Fatalf("after reload body=%q", rec.Body.String())
	}

	// Another plugin cannot take the route over.
	ok, owner := r.register("hc-b", "GET", "/shared", textHandler("hijack"))
	if ok || owner != "hc-a" {
		t.Fatalf("foreign register ok=%v owner=%q", ok, owner)
	}
	if rec := servePluginREST(e, http.MethodGet, "/plugin/shared"); rec.Body.String() != "v2" {
		t.Fatalf("after refused takeover body=%q", rec.Body.String())
	}
}

func TestPluginRESTRouterParamsAfterPooledContexts(t *testing.T) {
	r := newPluginRESTRouter()
	e := newPluginRESTTestEcho(r)

	// Warm the context pool while only the one-param catch-all exists.
	r.register("hc-a", "GET", "/hc-a/ping", textHandler("pong"))
	for i := 0; i < 4; i++ {
		servePluginREST(e, http.MethodGet, "/plugin/hc-a/ping")
	}

	// A route with more params than any pooled context was sized for.
	r.register("hc-a", "GET", "/hc-a/:org/:repo/:id", func(c echo.Context) error {
		return c.String(http.StatusOK, c.Param("org")+"|"+c.Param("repo")+"|"+c.Param("id"))
	})
	rec := servePluginREST(e, http.MethodGet, "/plugin/hc-a/acme/engine/42")
	if rec.Code != http.StatusOK || rec.Body.String() != "acme|engine|42" {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}

// Run with -race: reproduces the boot crash shape (parallel plugin loads
// registering while routes are listed) plus runtime reload while serving.
func TestPluginRESTRouterConcurrentRegisterAndDispatch(t *testing.T) {
	r := newPluginRESTRouter()
	e := newPluginRESTTestEcho(r)
	r.register("hc-base", "GET", "/hc-base/ping", textHandler("pong"))

	const plugins, routesPerPlugin, readers = 8, 20, 8
	var wg sync.WaitGroup
	for p := 0; p < plugins; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			id := fmt.Sprintf("hc-p%d", p)
			for i := 0; i < routesPerPlugin; i++ {
				r.register(id, "GET", fmt.Sprintf("/%s/r%d/:id", id, i), textHandler(id))
				r.routes()
			}
		}(p)
	}
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if rec := servePluginREST(e, http.MethodGet, "/plugin/hc-base/ping"); rec.Body.String() != "pong" {
					t.Errorf("dispatch during registration: code=%d body=%q", rec.Code, rec.Body.String())
					return
				}
			}
		}()
	}
	wg.Wait()

	if got := len(r.routes()); got != plugins*routesPerPlugin+1 {
		t.Fatalf("routes=%d", got)
	}
	if rec := servePluginREST(e, http.MethodGet, "/plugin/hc-p3/r7/x"); rec.Body.String() != "hc-p3" {
		t.Fatalf("late route body=%q", rec.Body.String())
	}
}

func testField(desc string) *graphql.Field {
	return &graphql.Field{Type: graphql.String, Description: desc}
}

func TestPluginSystemFieldsFirstOwnerWins(t *testing.T) {
	s := &GraphQLServer{SystemQueries: graphql.Fields{}, SystemMutations: graphql.Fields{}}
	s.overwriteSystemFields(graphql.Fields{"core": testField("core")}, nil)

	s.mergePluginSystemFields("hc-a", graphql.Fields{"ext_sign": testField("a1")}, graphql.Fields{"ext_del": testField("a1")})
	before, _ := s.SystemFieldsSnapshot()

	// Another plugin, or a plugin colliding with a core field, is ignored.
	s.mergePluginSystemFields("hc-b", graphql.Fields{"ext_sign": testField("b"), "core": testField("b")}, nil)
	// The owner's reload replaces its own definition.
	s.mergePluginSystemFields("hc-a", graphql.Fields{"ext_sign": testField("a2")}, nil)

	q, m := s.SystemFieldsSnapshot()
	if q["ext_sign"].Description != "a2" || q["core"].Description != "core" || m["ext_del"].Description != "a1" {
		t.Fatalf("queries=%v mutations=%v", q, m)
	}
	// Published maps are never mutated in place.
	if before["ext_sign"].Description != "a1" {
		t.Fatal("earlier snapshot was mutated")
	}
}

func TestPluginSystemFieldsConcurrentMergeAndSnapshot(t *testing.T) {
	s := &GraphQLServer{SystemQueries: graphql.Fields{}, SystemMutations: graphql.Fields{}}

	const writers = 8
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			id := fmt.Sprintf("hc-p%d", w)
			for i := 0; i < 25; i++ {
				key := fmt.Sprintf("ext_%s_%d", strings.TrimPrefix(id, "hc-"), i)
				s.mergePluginSystemFields(id, graphql.Fields{key: testField(id)}, graphql.Fields{key: testField(id)})
			}
		}(w)
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 25; i++ {
			s.overwriteSystemFields(graphql.Fields{fmt.Sprintf("core_%d", i): testField("core")}, nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			q, m := s.SystemFieldsSnapshot()
			for range q {
			}
			for range m {
			}
		}
	}()
	wg.Wait()

	q, m := s.SystemFieldsSnapshot()
	if len(q) != writers*25+25 || len(m) != writers*25 {
		t.Fatalf("queries=%d mutations=%d", len(q), len(m))
	}
}

func TestInstalledHCPluginListConcurrent(t *testing.T) {
	s := &GraphQLServer{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("hc-p%d", i)
			s.addInstalledHCPlugin(id)
			s.addInstalledHCPlugin(id)
			_ = s.installedHCPluginIDs()
			if i%2 == 0 {
				s.RemoveInstalledHCPlugin(id)
			}
		}(i)
	}
	wg.Wait()
	if got := s.installedHCPluginIDs(); len(got) != 4 {
		t.Fatalf("installed=%v", got)
	}
}
