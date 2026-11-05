package models

import (
	"log"
	"os"
	"strings"
)

// ApplyPluginUploadPolicy sets unsigned plugin upload when PLUGIN_ALLOW_LOCAL_UPLOAD
// is absent. Hosted Apito stays closed. A self-hosted or enterprise engine stays open.
func ApplyPluginUploadPolicy(cfg *Config) {
	if cfg == nil {
		return
	}
	raw, set := os.LookupEnv("PLUGIN_ALLOW_LOCAL_UPLOAD")
	raw = strings.TrimSpace(raw)
	if !set || raw == "" {
		cfg.PluginAllowLocalUpload = !apitoCloudEnv()
	}
	if cfg.PluginAllowLocalUpload {
		return
	}
	log.Printf("PLUGIN_ALLOW_LOCAL_UPLOAD is false. Unsigned plugin upload is disabled. Hosted Apito keeps this off so a CLI user cannot push a system plugin. A self-hosted or enterprise engine allows it unless the operator sets this false.")
}

func apitoCloudEnv() bool {
	v := strings.TrimSpace(os.Getenv("APITO_CLOUD"))
	return v == "1" || strings.EqualFold(v, "true")
}
