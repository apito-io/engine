package functions

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventDeliveryStatus for async function invocations.
const (
	EventAccepted     = "accepted"
	EventRunning      = "running"
	EventCommitted    = "committed"
	EventFailed       = "failed"
	EventDeadLettered = "dead_lettered"
)

// EventMessage is an async trigger payload (webhook, email, payment, lifecycle).
type EventMessage struct {
	ID             string                 `json:"id"`
	ProjectID      string                 `json:"project_id"`
	Function       string                 `json:"function"`
	Trigger        string                 `json:"trigger"` // webhook, email, payment, lifecycle
	IdempotencyKey string                 `json:"idempotency_key"`
	Payload        map[string]interface{} `json:"payload"`
	Attempt        int                    `json:"attempt"`
	MaxDeliver     int                    `json:"max_deliver"`
}

// EventDispatcher accepts async events for at-least-once delivery.
type EventDispatcher interface {
	Publish(ctx context.Context, msg *EventMessage) error
	Subscribe(ctx context.Context, handler func(ctx context.Context, msg *EventMessage) error) (func() error, error)
}

// MemoryEventDispatcher is an in-process event bus for tests/self-host without JetStream.
type MemoryEventDispatcher struct {
	mu       sync.Mutex
	handlers []func(ctx context.Context, msg *EventMessage) error
	dlq      []*EventMessage
}

func NewMemoryEventDispatcher() *MemoryEventDispatcher {
	return &MemoryEventDispatcher{}
}

func (d *MemoryEventDispatcher) Publish(ctx context.Context, msg *EventMessage) error {
	if msg == nil {
		return fmt.Errorf("nil event")
	}
	if msg.MaxDeliver <= 0 {
		msg.MaxDeliver = 5
	}
	d.mu.Lock()
	handlers := append([]func(context.Context, *EventMessage) error{}, d.handlers...)
	d.mu.Unlock()
	if len(handlers) == 0 {
		return nil
	}
	for _, h := range handlers {
		msg.Attempt++
		if err := h(ctx, msg); err != nil {
			if msg.Attempt >= msg.MaxDeliver {
				d.mu.Lock()
				d.dlq = append(d.dlq, msg)
				d.mu.Unlock()
				return fmt.Errorf("dead-lettered after %d attempts: %w", msg.Attempt, err)
			}
			// simple backoff
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(msg.Attempt) * 50 * time.Millisecond):
			}
			return d.Publish(ctx, msg)
		}
	}
	return nil
}

func (d *MemoryEventDispatcher) Subscribe(ctx context.Context, handler func(ctx context.Context, msg *EventMessage) error) (func() error, error) {
	_ = ctx
	d.mu.Lock()
	d.handlers = append(d.handlers, handler)
	idx := len(d.handlers) - 1
	d.mu.Unlock()
	return func() error {
		d.mu.Lock()
		defer d.mu.Unlock()
		if idx >= 0 && idx < len(d.handlers) {
			d.handlers = append(d.handlers[:idx], d.handlers[idx+1:]...)
		}
		return nil
	}, nil
}

func (d *MemoryEventDispatcher) DeadLetters() []*EventMessage {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]*EventMessage, len(d.dlq))
	copy(out, d.dlq)
	return out
}
