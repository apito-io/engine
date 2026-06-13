package memory

import (
	"context"
	"sync"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// MemoryRealtimeBus is an in-process fan-out bus. Every subscriber to a topic
// receives a copy of every published message. It is single-node only and
// non-durable, which is exactly the right model for low-latency GraphQL
// subscriptions on a single engine instance.
type MemoryRealtimeBus struct {
	mu     sync.RWMutex
	topics map[string]map[*subscriber]struct{}
	closed bool
}

type subscriber struct {
	ch chan []byte
}

// Ensure MemoryRealtimeBus implements RealtimeBus
var _ interfaces.RealtimeBus = (*MemoryRealtimeBus)(nil)

// GetMemoryRealtimeBus returns an in-process realtime bus.
func GetMemoryRealtimeBus(_ *models.Config) (*MemoryRealtimeBus, error) {
	return &MemoryRealtimeBus{
		topics: make(map[string]map[*subscriber]struct{}),
	}, nil
}

// Publish delivers payload to every active subscriber of topic (non-blocking).
func (b *MemoryRealtimeBus) Publish(_ context.Context, topic string, payload []byte) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil
	}
	for sub := range b.topics[topic] {
		// Non-blocking send: a slow subscriber must never stall the publisher.
		select {
		case sub.ch <- payload:
		default:
		}
	}
	return nil
}

// Subscribe registers a new subscriber channel for topic and removes it when
// ctx is cancelled.
func (b *MemoryRealtimeBus) Subscribe(ctx context.Context, topic string) (<-chan []byte, error) {
	sub := &subscriber{ch: make(chan []byte, 64)}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		close(sub.ch)
		return sub.ch, nil
	}
	if b.topics[topic] == nil {
		b.topics[topic] = make(map[*subscriber]struct{})
	}
	b.topics[topic][sub] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		if subs, ok := b.topics[topic]; ok {
			delete(subs, sub)
			if len(subs) == 0 {
				delete(b.topics, topic)
			}
		}
		b.mu.Unlock()
		close(sub.ch)
	}()

	return sub.ch, nil
}

// Close marks the bus closed; subscriber channels close via their ctx.
func (b *MemoryRealtimeBus) Close() error {
	b.mu.Lock()
	b.closed = true
	b.topics = make(map[string]map[*subscriber]struct{})
	b.mu.Unlock()
	return nil
}

// SubscribeReplay on the memory bus is a no-op alias of Subscribe (no durability).
func (b *MemoryRealtimeBus) SubscribeReplay(ctx context.Context, topic string, _ interfaces.ReplayPolicy) (<-chan []byte, error) {
	return b.Subscribe(ctx, topic)
}
