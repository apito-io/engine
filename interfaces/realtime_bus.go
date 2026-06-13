package interfaces

import (
	"context"
	"time"
)

// ReplayPolicy controls how SubscribeReplay delivers historical messages from
// JetStream. When all fields are zero, replay behaves like a live tail only.
type ReplayPolicy struct {
	// SinceSeq delivers messages with stream sequence >= SinceSeq (1-based).
	SinceSeq uint64
	// SinceTime delivers messages published at or after SinceTime.
	SinceTime *time.Time
	// LastPerSubject when true delivers only the last message per subject
	// (useful for catch-up snapshots).
	LastPerSubject bool
}

// RealtimeBus is a low-latency fan-out message bus used to power GraphQL
// subscriptions (per-model change streams, broadcast channels, and system
// operator notifications). NATS-backed implementations use JetStream for
// durable delivery and replay; the memory backend is single-node and
// non-durable.
//
// Topics use dot-separated segments (e.g. "project.<id>.model.<model>") so
// NATS implementations can use native subject hierarchies and wildcards.
type RealtimeBus interface {
	// Publish delivers payload to every subscriber currently subscribed to topic.
	Publish(ctx context.Context, topic string, payload []byte) error

	// Subscribe returns a receive-only channel of payloads for topic. The channel
	// is closed when ctx is cancelled or the bus is closed. Implementations must
	// fan out: multiple concurrent subscribers to the same topic each receive a
	// copy of every message published after subscription (live tail).
	Subscribe(ctx context.Context, topic string) (<-chan []byte, error)

	// SubscribeReplay returns a channel that may include historical messages per
	// policy before continuing with live delivery. Memory backend aliases Subscribe.
	SubscribeReplay(ctx context.Context, topic string, policy ReplayPolicy) (<-chan []byte, error)

	// Close releases all resources (and, for the embedded NATS backend, shuts
	// down the in-process server).
	Close() error
}
