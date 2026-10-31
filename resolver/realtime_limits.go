package resolver

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
)

const (
	defaultSubsPerConnection = 32
	defaultSubsPerProject    = 2000
)

var (
	realtimeIDPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	realtimeChannelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

	realtimePublishes atomic.Int64
	realtimeDrops     atomic.Int64
	realtimeFiltered  atomic.Int64
	realtimeAuthDeny  atomic.Int64
	realtimeActive    atomic.Int64
)

// RealtimeMetrics is a point-in-time counter snapshot.
type RealtimeMetrics struct {
	Publishes           int64
	Drops               int64
	Filtered            int64
	AuthDenials         int64
	ActiveSubscriptions int64
}

func RealtimeMetricsSnapshot() RealtimeMetrics {
	return RealtimeMetrics{
		Publishes:           realtimePublishes.Load(),
		Drops:               realtimeDrops.Load(),
		Filtered:            realtimeFiltered.Load(),
		AuthDenials:         realtimeAuthDeny.Load(),
		ActiveSubscriptions: realtimeActive.Load(),
	}
}

func noteRealtimeDrop()     { realtimeDrops.Add(1) }
func noteRealtimePublish()  { realtimePublishes.Add(1) }
func noteRealtimeFiltered() { realtimeFiltered.Add(1) }
func noteRealtimeAuthDeny() { realtimeAuthDeny.Add(1) }

func validateChannel(channel string) error {
	if !realtimeChannelPattern.MatchString(channel) {
		return fmt.Errorf("invalid broadcast channel")
	}
	return nil
}

func realtimeModelIDTopic(projectID, model, id string) (string, error) {
	if !realtimeIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid subscription id")
	}
	return RealtimeModelBaseTopic(projectID, model) + ".id." + id, nil
}

type subscriptionSlots struct {
	mu         sync.Mutex
	perConn    map[string]int
	perProject map[string]int
}

func newSubscriptionSlots() *subscriptionSlots {
	return &subscriptionSlots{
		perConn:    map[string]int{},
		perProject: map[string]int{},
	}
}

var slots = newSubscriptionSlots()

func (s *GraphQLServer) reserveRealtimeSubscription(ctx context.Context, projectID string) (func(), error) {
	connLimit := defaultSubsPerConnection
	projectLimit := defaultSubsPerProject
	if s != nil && s.Cfg != nil {
		if s.Cfg.RealtimeMaxSubscriptionsPerConnection > 0 {
			connLimit = s.Cfg.RealtimeMaxSubscriptionsPerConnection
		}
		if s.Cfg.RealtimeMaxSubscriptionsPerProject > 0 {
			projectLimit = s.Cfg.RealtimeMaxSubscriptionsPerProject
		}
	}
	auth := realtimeAuthFrom(ctx)
	connID := auth.ConnID
	if connID == "" {
		connID = "default"
	}
	if projectID == "" {
		projectID = "default"
	}
	slots.mu.Lock()
	defer slots.mu.Unlock()
	if slots.perConn[connID] >= connLimit {
		return nil, fmt.Errorf("too many subscriptions on this connection")
	}
	if slots.perProject[projectID] >= projectLimit {
		return nil, fmt.Errorf("too many subscriptions on this project")
	}
	slots.perConn[connID]++
	slots.perProject[projectID]++
	realtimeActive.Add(1)
	var once sync.Once
	return func() {
		once.Do(func() {
			slots.mu.Lock()
			slots.perConn[connID]--
			if slots.perConn[connID] <= 0 {
				delete(slots.perConn, connID)
			}
			slots.perProject[projectID]--
			if slots.perProject[projectID] <= 0 {
				delete(slots.perProject, projectID)
			}
			slots.mu.Unlock()
			realtimeActive.Add(-1)
		})
	}, nil
}
