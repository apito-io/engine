package resolver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gitlab.com/apito.io/open_driver/realtime/memory"
	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

func testMemoryGraphQLServer(t *testing.T) *GraphQLServer {
	t.Helper()
	bus, err := memory.GetMemoryRealtimeBus(&models.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return &GraphQLServer{RealtimeBus: bus, Cfg: &models.Config{}}
}

func TestEmitModelChange_ModelChangedSubscribeFn(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	projectID := "proj1"
	model := "author"
	subFn := s.ModelChangedSubscribeFn(projectID, model)
	ch, err := subFn(graphql.ResolveParams{Context: ctx, Args: map[string]interface{}{}})
	if err != nil {
		t.Fatal(err)
	}

	s.EmitModelChange(ctx, projectID, model, models.ChangeEventCreated, "doc1", map[string]interface{}{"title": "A"})

	select {
	case raw := <-ch.(chan interface{}):
		m := raw.(map[string]interface{})
		if m["event"] != models.ChangeEventCreated {
			t.Fatalf("event=%v", m["event"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestModelChangedSubscribeFn_EventFilter(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subFn := s.ModelChangedSubscribeFn("p1", "author")
	ch, err := subFn(graphql.ResolveParams{
		Context: ctx,
		Args: map[string]interface{}{
			"events": []interface{}{models.ChangeEventDeleted},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	s.EmitModelChange(ctx, "p1", "author", models.ChangeEventCreated, "1", nil)
	s.EmitModelChange(ctx, "p1", "author", models.ChangeEventDeleted, "1", nil)

	select {
	case raw := <-ch.(chan interface{}):
		m := raw.(map[string]interface{})
		if m["event"] != models.ChangeEventDeleted {
			t.Fatalf("event=%v", m["event"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for filtered event")
	}
}

func TestPublishBroadcast_BroadcastSubscribeFn(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subFn := s.BroadcastSubscribeFn("proj1")
	ch, err := subFn(graphql.ResolveParams{
		Context: ctx,
		Args:    map[string]interface{}{"channel": "chat"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.PublishBroadcast(ctx, "proj1", "chat", "msg", map[string]string{"text": "hi"}); err != nil {
		t.Fatal(err)
	}

	select {
	case raw := <-ch.(chan interface{}):
		m := raw.(map[string]interface{})
		if m["channel"] != "chat" {
			t.Fatalf("channel=%v", m["channel"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestPublishSystemMessage_UserTopic(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	userID := "user-1"
	msgs, err := s.RealtimeBus.Subscribe(ctx, RealtimeSystemUserNotifyTopic(userID))
	if err != nil {
		t.Fatal(err)
	}

	err = s.PublishSystemMessage(ctx, userID, &models.SubscriptionEvent{
		Type:      models.SystemEventInfo,
		ProjectID: "p1",
		Message:   "hello",
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case raw := <-msgs:
		var evt models.SubscriptionEvent
		if err := json.Unmarshal(raw, &evt); err != nil {
			t.Fatal(err)
		}
		if evt.Type != models.SystemEventInfo || evt.Message != "hello" {
			t.Fatalf("evt=%+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestPublishConsoleNotify_ConsoleTopic(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgs, err := s.RealtimeBus.Subscribe(ctx, RealtimeSystemConsoleNotifyTopic)
	if err != nil {
		t.Fatal(err)
	}

	s.EmitPluginStatusChanged(ctx, "hc-s3", "loaded", "")
	select {
	case raw := <-msgs:
		var evt models.SubscriptionEvent
		_ = json.Unmarshal(raw, &evt)
		if evt.Type != models.SystemEventPluginStatusChanged {
			t.Fatalf("type=%s", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
