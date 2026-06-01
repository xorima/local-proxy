package adapters_test

import (
	"testing"

	"local-proxy/internal/domains/proxy/adapters"
)

func TestNoProxyMatcher_Match(t *testing.T) {
	t.Run("it should match exact hostname", func(t *testing.T) {
		m := adapters.NewNoProxyMatcher([]string{"localhost"})
		if !m.Match("localhost") {
			t.Error("expected match for localhost")
		}
	})

	t.Run("it should not match different hostname", func(t *testing.T) {
		m := adapters.NewNoProxyMatcher([]string{"localhost"})
		if m.Match("example.com") {
			t.Error("expected no match for example.com")
		}
	})

	t.Run("it should match wildcard patterns", func(t *testing.T) {
		m := adapters.NewNoProxyMatcher([]string{"*.local"})
		if !m.Match("test.local") {
			t.Error("expected match for test.local with *.local pattern")
		}
	})

	t.Run("it should match IP prefix pattern", func(t *testing.T) {
		m := adapters.NewNoProxyMatcher([]string{"192.168.*"})
		if !m.Match("192.168.1.100") {
			t.Error("expected match for 192.168.1.100 with 192.168.* pattern")
		}
	})

	t.Run("it should not match different IP prefix", func(t *testing.T) {
		m := adapters.NewNoProxyMatcher([]string{"192.168.*"})
		if m.Match("10.0.0.1") {
			t.Error("expected no match for 10.0.0.1 with 192.168.* pattern")
		}
	})

	t.Run("it should match when target contains pattern string", func(t *testing.T) {
		m := adapters.NewNoProxyMatcher([]string{"internal"})
		if !m.Match("server.internal.company.com") {
			t.Error("expected match for server.internal.company.com when pattern contains 'internal'")
		}
	})

	t.Run("it should handle empty patterns list", func(t *testing.T) {
		m := adapters.NewNoProxyMatcher([]string{})
		if m.Match("anything") {
			t.Error("expected no match for empty patterns")
		}
	})

	t.Run("it should handle multiple patterns and match first", func(t *testing.T) {
		m := adapters.NewNoProxyMatcher([]string{"localhost", "127.0.0.*", "*.local"})
		if !m.Match("127.0.0.1") {
			t.Error("expected match for 127.0.0.1")
		}
	})
}
