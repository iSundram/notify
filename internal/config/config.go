package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	DBPath     string `yaml:"db_path"`
	SocketPath string `yaml:"socket_path"`
	HTTPAddr   string `yaml:"http_addr"`
	LogDir     string `yaml:"log_dir"`
	CacheFile  string `yaml:"cache_file"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DBPath:     "/var/lib/notify/notify.db",
		SocketPath: "/run/notify/notify.sock",
		HTTPAddr:   ":8008",
		LogDir:     "/var/log/notify",
		CacheFile:  "/run/notify/unread_count",
	}
}

// Load reads a YAML config file and merges it with defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
