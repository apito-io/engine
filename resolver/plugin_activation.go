package resolver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/apito-io/engine/models"
	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types/protobuff"
	"github.com/labstack/echo/v4"
)

type pluginUpsertInput struct {
	ID             string
	Enable         *bool
	ActivateStatus *protobuff.PluginActivateStatus
	EnvVars        []*protobuff.EnvVariable
}

// applyPluginUpsert mutates project.Plugins: honor enable, set activate_status
// without inversion, and append when the plugin is new.
func applyPluginUpsert(project *models.Project, catalog *models.SavedPluginDetails, in pluginUpsertInput) (*models.SavedPluginDetails, error) {
	if project == nil {
		return nil, errPluginProjectRequired
	}
	if in.ID == "" {
		return nil, errPluginIDRequired
	}

	details := findProjectPlugin(project.Plugins, in.ID)
	if details == nil {
		if catalog == nil {
			return nil, errPluginNotInRegistry
		}
		cloned := *catalog
		cloned.ProjectID = project.ID
		cloned.ID = in.ID
		cloned.EnvVars = cloneEnvVars(catalog.EnvVars)
		details = &cloned
		project.Plugins = append(project.Plugins, details)
	}

	if len(in.EnvVars) > 0 {
		applyEnvVars(details, in.EnvVars)
	}

	if in.Enable != nil {
		details.Enable = *in.Enable
		if *in.Enable {
			details.ActivateStatus = protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED
		} else {
			details.ActivateStatus = protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_DEACTIVATED
		}
	}

	if in.ActivateStatus != nil {
		details.ActivateStatus = *in.ActivateStatus
		details.Enable = *in.ActivateStatus == protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED
	}

	return details, nil
}

func removeProjectPlugin(plugins []*models.SavedPluginDetails, id string) []*models.SavedPluginDetails {
	if id == "" || len(plugins) == 0 {
		return plugins
	}
	out := plugins[:0]
	for _, p := range plugins {
		if p == nil || p.ID == id {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isProjectPluginActivated(project *models.Project, pluginID string) bool {
	if project == nil || pluginID == "" {
		return false
	}
	for _, p := range project.Plugins {
		if p != nil && p.ID == pluginID {
			return isSavedPluginActivated(p)
		}
	}
	return false
}

func isSavedPluginActivated(p *models.SavedPluginDetails) bool {
	if p == nil || !strings.HasPrefix(p.ID, "hc-") {
		return false
	}
	if p.ActivateStatus == protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_DEACTIVATED && !p.Enable {
		return false
	}
	return p.Enable || p.ActivateStatus == protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED
}

func pluginRequiresProjectActivation(pluginID string) bool {
	return pluginService.HasCapability(pluginID, pluginService.CapProjectGraphQL) ||
		pluginService.HasCapability(pluginID, pluginService.CapProjectREST) ||
		pluginService.HasCapability(pluginID, pluginService.CapConsoleRoutes) ||
		pluginService.HasCapability(pluginID, pluginService.CapConsoleSettings) ||
		pluginService.HasCapability(pluginID, pluginService.CapContentFields)
}

func findProjectPlugin(plugins []*models.SavedPluginDetails, id string) *models.SavedPluginDetails {
	for _, p := range plugins {
		if p != nil && p.ID == id {
			return p
		}
	}
	return nil
}

func cloneEnvVars(in []*protobuff.EnvVariable) []*protobuff.EnvVariable {
	if len(in) == 0 {
		return nil
	}
	out := make([]*protobuff.EnvVariable, 0, len(in))
	for _, e := range in {
		if e == nil || e.Key == "" {
			continue
		}
		out = append(out, &protobuff.EnvVariable{Key: e.Key, Value: e.Value})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mergeYAMLAndProjectEnvVars keeps config.yml keys as the template and overlays
// per-project values. Extra project keys that yaml does not declare stay last.
func mergeYAMLAndProjectEnvVars(yamlVars, projectVars []*protobuff.EnvVariable) []*protobuff.EnvVariable {
	byKey := make(map[string]string, len(projectVars))
	order := make([]string, 0, len(yamlVars)+len(projectVars))
	seen := map[string]struct{}{}
	for _, e := range yamlVars {
		if e == nil || e.Key == "" {
			continue
		}
		if _, ok := seen[e.Key]; ok {
			continue
		}
		seen[e.Key] = struct{}{}
		order = append(order, e.Key)
		byKey[e.Key] = e.Value
	}
	for _, e := range projectVars {
		if e == nil || e.Key == "" {
			continue
		}
		byKey[e.Key] = e.Value
		if _, ok := seen[e.Key]; ok {
			continue
		}
		seen[e.Key] = struct{}{}
		order = append(order, e.Key)
	}
	if len(order) == 0 {
		return nil
	}
	out := make([]*protobuff.EnvVariable, 0, len(order))
	for _, k := range order {
		out = append(out, &protobuff.EnvVariable{Key: k, Value: byKey[k]})
	}
	return out
}

func applyEnvVars(details *models.SavedPluginDetails, incoming []*protobuff.EnvVariable) {
	if details.EnvVars == nil {
		details.EnvVars = cloneEnvVars(incoming)
		return
	}
	for _, next := range incoming {
		if next == nil || next.Key == "" {
			continue
		}
		found := false
		for _, existing := range details.EnvVars {
			if existing != nil && existing.Key == next.Key {
				existing.Value = next.Value
				found = true
				break
			}
		}
		if !found {
			details.EnvVars = append(details.EnvVars, &protobuff.EnvVariable{Key: next.Key, Value: next.Value})
		}
	}
}

func pluginIDFromSource(source interface{}) string {
	switch v := source.(type) {
	case *protobuff.PluginDetails:
		if v != nil {
			return v.Id
		}
	case protobuff.PluginDetails:
		return v.Id
	case map[string]interface{}:
		if id, ok := v["id"].(string); ok {
			return id
		}
	}
	return ""
}

func pluginRESTRequiresActivation(pluginID string) bool {
	systemOnlyREST := pluginService.HasCapability(pluginID, pluginService.CapSystemREST) &&
		!pluginService.HasCapability(pluginID, pluginService.CapProjectREST)
	if systemOnlyREST && !pluginRequiresProjectActivation(pluginID) {
		return false
	}
	return true
}

func (s *GraphQLServer) enforcePluginRESTActivation(c echo.Context, pluginID string) error {
	if !pluginRESTRequiresActivation(pluginID) {
		return nil
	}

	projectID := pluginProjectIDFromEcho(c)
	if projectID == "" {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error": "project id required to call this plugin",
			"code":  403,
		})
	}
	project, err := s.LoadProjectCache(c.Request().Context(), projectID)
	if err != nil || project == nil {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error": "plugin is not activated for this project",
			"code":  403,
		})
	}
	if !isProjectPluginActivated(project, pluginID) {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error": "plugin is not activated for this project",
			"code":  403,
		})
	}
	return nil
}

