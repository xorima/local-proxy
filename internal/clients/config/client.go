package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Default() *Config {
	return &Config{
		Upstream: UpstreamConfig{
			Host: "localhost",
			Port: 13128,
		},
		Failover: FailoverConfig{
			RetryInterval: "30s",
		},
		Listen: "127.0.0.1:3128",
		Cache: CacheConfig{
			MaxConnections: 20,
			IdleTimeout:    "5m",
		},
		LogLevel: "info",
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}
