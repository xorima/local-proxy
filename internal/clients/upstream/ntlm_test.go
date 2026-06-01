package upstream_test

import (
	"context"
	"testing"

	authAdapters "local-proxy/internal/domains/auth/adapters"
	authModel "local-proxy/internal/domains/auth/model"
	"local-proxy/internal/clients/ntlmmock"
	"local-proxy/internal/clients/upstream"
	proxyModel "local-proxy/internal/domains/proxy/model"
)

func TestClient_NTLMHandshake(t *testing.T) {
	t.Run("it should complete NTLMv1 handshake for HTTP proxy request", func(t *testing.T) {
		ms := ntlmmock.New("password", "user", "domain", false)
		if err := ms.Start(); err != nil {
			t.Fatalf("start mock: %v", err)
		}
		defer ms.Close()

		auth := &staticAuth{provider: authAdapters.NewNTLMProvider("user", "password", "domain", authModel.NTLMModeV1)}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: ms.Port}, auth)

		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})

	t.Run("it should complete NTLMv2 handshake for HTTP proxy request", func(t *testing.T) {
		ms := ntlmmock.New("password", "user", "domain", true)
		if err := ms.Start(); err != nil {
			t.Fatalf("start mock: %v", err)
		}
		defer ms.Close()

		auth := &staticAuth{provider: authAdapters.NewNTLMProvider("user", "password", "domain", authModel.NTLMModeV2)}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: ms.Port}, auth)

		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})

	t.Run("it should complete NTLM Session handshake for HTTP proxy request", func(t *testing.T) {
		ms := ntlmmock.New("password", "user", "domain", false)
		if err := ms.Start(); err != nil {
			t.Fatalf("start mock: %v", err)
		}
		defer ms.Close()

		auth := &staticAuth{provider: authAdapters.NewNTLMProvider("user", "password", "domain", authModel.NTLMModeSession)}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: ms.Port}, auth)

		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})

	t.Run("it should complete NTLM handshake for CONNECT tunnel", func(t *testing.T) {
		ms := ntlmmock.New("password", "user", "domain", false)
		if err := ms.Start(); err != nil {
			t.Fatalf("start mock: %v", err)
		}
		defer ms.Close()

		auth := &staticAuth{provider: authAdapters.NewNTLMProvider("user", "password", "domain", authModel.NTLMModeV1)}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: ms.Port}, auth)

		conn, err := client.ConnectTunnel(context.Background(), "example.com:443")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = conn.Close()
	})

	t.Run("it should return 407 on wrong password for NTLM", func(t *testing.T) {
		ms := ntlmmock.New("correct-password", "user", "domain", false)
		if err := ms.Start(); err != nil {
			t.Fatalf("start mock: %v", err)
		}
		defer ms.Close()

		auth := &staticAuth{provider: authAdapters.NewNTLMProvider("user", "wrong-password", "domain", authModel.NTLMModeV1)}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: ms.Port}, auth)

		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 407 {
			t.Errorf("got status %d, want 407", resp.StatusCode)
		}
	})
}
