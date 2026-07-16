package functions

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LocalTransport is an in-process FunctionTransport for self-host/dev and tests.
type LocalTransport struct {
	mu       sync.RWMutex
	handlers map[string]func(payload []byte) ([]byte, error)
	closed   bool
}

// NewLocalTransport creates an empty local transport.
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{handlers: make(map[string]func(payload []byte) ([]byte, error))}
}

func (t *LocalTransport) Request(ctx context.Context, subject string, payload []byte, timeout time.Duration) ([]byte, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, fmt.Errorf("function transport closed")
	}
	h, ok := t.handlers[subject]
	t.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no handler for subject %q", subject)
	}
	type result struct {
		b   []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := h(payload)
		ch <- result{b, err}
	}()
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	select {
	case r := <-ch:
		return r.b, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("function transport request timeout")
	}
}

func (t *LocalTransport) RespondHandler(ctx context.Context, subject string, _ string, handler func(payload []byte) ([]byte, error)) (func() error, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil, fmt.Errorf("function transport closed")
	}
	t.handlers[subject] = handler
	return func() error {
		t.mu.Lock()
		defer t.mu.Unlock()
		delete(t.handlers, subject)
		return nil
	}, nil
}

func (t *LocalTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	t.handlers = nil
	return nil
}
