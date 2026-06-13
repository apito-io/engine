package nats

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	streamSystem  = "SYSTEM"
	streamProject = "PROJECT"
)

// NatsRealtimeBus is a NATS-backed fan-out bus with optional JetStream durability.
type NatsRealtimeBus struct {
	server    *natsserver.Server
	conn      *nats.Conn
	js        nats.JetStreamContext
	jetStream bool
}

var _ interfaces.RealtimeBus = (*NatsRealtimeBus)(nil)

// GetNatsRealtimeBus builds a NATS realtime bus, embedding a server when needed.
func GetNatsRealtimeBus(cfg *models.Config) (*NatsRealtimeBus, error) {
	jetStream := cfg.RealtimeNatsJetStream

	if cfg.RealtimeNatsURL != "" {
		conn, err := nats.Connect(cfg.RealtimeNatsURL,
			nats.Name("apito-engine"),
			nats.MaxReconnects(-1),
			nats.ReconnectWait(time.Second),
		)
		if err != nil {
			return nil, fmt.Errorf("realtime: connect external nats %s: %w", cfg.RealtimeNatsURL, err)
		}
		bus := &NatsRealtimeBus{conn: conn, jetStream: jetStream}
		if jetStream {
			if err := bus.initJetStream(); err != nil {
				conn.Close()
				return nil, err
			}
		}
		return bus, nil
	}

	opts := &natsserver.Options{
		ServerName: "apito-embedded",
		JetStream:  jetStream,
	}
	if jetStream {
		storeDir := cfg.RealtimeNatsStoreDir
		if storeDir == "" {
			storeDir = filepath.Join(cfg.DefaultDatabaseDir, "nats-jetstream")
		}
		if err := os.MkdirAll(storeDir, 0o755); err != nil {
			return nil, fmt.Errorf("realtime: jetstream store dir: %w", err)
		}
		opts.StoreDir = storeDir
	}
	if cfg.RealtimeNatsPort > 0 {
		opts.Host = "0.0.0.0"
		opts.Port = cfg.RealtimeNatsPort
	} else {
		opts.DontListen = true
	}

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("realtime: create embedded nats: %w", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		ns.Shutdown()
		return nil, fmt.Errorf("realtime: embedded nats not ready")
	}

	clientOpts := []nats.Option{nats.Name("apito-engine")}
	if cfg.RealtimeNatsPort <= 0 {
		clientOpts = append(clientOpts, nats.InProcessServer(ns))
	}
	conn, err := nats.Connect(ns.ClientURL(), clientOpts...)
	if err != nil {
		ns.Shutdown()
		return nil, fmt.Errorf("realtime: connect embedded nats: %w", err)
	}

	bus := &NatsRealtimeBus{server: ns, conn: conn, jetStream: jetStream}
	if jetStream {
		if err := bus.initJetStream(); err != nil {
			bus.Close()
			return nil, err
		}
	}
	return bus, nil
}

func (b *NatsRealtimeBus) initJetStream() error {
	js, err := b.conn.JetStream()
	if err != nil {
		return fmt.Errorf("realtime: jetstream context: %w", err)
	}
	b.js = js

	for _, spec := range []nats.StreamConfig{
		{
			Name:      streamSystem,
			Subjects:  []string{"system.>"},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		},
		{
			Name:      streamProject,
			Subjects:  []string{"project.>"},
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		},
	} {
		if _, err := js.AddStream(&spec); err != nil {
			if _, uerr := js.UpdateStream(&spec); uerr != nil {
				return fmt.Errorf("realtime: ensure stream %s: %w", spec.Name, err)
			}
		}
	}
	return nil
}

// Publish sends payload on the NATS subject (JetStream when enabled).
func (b *NatsRealtimeBus) Publish(_ context.Context, topic string, payload []byte) error {
	if b.jetStream && b.js != nil {
		_, err := b.js.Publish(topic, payload)
		return err
	}
	return b.conn.Publish(topic, payload)
}

// Subscribe creates a live-tail subscription (DeliverNew when JetStream is on).
func (b *NatsRealtimeBus) Subscribe(ctx context.Context, topic string) (<-chan []byte, error) {
	return b.subscribeInternal(ctx, topic, interfaces.ReplayPolicy{}, true)
}

// SubscribeReplay delivers historical messages per policy then continues live.
func (b *NatsRealtimeBus) SubscribeReplay(ctx context.Context, topic string, policy interfaces.ReplayPolicy) (<-chan []byte, error) {
	return b.subscribeInternal(ctx, topic, policy, false)
}

func (b *NatsRealtimeBus) subscribeInternal(ctx context.Context, topic string, policy interfaces.ReplayPolicy, liveOnly bool) (<-chan []byte, error) {
	out := make(chan []byte, 64)

	var sub *nats.Subscription
	var err error

	if b.jetStream && b.js != nil {
		opts := []nats.SubOpt{nats.AckNone()}
		if liveOnly {
			opts = append(opts, nats.DeliverNew())
		} else {
			switch {
			case policy.SinceSeq > 0:
				opts = append(opts, nats.StartSequence(policy.SinceSeq))
			case policy.SinceTime != nil:
				opts = append(opts, nats.StartTime(*policy.SinceTime))
			case policy.LastPerSubject:
				opts = append(opts, nats.DeliverLastPerSubject())
			default:
				opts = append(opts, nats.DeliverAll())
			}
		}
		sub, err = b.js.Subscribe(topic, func(msg *nats.Msg) {
			select {
			case out <- msg.Data:
			default:
			}
		}, opts...)
	} else {
		sub, err = b.conn.Subscribe(topic, func(msg *nats.Msg) {
			select {
			case out <- msg.Data:
			default:
			}
		})
	}

	if err != nil {
		close(out)
		return nil, fmt.Errorf("realtime: subscribe %s: %w", topic, err)
	}

	go func() {
		<-ctx.Done()
		_ = sub.Unsubscribe()
		close(out)
	}()

	return out, nil
}

// Close drains the connection and shuts down the embedded server if present.
func (b *NatsRealtimeBus) Close() error {
	if b.conn != nil {
		_ = b.conn.Drain()
	}
	if b.server != nil {
		b.server.Shutdown()
	}
	return nil
}
