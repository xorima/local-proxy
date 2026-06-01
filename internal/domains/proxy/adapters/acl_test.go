package adapters_test

import (
	"testing"

	"local-proxy/internal/domains/proxy/adapters"
)

func TestACLMatcher_Allow(t *testing.T) {
	t.Run("it should allow all when no rules configured", func(t *testing.T) {
		acl, err := adapters.NewACLMatcher(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !acl.Allow("192.168.1.1") {
			t.Error("expected all IPs to be allowed")
		}
	})

	t.Run("it should allow IPs in allow list", func(t *testing.T) {
		acl, err := adapters.NewACLMatcher([]string{"10.0.0.0/8"}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !acl.Allow("10.0.0.1") {
			t.Error("expected 10.0.0.1 to be allowed")
		}
		if acl.Allow("192.168.1.1") {
			t.Error("expected 192.168.1.1 to be denied")
		}
	})

	t.Run("it should deny IPs in deny list", func(t *testing.T) {
		acl, err := adapters.NewACLMatcher(nil, []string{"10.0.0.0/8"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if acl.Allow("10.0.0.1") {
			t.Error("expected 10.0.0.1 to be denied")
		}
		if !acl.Allow("192.168.1.1") {
			t.Error("expected 192.168.1.1 to be allowed")
		}
	})

	t.Run("it should deny take precedence over allow", func(t *testing.T) {
		acl, err := adapters.NewACLMatcher([]string{"0.0.0.0/0"}, []string{"10.0.0.0/8"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if acl.Allow("10.0.0.1") {
			t.Error("expected 10.0.0.1 to be denied by deny rule")
		}
		if !acl.Allow("192.168.1.1") {
			t.Error("expected 192.168.1.1 to be allowed")
		}
	})

	t.Run("it should return false for invalid IP", func(t *testing.T) {
		acl, err := adapters.NewACLMatcher(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if acl.Allow("not-an-ip") {
			t.Error("expected false for invalid IP")
		}
	})

	t.Run("it should error on invalid CIDR", func(t *testing.T) {
		_, err := adapters.NewACLMatcher([]string{"not-a-cidr"}, nil)
		if err == nil {
			t.Fatal("expected error for invalid CIDR")
		}
	})
}
