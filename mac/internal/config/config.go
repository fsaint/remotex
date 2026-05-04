package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey        string `json:"api_key"`
	TailscaleHost string `json:"tailscale_host"`
	DaemonPort    int    `json:"daemon_port"`
	SSHKeyPath    string `json:"ssh_key_path"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".remotex")
}

func configPath() string {
	return filepath.Join(Dir(), "config.json")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	return &cfg, json.Unmarshal(data, &cfg)
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}
