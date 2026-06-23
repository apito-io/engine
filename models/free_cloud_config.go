package models

const defaultFreeCloudStorageLimitGB = 0.5

// FreeCloudStorageLimitBytes returns the per-project free-cloud quota in bytes.
func FreeCloudStorageLimitBytes(cfg *Config) int64 {
	limitGB := defaultFreeCloudStorageLimitGB
	if cfg != nil && cfg.FreeCloudStorageLimitGB > 0 {
		limitGB = cfg.FreeCloudStorageLimitGB
	}
	return int64(limitGB * 1024 * 1024 * 1024)
}
