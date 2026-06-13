package resolver

import (
	"encoding/json"
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

// RealtimeSystemUserNotifyTopic is the per-user subject for operator notifications.
func RealtimeSystemUserNotifyTopic(userID string) string {
	return fmt.Sprintf("system.user.%s.notify", userID)
}

// RealtimeSystemProjectEventTopic is the project-scoped subject for fan-out events.
func RealtimeSystemProjectEventTopic(projectID, eventType string) string {
	return fmt.Sprintf("system.project.%s.%s", projectID, eventType)
}

func (s *GraphQLServer) SendEvent(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	var messageText string
	if val, ok := p.Args["message"].(string); ok {
		messageText = val
	}

	data := &models.SubscriptionEvent{
		ProjectID: param.ProjectID,
		UserID:    param.UserID,
		Message:   messageText,
		Type:      models.SystemEventInfo,
	}

	if err := s.PublishSystemMessage(p.Context, param.UserID, data); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message": "event published to realtime bus",
	}, nil
}

func (s *GraphQLServer) EventSubscription(p graphql.ResolveParams) (interface{}, error) {
	if s.RealtimeBus == nil {
		return nil, fmt.Errorf("realtime bus is not configured")
	}

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	typeFilter, _ := p.Args["type"].(string)

	topic := s.realtimeTopic(p.Context, RealtimeSystemUserNotifyTopic(param.UserID))
	msgs, err := s.RealtimeBus.Subscribe(p.Context, topic)
	if err != nil {
		return nil, err
	}

	consoleTopic := s.realtimeTopic(p.Context, RealtimeSystemConsoleNotifyTopic)
	consoleMsgs, err := s.RealtimeBus.Subscribe(p.Context, consoleTopic)
	if err != nil {
		return nil, err
	}

	out := make(chan interface{}, 16)
	go func() {
		defer close(out)
		forward := func(raw []byte) {
			var evt models.SubscriptionEvent
			if err := json.Unmarshal(raw, &evt); err != nil {
				return
			}
			if typeFilter != "" && evt.Type != typeFilter {
				return
			}
			payload := map[string]interface{}{
				"message":    evt.Message,
				"type":       evt.Type,
				"project_id": evt.ProjectID,
			}
			select {
			case out <- payload:
			case <-p.Context.Done():
			}
		}
		for {
			select {
			case <-p.Context.Done():
				return
			case raw, ok := <-msgs:
				if !ok {
					msgs = nil
				} else {
					forward(raw)
				}
			case raw, ok := <-consoleMsgs:
				if !ok {
					consoleMsgs = nil
				} else {
					forward(raw)
				}
			}
			if msgs == nil && consoleMsgs == nil {
				return
			}
		}
	}()

	return out, nil
}
