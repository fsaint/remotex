package config_test

import (
	"path/filepath"
	"testing"

	"github.com/fsaint/remotex/internal/config"
)

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// No config written — Load should return an error
	if _, err := config.Load(); err == nil {
		t.Error("Load should return an error when config file does not exist")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &config.Config{
		APIKey:        "test-api-key",
		TailscaleHost: "myhost.tailnet.ts.net",
		DaemonPort:    7654,
		SSHKeyPath:    filepath.Join(dir, ".remotex", "id_ed25519"),
	}

	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.APIKey != cfg.APIKey {
		t.Errorf("APIKey: got %q want %q", loaded.APIKey, cfg.APIKey)
	}
	if loaded.TailscaleHost != cfg.TailscaleHost {
		t.Errorf("TailscaleHost: got %q want %q", loaded.TailscaleHost, cfg.TailscaleHost)
	}
	if loaded.DaemonPort != cfg.DaemonPort {
		t.Errorf("DaemonPort: got %d want %d", loaded.DaemonPort, cfg.DaemonPort)
	}
}
