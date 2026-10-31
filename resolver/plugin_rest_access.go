package resolver

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/apito-io/engine/services"
	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/types/protobuff"
	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	pluginRESTAccessProtected = "protected"
	pluginRESTAccessPublic    = "public"
	pluginRESTAccessVersion   = 1
	pluginPublicBodyLimit     = 64 << 10
	pluginPublicRatePerMinute = 60
)

var (
	errPluginRESTUnauthenticated = errors.New("authentication required")
	errPluginRESTProjectMismatch = errors.New("project access denied")
	secretPayloadKey             = regexp.MustCompile(`(?i)secret|password|hash_key|hashkey|api_secret|api_key|jwt|authorization|refresh_token`)
)

type pluginRESTWindow struct {
	start time.Time
	count int
}

var pluginPublicRESTHits = struct {
	mu sync.Mutex
	m  map[string]*pluginRESTWindow
}{m: map[string]*pluginRESTWindow{}}

func resetPluginPublicRESTLimits() {
	pluginPublicRESTHits.mu.Lock()
	pluginPublicRESTHits.m = map[string]*pluginRESTWindow{}
	pluginPublicRESTHits.mu.Unlock()
}

func allowPluginPublicREST(key string, now time.Time, limit int) bool {
	if limit <= 0 {
		limit = pluginPublicRatePerMinute
	}
	pluginPublicRESTHits.mu.Lock()
	defer pluginPublicRESTHits.mu.Unlock()
	slot := pluginPublicRESTHits.m[key]
	if slot == nil || now.Sub(slot.start) >= time.Minute {
		pluginPublicRESTHits.m[key] = &pluginRESTWindow{start: now, count: 1}
		return true
	}
	if slot.count >= limit {
		return false
	}
	slot.count++
	return true
}

func explicitPluginRESTAccess(route *protobuff.ThirdPartyRESTApi) (string, bool) {
	if route == nil || route.Schema == nil {
		return "", false
	}
	raw, _ := route.Schema.AsMap()["_apito"].(map[string]interface{})
	if raw == nil {
		return "", false
	}
	access, _ := raw["access"].(string)
	switch strings.ToLower(strings.TrimSpace(access)) {
	case pluginRESTAccessPublic:
		return pluginRESTAccessPublic, true
	case pluginRESTAccessProtected:
		return pluginRESTAccessProtected, true
	default:
		return "", false
	}
}

func pluginRESTAccess(pluginID string, route *protobuff.ThirdPartyRESTApi) string {
	if access, ok := explicitPluginRESTAccess(route); ok {
		return access
	}
	if route != nil {
		if c := pluginService.ContributionsFor(pluginID); c != nil && c.API != nil {
			for _, op := range c.API.REST {
				if strings.EqualFold(op.Method, route.Method) && op.Path == route.Path {
					switch op.Access {
					case pluginRESTAccessPublic:
						return pluginRESTAccessPublic
					case pluginRESTAccessProtected:
						return pluginRESTAccessProtected
					}
				}
			}
		}
	}
	return pluginRESTAccessProtected
}

func stampPluginRESTAccess(pluginID string, route *protobuff.ThirdPartyRESTApi) {
	if route == nil {
		return
	}
	if _, ok := explicitPluginRESTAccess(route); ok {
		return
	}
	access := pluginRESTAccess(pluginID, route)
	schema := map[string]interface{}{}
	if route.Schema != nil {
		schema = route.Schema.AsMap()
	}
	schema["_apito"] = map[string]interface{}{
		"version": float64(pluginRESTAccessVersion),
		"access":  access,
	}
	st, err := structpb.NewStruct(schema)
	if err != nil {
		return
	}
	route.Schema = st
}

func requestedPluginProjectID(c echo.Context) string {
	if c == nil || c.Request() == nil {
		return ""
	}
	if h := strings.TrimSpace(c.Request().Header.Get("X-Apito-Project-ID")); h != "" {
		return h
	}
	return strings.TrimSpace(c.QueryParam("project_id"))
}

func enforcePluginRESTProjectMatch(session, requested string) error {
	session = strings.TrimSpace(session)
	requested = strings.TrimSpace(requested)
	if session == "" {
		return errPluginRESTUnauthenticated
	}
	if requested != "" && requested != session {
		return errPluginRESTProjectMismatch
	}
	return nil
}

func (s *GraphQLServer) guardPluginREST(c echo.Context, pluginID string, route *protobuff.ThirdPartyRESTApi) error {
	if pluginRESTAccess(pluginID, route) == pluginRESTAccessPublic {
		return s.guardPublicPluginREST(c, pluginID, route)
	}
	return s.guardProtectedPluginREST(c)
}

func (s *GraphQLServer) guardPublicPluginREST(c echo.Context, pluginID string, route *protobuff.ThirdPartyRESTApi) error {
	if c.Request() != nil && c.Request().ContentLength > pluginPublicBodyLimit {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]interface{}{
			"error": "payload too large",
			"code":  http.StatusRequestEntityTooLarge,
		})
	}
	path := ""
	if route != nil {
		path = route.Path
	}
	key := c.RealIP() + "|" + pluginID + "|" + path
	if !allowPluginPublicREST(key, time.Now(), pluginPublicRatePerMinute) {
		return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
			"error": "rate limit",
			"code":  http.StatusTooManyRequests,
		})
	}
	return nil
}

func (s *GraphQLServer) guardProtectedPluginREST(c echo.Context) error {
	if s == nil || s.ApitoTokenService == nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error": errPluginRESTUnauthenticated.Error(),
			"code":  http.StatusUnauthorized,
		})
	}
	err := s.ApitoTokenService.ApitoTokenHandler(func(c echo.Context) error {
		session, _ := c.Get("project").(string)
		if matchErr := enforcePluginRESTProjectMatch(session, requestedPluginProjectID(c)); matchErr != nil {
			code := http.StatusUnauthorized
			if errors.Is(matchErr, errPluginRESTProjectMismatch) {
				code = http.StatusForbidden
			}
			return c.JSON(code, map[string]interface{}{"error": matchErr.Error(), "code": code})
		}
		if grantErr := enforcePluginRESTProjectGrant(
			c,
			s.ApitoTokenService.AccessTokens(),
			strings.TrimSpace(session),
		); grantErr != nil {
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"error": errPluginRESTProjectMismatch.Error(),
				"code":  http.StatusForbidden,
			})
		}
		return nil
	})(c)
	if c.Response().Committed {
		if err != nil {
			return err
		}
		if c.Response().Status >= 400 {
			return errPluginRESTUnauthenticated
		}
	}
	return err
}

// enforcePluginRESTProjectGrant is intentionally strict. The general access
// policy can run in report-only mode for migration diagnostics; plugin REST
// must never let an under-scoped apt_ principal cross a project boundary.
func enforcePluginRESTProjectGrant(
	c echo.Context,
	accessTokens *services.AccessTokenService,
	projectID string,
) error {
	principal := services.PrincipalFromEcho(c)
	if principal == nil || accessTokens == nil {
		return nil
	}
	return accessTokens.AuthorizeProject(c.Request().Context(), principal, projectID)
}

func sanitizePublicPluginResult(result interface{}) interface{} {
	switch v := result.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, val := range v {
			if secretPayloadKey.MatchString(k) {
				continue
			}
			out[k] = sanitizePublicPluginResult(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i := range v {
			out[i] = sanitizePublicPluginResult(v[i])
		}
		return out
	default:
		return result
	}
}
