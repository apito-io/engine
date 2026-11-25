package resolver

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
)

// pluginRESTRouter owns every plugin REST route behind a single catch-all
// route on the extension group.
//
// Echo's Router is not safe for concurrent use: Add mutates the radix tree and
// route map with no locking, while Find and Routes read them. Plugins register
// routes from parallel load goroutines at boot and again on hot reload or
// monitor restart while requests are being served, so the main Echo router must
// never be mutated after startup. Plugin routes instead live in this private
// router, guarded by an RWMutex: registrations take the write lock, request
// dispatch and route listing take the read lock.
type pluginRESTRouter struct {
	mu        sync.RWMutex
	router    *echo.Router
	owners    map[string]string // "METHOD path" -> owning plugin ID
	maxParams int
}

func newPluginRESTRouter() *pluginRESTRouter {
	// A private Echo instance keeps the router's maxParam bookkeeping away from
	// the main instance, whose context pool reads it concurrently.
	return &pluginRESTRouter{
		router: echo.NewRouter(echo.New()),
		owners: make(map[string]string),
	}
}

func pluginRouteKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// register adds or replaces a plugin route. A route is keyed by method and
// path, so one plugin may expose several methods on the same path. The plugin
// that registered a route first owns it: re-registering by the same plugin
// (hot reload / restart) swaps in the fresh handler, while another plugin's
// attempt is refused and the current owner is returned.
func (r *pluginRESTRouter) register(pluginID, method, path string, h echo.HandlerFunc) (bool, string) {
	method = strings.ToUpper(method)
	key := pluginRouteKey(method, path)

	r.mu.Lock()
	defer r.mu.Unlock()

	if owner, exists := r.owners[key]; exists && owner != pluginID {
		return false, owner
	}
	r.router.Add(method, path, h)
	r.owners[key] = pluginID
	if n := routeParamCount(path); n > r.maxParams {
		r.maxParams = n
	}
	return true, pluginID
}

// dispatch is the catch-all handler mounted on the extension group. It resolves
// the remainder of the request path against the plugin routes and runs the
// matched handler outside the lock.
func (r *pluginRESTRouter) dispatch(c echo.Context) error {
	path := "/" + c.Param("*")
	method := c.Request().Method

	r.mu.RLock()
	// Find writes path params into the context's pooled value slice, which is
	// only sized for the main router's params. Grow it to fit plugin routes.
	if r.maxParams > 0 {
		c.SetParamNames(make([]string, r.maxParams)...)
	}
	// Find leaves the handler untouched when nothing matches at all; clear it
	// so a miss cannot re-enter this catch-all via the outer handler.
	c.SetHandler(nil)
	r.router.Find(method, path, c)
	handler := c.Handler()
	r.mu.RUnlock()

	if handler == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return handler(c)
}

// pluginRoute describes one registered plugin REST route.
type pluginRoute struct {
	Method   string
	Path     string
	PluginID string
}

// routes returns a sorted snapshot of the registered plugin routes.
func (r *pluginRESTRouter) routes() []pluginRoute {
	r.mu.RLock()
	routes := make([]pluginRoute, 0, len(r.owners))
	for key, owner := range r.owners {
		method, path, _ := strings.Cut(key, " ")
		routes = append(routes, pluginRoute{Method: method, Path: path, PluginID: owner})
	}
	r.mu.RUnlock()

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes
}

// routeParamCount is an upper bound on the path params Echo extracts for path.
func routeParamCount(path string) int {
	return strings.Count(path, ":") + strings.Count(path, "*")
}
