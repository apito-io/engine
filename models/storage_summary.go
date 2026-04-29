package models

import (
	"fmt"
	"strings"
)

// StorageEnginesSummary returns one line listing configured storage engines (no hosts, secrets, or DSNs).
// Use in production logs to verify SYSTEM_DB_ENGINE, CACHE_DB, KV_ENGINE, and QUEUE_ENGINE.
func (c *Config) StorageEnginesSummary() string {
	if c == nil {
		return "[storage] engines: (nil config)"
	}
	return fmt.Sprintf(
		"[storage] engines: system=%s cache=%s kv=%s queue=%s",
		strings.TrimSpace(c.SystemDatabaseEngine),
		strings.TrimSpace(c.CacheEngine),
		strings.TrimSpace(c.KVStorageEngine),
		strings.TrimSpace(c.QueueStorageEngine),
	)
}
