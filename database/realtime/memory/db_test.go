package memory

import (
	"context"
	"testing"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func TestMemoryRealtimeBus_PublishSubscribe(t *testing.T) {
	bus, err := GetMemoryRealtimeBus(&models.Config{})
	if err != nil {
		t.Fatalf("GetMemoryRealtimeBus: %v", err)
	}
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Subscribe(ctx, "test.topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	payload := []byte(`{"hello":"world"}`)
	if err := bus.Publish(ctx, "test.topic", payload); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case got := <-ch:
		if string(got) != string(payload) {
			t.Fatalf("got %q want %q", got, payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestMemoryRealtimeBus_FanOut(t *testing.T) {
	bus, err := GetMemoryRealtimeBus(&models.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch1, _ := bus.Subscribe(ctx, "fanout")
	ch2, _ := bus.Subscribe(ctx, "fanout")

	_ = bus.Publish(ctx, "fanout", []byte("x"))

	got := 0
	deadline := time.After(time.Second)
	for got < 2 {
		select {
		case <-ch1:
			got++
		case <-ch2:
			got++
		case <-deadline:
			t.Fatalf("fan-out got %d/2", got)
		}
	}
}

func TestMemoryRealtimeBus_SubscribeReplayAlias(t *testing.T) {
	bus, err := GetMemoryRealtimeBus(&models.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.SubscribeReplay(ctx, "replay", interfaces.ReplayPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	_ = bus.Publish(ctx, "replay", []byte("live"))
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("SubscribeReplay alias failed")
	}
}

func TestMemoryRealtimeBus_ContextCancel(t *testing.T) {
	bus, err := GetMemoryRealtimeBus(&models.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := bus.Subscribe(ctx, "cancel.me")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should close after ctx cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("channel did not close")
	}
}
