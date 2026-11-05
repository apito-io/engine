package models

import (
	"testing"
)

func TestApplyPluginUploadPolicy(t *testing.T) {
	t.Run("hosted default is closed", func(t *testing.T) {
		t.Setenv("APITO_CLOUD", "true")
		t.Setenv("PLUGIN_ALLOW_LOCAL_UPLOAD", "")
		cfg := &Config{PluginAllowLocalUpload: true}
		ApplyPluginUploadPolicy(cfg)
		if cfg.PluginAllowLocalUpload {
			t.Fatal("hosted Apito must keep unsigned upload off when the variable is empty")
		}
	})

	t.Run("self-hosted default is open", func(t *testing.T) {
		t.Setenv("APITO_CLOUD", "")
		t.Setenv("PLUGIN_ALLOW_LOCAL_UPLOAD", "")
		cfg := &Config{}
		ApplyPluginUploadPolicy(cfg)
		if !cfg.PluginAllowLocalUpload {
			t.Fatal("self-hosted engine must allow unsigned upload when the variable is unset")
		}
	})

	t.Run("explicit false stays closed", func(t *testing.T) {
		t.Setenv("APITO_CLOUD", "")
		t.Setenv("PLUGIN_ALLOW_LOCAL_UPLOAD", "false")
		cfg := &Config{}
		ApplyPluginUploadPolicy(cfg)
		if cfg.PluginAllowLocalUpload {
			t.Fatal("explicit false must stay off")
		}
	})

	t.Run("explicit true stays open on hosted", func(t *testing.T) {
		t.Setenv("APITO_CLOUD", "true")
		t.Setenv("PLUGIN_ALLOW_LOCAL_UPLOAD", "true")
		cfg := &Config{PluginAllowLocalUpload: true}
		ApplyPluginUploadPolicy(cfg)
		if !cfg.PluginAllowLocalUpload {
			t.Fatal("explicit true must stay on")
		}
	})
}
