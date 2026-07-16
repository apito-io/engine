package functions

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NATSConn is the minimal NATS surface used by NATSFunctionTransport.
// Implementations wrap nats.Conn without importing nats into every build.
type NATSConn interface {
	RequestWithContext(ctx context.Context, subject string, data []byte) ([]byte, error)
	QueueSubscribe(subject, queue string, handler func(subject string, data []byte) ([]byte, error)) (func() error, error)
	Close()
}

// NATSFunctionTransport implements FunctionTransport over authenticated NATS.
type NATSFunctionTransport struct {
	conn   NATSConn
	mu     sync.Mutex
	closed bool
}

// NewNATSFunctionTransport wraps a NATS connection.
func NewNATSFunctionTransport(conn NATSConn) *NATSFunctionTransport {
	return &NATSFunctionTransport{conn: conn}
}

func (t *NATSFunctionTransport) Request(ctx context.Context, subject string, payload []byte, timeout time.Duration) ([]byte, error) {
	if t == nil || t.conn == nil {
		return nil, fmt.Errorf("nats function transport not configured")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return t.conn.RequestWithContext(ctx, subject, payload)
}

func (t *NATSFunctionTransport) RespondHandler(ctx context.Context, subject string, queueGroup string, handler func(payload []byte) ([]byte, error)) (func() error, error) {
	_ = ctx
	if t == nil || t.conn == nil {
		return nil, fmt.Errorf("nats function transport not configured")
	}
	if queueGroup == "" {
		queueGroup = "fn-workers"
	}
	return t.conn.QueueSubscribe(subject, queueGroup, func(_ string, data []byte) ([]byte, error) {
		return handler(data)
	})
}

func (t *NATSFunctionTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	if t.conn != nil {
		t.conn.Close()
	}
	return nil
}

// InvokeSubject builds the NATS subject for a synchronous function invoke.
func InvokeSubject(projectID, functionName string) string {
	return fmt.Sprintf("functions.%s.%s.invoke", projectID, functionName)
}

// DataGatewaySubject builds the NATS subject for worker→engine data RPC.
func DataGatewaySubject(projectID string) string {
	return fmt.Sprintf("functions.%s.data", projectID)
}

// EventSubject builds the JetStream-oriented subject for async function events.
func EventSubject(projectID, functionName string) string {
	return fmt.Sprintf("functions.%s.%s.event", projectID, functionName)
}
