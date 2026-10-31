package resolver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/open_driver/realtime/memory"
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

func TestModelChangedSubscribeFn_WhereAndTransition(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subFn := s.ModelChangedSubscribeFn("p1", "note")
	ch, err := subFn(graphql.ResolveParams{
		Context: ctx,
		Args: map[string]interface{}{
			"where": map[string]interface{}{"title": map[string]interface{}{"eq": "keep"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	s.EmitModelChange(ctx, "p1", "note", models.ChangeEventCreated, "1", map[string]interface{}{"title": "drop"})
	s.EmitModelChangeWithPrevious(ctx, "p1", "note", models.ChangeEventUpdated, "2",
		map[string]interface{}{"title": "drop"},
		map[string]interface{}{"title": "keep"},
	)

	select {
	case raw := <-ch.(chan interface{}):
		m := raw.(map[string]interface{})
		if m["id"] != "2" {
			t.Fatalf("id=%v", m["id"])
		}
		prev := m["previousValues"].(map[string]interface{})
		if prev["title"] != "keep" {
			t.Fatalf("previous=%v", prev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestModelChangedSubscribeFn_RejectsUnsupportedWhere(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	subFn := s.ModelChangedSubscribeFn("p1", "note")
	_, err := subFn(graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"where": map[string]interface{}{"relation": map[string]interface{}{"eq": "x"}},
		},
	})
	if err == nil {
		t.Fatal("expected unsupported where to fail")
	}
}

func TestModelChangedSubscribeFn_IDTopic(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subFn := s.ModelChangedSubscribeFn("p1", "note")
	ch, err := subFn(graphql.ResolveParams{
		Context: ctx,
		Args:    map[string]interface{}{"id": "doc-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.EmitModelChange(ctx, "p1", "note", models.ChangeEventCreated, "doc-2", map[string]interface{}{"title": "other"})
	s.EmitModelChange(ctx, "p1", "note", models.ChangeEventCreated, "doc-1", map[string]interface{}{"title": "mine"})

	select {
	case raw := <-ch.(chan interface{}):
		m := raw.(map[string]interface{})
		if m["id"] != "doc-1" {
			t.Fatalf("id=%v", m["id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestModelChangedSubscribeFn_OwnRowDenied(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	role := &models.Role{
		IsProjectUser: true,
		APIPermissions: map[string]*models.APIPermission{
			"note": {Read: "own"},
		},
	}
	ctx = WithRealtimeAuth(ctx, role, "user-a", "conn-own")
	subFn := s.ModelChangedSubscribeFn("p1", "note")
	ch, err := subFn(graphql.ResolveParams{Context: ctx, Args: map[string]interface{}{}})
	if err != nil {
		t.Fatal(err)
	}
	other := map[string]interface{}{"id": "doc-b", "data": map[string]interface{}{"owner_id": "user-b"}}
	mine := map[string]interface{}{"id": "doc-a", "data": map[string]interface{}{"owner_id": "user-a"}}
	s.EmitModelChange(ctx, "p1", "note", models.ChangeEventCreated, "doc-b", other)
	s.EmitModelChange(ctx, "p1", "note", models.ChangeEventCreated, "doc-a", mine)
	select {
	case raw := <-ch.(chan interface{}):
		m := raw.(map[string]interface{})
		if m["id"] != "doc-a" {
			t.Fatalf("id=%v", m["id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestBroadcastChannelRejected(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	if err := s.PublishBroadcast(context.Background(), "p1", "bad channel", "msg", nil); err == nil {
		t.Fatal("expected invalid channel")
	}
	s.Cfg.RealtimeBroadcastAuthHook = func(context.Context, string, string) error {
		return context.Canceled
	}
	if err := s.PublishBroadcast(context.Background(), "p1", "chat", "msg", nil); err == nil {
		t.Fatal("expected hook denial")
	}
}

func TestSubscriptionLimit(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	s.Cfg.RealtimeMaxSubscriptionsPerConnection = 1
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = WithRealtimeAuth(ctx, nil, "", "conn-limit")
	subFn := s.ModelChangedSubscribeFn("p1", "note")
	if _, err := subFn(graphql.ResolveParams{Context: ctx, Args: map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	_, err := subFn(graphql.ResolveParams{Context: ctx, Args: map[string]interface{}{}})
	if err == nil {
		t.Fatal("expected connection limit")
	}
}

func TestSlowConsumerTerminatesSubscription(t *testing.T) {
	s := testMemoryGraphQLServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subFn := s.ModelChangedSubscribeFn("p1", "note")
	ch, err := subFn(graphql.ResolveParams{Context: ctx, Args: map[string]interface{}{}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		s.EmitModelChange(ctx, "p1", "note", models.ChangeEventCreated, "doc", map[string]interface{}{"n": i})
	}
	time.Sleep(200 * time.Millisecond)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch.(chan interface{}):
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("lagging subscription was not terminated")
		}
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
