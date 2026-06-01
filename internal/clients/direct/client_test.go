package direct_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	proxyModel "local-proxy/internal/domains/proxy/model"
	"local-proxy/internal/clients/direct"
)

func TestDirect_RoundTrip(t *testing.T) {
	t.Run("it should forward GET request and return response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("hello"))
		}))
		defer ts.Close()

		client := direct.New()
		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    ts.URL + "/test",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		if string(resp.Body) != "hello" {
			t.Errorf("got body %q, want %q", string(resp.Body), "hello")
		}
	})

	t.Run("it should forward request headers", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Custom") != "value" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := direct.New()
		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    ts.URL,
			Headers: map[string][]string{
				"X-Custom": {"value"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})

	t.Run("it should return error for unreachable server", func(t *testing.T) {
		client := direct.New()
		_, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://127.0.0.1:1",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestDirect_ConnectTunnel(t *testing.T) {
	t.Run("it should establish TCP connection to target", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer func() { _ = ln.Close() }()

		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}()

		client := direct.New()
		conn, err := client.ConnectTunnel(context.Background(), ln.Addr().String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = conn.Close()
	})

	t.Run("it should return error for unreachable target", func(t *testing.T) {
		client := direct.New()
		_, err := client.ConnectTunnel(context.Background(), "127.0.0.1:1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
