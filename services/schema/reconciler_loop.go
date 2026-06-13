package schema

import (
	"context"
	"log"
	"time"
)

// RunBackgroundReconciler periodically lists repairable operations for observability.
// Full automatic replay requires operation-specific handlers; mutations remain synchronous.
func RunBackgroundReconciler(ctx context.Context, h Hooks, projectID string, interval time.Duration) {
	if h.Store == nil || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ops, err := ListRepairable(ctx, h.Store, projectID, 20)
			if err != nil {
				log.Printf("[schema-reconciler] list repairable: %v", err)
				continue
			}
			for _, op := range ops {
				if op == nil {
					continue
				}
				log.Printf("[schema-reconciler] project=%s op=%s type=%s status=%s attempts=%d err=%s",
					op.ProjectID, op.ID, op.OperationType, op.Status, op.AttemptCount, op.Error)
			}
		}
	}
}
