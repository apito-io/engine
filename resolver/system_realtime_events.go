package resolver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apito-io/engine/models"
)

// RealtimeSystemConsoleNotifyTopic is the engine-wide console operator broadcast subject.
const RealtimeSystemConsoleNotifyTopic = "system.console.notify"

// pluginStatusEventPayload is the JSON message body for plugin_status_changed events.
type pluginStatusEventPayload struct {
	PluginID string `json:"plugin_id"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

// projectLifecycleEventPayload is the JSON message body for project_created/updated events.
type projectLifecycleEventPayload struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name,omitempty"`
}

// PublishConsoleNotify broadcasts an operator notification to all console subscribers.
func (s *GraphQLServer) PublishConsoleNotify(ctx context.Context, data *models.SubscriptionEvent) error {
	if s == nil || s.RealtimeBus == nil || data == nil {
		return fmt.Errorf("realtime bus is not configured")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	topic := s.realtimeTopic(ctx, RealtimeSystemConsoleNotifyTopic)
	if err := s.RealtimeBus.Publish(ctx, topic, payload); err != nil {
		return err
	}
	if data.ProjectID != "" && data.Type != "" {
		projectTopic := s.realtimeTopic(ctx, RealtimeSystemProjectEventTopic(data.ProjectID, data.Type))
		_ = s.RealtimeBus.Publish(ctx, projectTopic, payload)
	}
	return nil
}

// EmitPluginStatusChanged notifies console clients of a plugin load/health transition.
func (s *GraphQLServer) EmitPluginStatusChanged(ctx context.Context, pluginID, status, reason string) {
	if s == nil || pluginID == "" {
		return
	}
	body, _ := json.Marshal(pluginStatusEventPayload{
		PluginID: pluginID,
		Status:   status,
		Reason:   reason,
	})
	_ = s.PublishConsoleNotify(ctx, &models.SubscriptionEvent{
		Type:    models.SystemEventPluginStatusChanged,
		Message: string(body),
	})
}

// EmitProjectLifecycle notifies the project owner of create/update events.
func (s *GraphQLServer) EmitProjectLifecycle(ctx context.Context, userID, projectID, projectName, eventType string) {
	if s == nil || userID == "" || projectID == "" {
		return
	}
	body, _ := json.Marshal(projectLifecycleEventPayload{
		ProjectID:   projectID,
		ProjectName: projectName,
	})
	_ = s.PublishSystemMessage(ctx, userID, &models.SubscriptionEvent{
		Type:      eventType,
		ProjectID: projectID,
		UserID:    userID,
		Message:   string(body),
	})
}
