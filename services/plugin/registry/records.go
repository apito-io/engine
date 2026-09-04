package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func recordPath(pluginPath, id string) string {
	return filepath.Join(pluginPath, ".installs", id+".json")
}

func writeInstallRecord(pluginPath string, rec InstallRecord) error {
	dir := filepath.Join(pluginPath, ".installs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if rec.InstalledAt.IsZero() {
		rec.InstalledAt = time.Now().UTC()
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(recordPath(pluginPath, rec.ID), b, 0o644)
}

// ReadInstallRecord loads the persisted install record, if any.
func ReadInstallRecord(pluginPath, id string) (*InstallRecord, error) {
	b, err := os.ReadFile(recordPath(pluginPath, id))
	if err != nil {
		return nil, err
	}
	var rec InstallRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}
