package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-platform/graphql"
)

// RealtimeModelBaseTopic is the neutral fan-out topic for a model's change
// stream. Pro may rewrite it (tenant scoping) via Config.RealtimeTopicHook.
func RealtimeModelBaseTopic(projectID, model string) string {
	return fmt.Sprintf("project.%s.model.%s", projectID, model)
}

// RealtimeBroadcastBaseTopic is the neutral fan-out topic for a broadcast channel.
func RealtimeBroadcastBaseTopic(projectID, channel string) string {
	return fmt.Sprintf("project.%s.broadcast.%s", projectID, channel)
}

// realtimeTopic applies the optional host topic hook (tenant scoping) so emit and
// subscribe sides resolve to the same physical topic.
func (s *GraphQLServer) realtimeTopic(ctx context.Context, base string) string {
	if s.Cfg != nil && s.Cfg.RealtimeTopicHook != nil {
		return s.Cfg.RealtimeTopicHook(ctx, base)
	}
	return base
}

// EmitModelChange publishes a CREATED/UPDATED/DELETED event for a document onto
// the realtime bus. Best-effort: never blocks or fails the originating mutation.
func (s *GraphQLServer) EmitModelChange(ctx context.Context, projectID, model, event, id string, node interface{}) {
	if s == nil || s.RealtimeBus == nil || projectID == "" || model == "" {
		return
	}
	evt := &models.ModelChangeEvent{
		Event:     event,
		Model:     model,
		ProjectID: projectID,
		ID:        id,
		Node:      node,
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return
	}
	topic := s.realtimeTopic(ctx, RealtimeModelBaseTopic(projectID, model))
	_ = s.RealtimeBus.Publish(ctx, topic, payload)
}

// PublishBroadcast publishes a generic broadcast message to a channel.
func (s *GraphQLServer) PublishBroadcast(ctx context.Context, projectID, channel, event string, payload interface{}) error {
	if s == nil || s.RealtimeBus == nil {
		return fmt.Errorf("realtime bus is not configured")
	}
	evt := &models.BroadcastEvent{
		Channel:   channel,
		ProjectID: projectID,
		Event:     event,
		Payload:   payload,
		At:        time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	topic := s.realtimeTopic(ctx, RealtimeBroadcastBaseTopic(projectID, channel))
	return s.RealtimeBus.Publish(ctx, topic, data)
}

// changeEventFilter parses the `events: [ChangeEventType!]` argument.
func changeEventFilter(arg interface{}) map[string]bool {
	vals, ok := arg.([]interface{})
	if !ok || len(vals) == 0 {
		return nil // nil = all events
	}
	out := make(map[string]bool, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok {
			out[s] = true
		}
	}
	return out
}

// ModelChangedSubscribeFn builds the Subscribe function for a model's
// `<model>Changed` subscription field. It returns a channel of change-event maps
// filtered by the `events` and `id` arguments.
func (s *GraphQLServer) ModelChangedSubscribeFn(projectID, model string) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		if s.RealtimeBus == nil {
			return nil, fmt.Errorf("realtime bus is not configured")
		}
		wantEvents := changeEventFilter(p.Args["events"])
		idFilter, _ := p.Args["id"].(string)

		topic := s.realtimeTopic(p.Context, RealtimeModelBaseTopic(projectID, model))
		msgs, err := s.RealtimeBus.Subscribe(p.Context, topic)
		if err != nil {
			return nil, err
		}

		out := make(chan interface{}, 16)
		go func() {
			defer close(out)
			for raw := range msgs {
				var evt models.ModelChangeEvent
				if err := json.Unmarshal(raw, &evt); err != nil {
					continue
				}
				if wantEvents != nil && !wantEvents[evt.Event] {
					continue
				}
				if idFilter != "" && evt.ID != idFilter {
					continue
				}
				payload := map[string]interface{}{
					"event":          evt.Event,
					"id":             evt.ID,
					"model":          evt.Model,
					"node":           evt.Node,
					"previousValues": evt.PreviousValues,
				}
				select {
				case out <- payload:
				case <-p.Context.Done():
					return
				}
			}
		}()
		return out, nil
	}
}

// BroadcastSubscribeFn builds the Subscribe function for the generic
// `broadcast(channel)` subscription field.
func (s *GraphQLServer) BroadcastSubscribeFn(projectID string) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		if s.RealtimeBus == nil {
			return nil, fmt.Errorf("realtime bus is not configured")
		}
		channel, _ := p.Args["channel"].(string)
		if channel == "" {
			return nil, fmt.Errorf("channel is required")
		}
		topic := s.realtimeTopic(p.Context, RealtimeBroadcastBaseTopic(projectID, channel))
		msgs, err := s.RealtimeBus.Subscribe(p.Context, topic)
		if err != nil {
			return nil, err
		}
		out := make(chan interface{}, 16)
		go func() {
			defer close(out)
			for raw := range msgs {
				var evt models.BroadcastEvent
				if err := json.Unmarshal(raw, &evt); err != nil {
					continue
				}
				payload := map[string]interface{}{
					"channel": evt.Channel,
					"event":   evt.Event,
					"payload": evt.Payload,
					"at":      evt.At,
				}
				select {
				case out <- payload:
				case <-p.Context.Done():
					return
				}
			}
		}()
		return out, nil
	}
}

// ModelReadableForRole reports whether a role may read a model (used to shape the
// public subscription schema like the query/mutation schema).
func ModelReadableForRole(role *models.Role, modelName string) bool {
	if role == nil {
		return false
	}
	if role.ID == "admin" || role.IsAdmin {
		return true
	}
	if val, ok := utility.LookupAPIPermission(role, modelName); ok && val != nil {
		return val.Read != "" && val.Read != "none"
	}
	if perm, err := utility.BuildCRUDPermissions(modelName, role); err == nil && perm != nil {
		return perm.Read != "" && perm.Read != "none"
	}
	return false
}