func (s *GraphQLServer) enforcePluginGraphQLActivation(ctx context.Context, pluginID string) error {
	if !pluginRequiresProjectActivation(pluginID) {
		return nil
	}
	projectID := pluginProjectIDFromContext(ctx)
	if projectID == "" {
		return errPluginNotActivated
	}
	project, err := s.LoadProjectCache(ctx, projectID)
	if err != nil || project == nil || !isProjectPluginActivated(project, pluginID) {
		return errPluginNotActivated
	}
	return nil
}

func envVarsMap(details *models.SavedPluginDetails) map[string]interface{} {
	if details == nil || len(details.EnvVars) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(details.EnvVars))
	for _, e := range details.EnvVars {
		if e == nil || e.Key == "" {
			continue
		}
		out[e.Key] = e.Value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *GraphQLServer) projectPluginEnvVars(ctx context.Context, projectID, pluginID string) map[string]interface{} {
	if s == nil || projectID == "" || pluginID == "" {
		return nil
	}
	project, err := s.LoadProjectCache(ctx, projectID)
	if err != nil || project == nil {
		return nil
	}
	return envVarsMap(findProjectPlugin(project.Plugins, pluginID))
}

func pluginProjectIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value("project_id"); v != nil {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	if cache, ok := utility.LegacyApplicationCache(ctx); ok && cache != nil && cache.Project != nil {
		return cache.Project.ID
	}
	return ""
}

func attachPluginEnvVars(contextData map[string]interface{}, env map[string]interface{}) {
	if len(env) == 0 {
		return
	}
	contextData["env_vars"] = env
	for k, v := range env {
		switch k {
		case "", "plugin_id", "project_id", "user_id", "env_vars":
			continue
		}
		contextData[k] = v
	}
}

func (s *GraphQLServer) pluginExecuteContext(ctx context.Context, pluginID, projectID string) map[string]interface{} {
	contextData := map[string]interface{}{
		"plugin_id": pluginID,
	}
	if projectID == "" {
		projectID = pluginProjectIDFromContext(ctx)
	}
	if projectID != "" {
		contextData["project_id"] = projectID
		attachPluginEnvVars(contextData, s.projectPluginEnvVars(ctx, projectID, pluginID))
	}
	if ctx != nil {
		if userID := ctx.Value("user_id"); userID != nil {
			if s, ok := userID.(string); ok {
				if s != "" {
					contextData["user_id"] = s
				}
			} else {
				contextData["user_id"] = fmt.Sprint(userID)
			}
		}
	}
	return contextData
}

func pluginProjectIDFromEcho(c echo.Context) string {
	if v := c.Get("project"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v := c.Request().Header.Get("X-Apito-Project-ID"); v != "" {
		return strings.TrimSpace(v)
	}
	if v := c.QueryParam("project_id"); v != "" {
		return v
	}
	return ""
}
