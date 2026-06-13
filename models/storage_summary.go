package models

import (
	"fmt"
	"strings"
)

// StorageEnginesSummary returns one line listing configured storage engines (no hosts, secrets, or DSNs).
func (c *Config) StorageEnginesSummary() string {
	if c == nil {
		return "[storage] engines: (nil config)"
	}
	realtime := strings.TrimSpace(c.RealtimeEngine)
	if c.RealtimeNatsJetStream {
		realtime += "+jetstream"
	}
	return fmt.Sprintf(
		"[storage] engines: system=%s cache=%s kv=%s realtime=%s",
		strings.TrimSpace(c.SystemDatabaseEngine),
		strings.TrimSpace(c.CacheEngine),
		strings.TrimSpace(c.KVStorageEngine),
		realtime,
	)
}
