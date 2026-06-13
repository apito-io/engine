package nats

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func testNatsBus(t *testing.T, jetStream bool) *NatsRealtimeBus {
	t.Helper()
	storeDir := filepath.Join(t.TempDir(), "js")
	cfg := &models.Config{
		RealtimeNatsJetStream: jetStream,
		RealtimeNatsStoreDir:  storeDir,
		DefaultDatabaseDir:      t.TempDir(),
	}
	bus, err := GetNatsRealtimeBus(cfg)
	if err != nil {
		t.Fatalf("GetNatsRealtimeBus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func TestNatsRealtimeBus_EmbeddedRoundTrip(t *testing.T) {
	bus := testNatsBus(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Subscribe(ctx, "project.test.model.author")
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte(`{"event":"CREATED"}`)
	if err := bus.Publish(ctx, "project.test.model.author", msg); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-ch:
		if string(got) != string(msg) {
			t.Fatalf("got %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestNatsRealtimeBus_JetStreamPublishSubscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("jetstream embedded nats")
	}
	bus := testNatsBus(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topic := "system.user.u1.notify"
	payload := []byte(`{"type":"info","message":"hi"}`)
	if err := bus.Publish(ctx, topic, payload); err != nil {
		t.Fatal(err)
	}

	ch, err := bus.SubscribeReplay(ctx, topic, interfaces.ReplayPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-ch:
		if string(got) != string(payload) {
			t.Fatalf("replay got %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("jetstream replay timeout")
	}
}
