package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"local-proxy/internal/clients/config"
)

func TestLoad(t *testing.T) {
	t.Run("it should return defaults when config file does not exist", func(t *testing.T) {
		cfg, err := config.Load("/nonexistent/path.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Listen != "127.0.0.1:3128" {
			t.Errorf("got Listen %q, want %q", cfg.Listen, "127.0.0.1:3128")
		}
		if cfg.Upstream.Host != "localhost" {
			t.Errorf("got Upstream.Host %q, want %q", cfg.Upstream.Host, "localhost")
		}
		if cfg.Upstream.Port != 13128 {
			t.Errorf("got Upstream.Port %d, want %d", cfg.Upstream.Port, 13128)
		}
	})

	t.Run("it should parse valid config file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := []byte(`
upstream:
  host: proxy.example.com
  port: 8080
auth:
  username: myuser
  password: mypass
  auth_type: basic
listen: "0.0.0.0:3129"
no_proxy:
  - localhost
  - 10.*
`)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Upstream.Host != "proxy.example.com" {
			t.Errorf("got Upstream.Host %q, want %q", cfg.Upstream.Host, "proxy.example.com")
		}
		if cfg.Upstream.Port != 8080 {
			t.Errorf("got Upstream.Port %d, want %d", cfg.Upstream.Port, 8080)
		}
		if cfg.Auth.Username != "myuser" {
			t.Errorf("got Auth.Username %q, want %q", cfg.Auth.Username, "myuser")
		}
		if cfg.Listen != "0.0.0.0:3129" {
			t.Errorf("got Listen %q, want %q", cfg.Listen, "0.0.0.0:3129")
		}
		if len(cfg.NoProxy) != 2 {
			t.Errorf("got %d NoProxy entries, want 2", len(cfg.NoProxy))
		}
	})

	t.Run("it should return error on malformed YAML", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.yaml")
		content := []byte(`upstream: [invalid: yaml: broken`)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		_, err := config.Load(path)
		if err == nil {
			t.Fatal("expected error for malformed YAML, got nil")
		}
	})

	t.Run("it should return error when file cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "unreadable.yaml")
		if err := os.WriteFile(path, []byte("key: value"), 0000); err != nil {
			t.Fatal(err)
		}

		_, err := config.Load(path)
		if err == nil {
			t.Fatal("expected error for unreadable file, got nil")
		}
	})

	t.Run("it should parse multiple upstreams", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "multi.yaml")
		content := []byte(`
upstreams:
  - host: proxy1.example.com
    port: 8080
  - host: proxy2.example.com
    port: 8081
failover:
  retry_interval: 15s
  standalone: true
`)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Upstreams) != 2 {
			t.Fatalf("got %d upstreams, want 2", len(cfg.Upstreams))
		}
		if cfg.Upstreams[0].Host != "proxy1.example.com" {
			t.Errorf("got Upstreams[0].Host %q, want %q", cfg.Upstreams[0].Host, "proxy1.example.com")
		}
		if cfg.Upstreams[1].Port != 8081 {
			t.Errorf("got Upstreams[1].Port %d, want %d", cfg.Upstreams[1].Port, 8081)
		}
		if cfg.Failover.RetryInterval != "15s" {
			t.Errorf("got Failover.RetryInterval %q, want %q", cfg.Failover.RetryInterval, "15s")
		}
		if !cfg.Failover.Standalone {
			t.Error("expected Failover.Standalone to be true")
		}
	})

	t.Run("it should preserve defaults for missing fields in config file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "partial.yaml")
		content := []byte(`listen: "127.0.0.1:9999"`)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Listen != "127.0.0.1:9999" {
			t.Errorf("got Listen %q, want %q", cfg.Listen, "127.0.0.1:9999")
		}
		if cfg.Upstream.Port != 13128 {
			t.Errorf("expected default upstream port, got %d", cfg.Upstream.Port)
		}
	})
}
