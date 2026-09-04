package resolver

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/apito-io/engine/models"
	pluginService "github.com/apito-io/engine/services/plugin"
)

var defaultOrderModels = map[string]struct{}{
	"order":   {},
	"orders":  {},
	"invoice": {},
	"invoices": {},
}

// NotifyEventSinkPlugins fans document/auth/error events to activated
// system.events plugins. Best-effort: never fails the caller.
func (s *GraphQLServer) NotifyEventSinkPlugins(ctx context.Context, projectID, event string, payload map[string]interface{}) {
	if s == nil || s.ProjectCache == nil {
		return
	}
	if projectID == "" || event == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	detached := context.WithoutCancel(ctx)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("event sink panic: %v", rec)
			}
		}()
		s.dispatchEventSink(detached, projectID, event, payload)
	}()
}

func (s *GraphQLServer) dispatchEventSink(ctx context.Context, projectID, event string, payload map[string]interface{}) {
	project, err := s.LoadProjectCache(ctx, projectID)
	if err != nil || project == nil {
		return
	}
	for _, p := range project.Plugins {
		if p == nil || !isSavedPluginActivated(p) {
			continue
		}
		if !pluginService.HasCapability(p.ID, pluginService.CapEventSink) {
			continue
		}
		s.executePluginEventSink(ctx, p.ID, projectID, event, payload)
	}
}

func (s *GraphQLServer) executePluginEventSink(ctx context.Context, pluginID, projectID, event string, payload map[string]interface{}) {
	plugin := s.tryGetPluginNoBlock(pluginID)
	if plugin == nil {
		return
	}
	rpcClient, err := plugin.Client.Client()
	if err != nil {
		log.Printf("event sink rpc %s: %v", pluginID, err)
		return
	}
	raw, err := rpcClient.Dispense(plugin.PluginConfigurations.ExportedVariable)
	if err != nil {
		log.Printf("event sink dispense %s: %v", pluginID, err)
		return
	}
	loadedPlugin, ok := raw.(*pluginService.HashiCorpNormalPluginGRPC)
	if !ok {
		return
	}
	args := map[string]interface{}{
		"event": event,
	}
	if payload != nil {
		args["payload"] = payload
	}
	contextData := s.pluginExecuteContext(ctx, pluginID, projectID)
	if _, err := loadedPlugin.Execute(ctx, "event_sink", "function", args, contextData); err != nil {
		log.Printf("event sink execute %s: %v", pluginID, err)
	}
}

func (s *GraphQLServer) notifyDocumentEventSink(ctx context.Context, projectID, model, changeEvent, id string, node interface{}) {
	if projectID == "" || model == "" {
		return
	}
	payload := map[string]interface{}{
		"model":         model,
		"change_event":  changeEvent,
		"id":            id,
		"document":      nodeToMap(node),
	}
	s.NotifyEventSinkPlugins(ctx, projectID, "publish", payload)
	if isOrderModelName(model) {
		s.NotifyEventSinkPlugins(ctx, projectID, "order", payload)
	}
}

func isOrderModelName(model string) bool {
	key := strings.ToLower(strings.TrimSpace(model))
	_, ok := defaultOrderModels[key]
	return ok
}

func nodeToMap(node interface{}) interface{} {
	if node == nil {
		return nil
	}
	if m, ok := node.(map[string]interface{}); ok {
		return m
	}
	return fmt.Sprintf("%v", node)
}

func activatedEventSinkIDs(project *models.Project) []string {
	if project == nil {
		return nil
	}
	var ids []string
	for _, p := range project.Plugins {
		if p == nil || !isSavedPluginActivated(p) {
			continue
		}
		if pluginService.HasCapability(p.ID, pluginService.CapEventSink) {
			ids = append(ids, p.ID)
		}
	}
	return ids
}
