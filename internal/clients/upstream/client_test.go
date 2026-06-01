package upstream_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	proxyModel "local-proxy/internal/domains/proxy/model"
	"local-proxy/internal/clients/upstream"
)

type staticAuth struct {
	header   string
	provider interface {
		Header() string
		HandleChallenge(string) (string, error)
	}
}

func (a *staticAuth) Header() string {
	if a.provider != nil {
		return a.provider.Header()
	}
	return a.header
}

func (a *staticAuth) HandleChallenge(challenge string) (string, error) {
	if a.provider != nil {
		return a.provider.HandleChallenge(challenge)
	}
	return a.header, nil
}

func TestClient_RoundTrip(t *testing.T) {
	t.Run("it should forward request to upstream proxy and return response", func(t *testing.T) {
		var gotMethod, gotURL string
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotURL = r.URL.String()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("hello from upstream"))
		}))
		defer backend.Close()

		auth := &staticAuth{}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: portFromURL(backend.URL)}, auth)

		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
			Body:   nil,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		if string(resp.Body) != "hello from upstream" {
			t.Errorf("got body %q, want %q", string(resp.Body), "hello from upstream")
		}
		if gotMethod != "GET" {
			t.Errorf("got method %s, want GET", gotMethod)
		}
		if gotURL != "http://example.com/" {
			t.Errorf("got URL %s, want http://example.com/", gotURL)
		}
	})

	t.Run("it should forward request body to upstream", func(t *testing.T) {
		var gotBody string
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			gotBody = string(buf[:n])
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()

		auth := &staticAuth{}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: portFromURL(backend.URL)}, auth)

		_, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "POST",
			URL:    "http://example.com/submit",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
			Body:   []byte("request body data"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotBody != "request body data" {
			t.Errorf("got body %q, want %q", gotBody, "request body data")
		}
	})

	t.Run("it should return error when upstream is unreachable", func(t *testing.T) {
		auth := &staticAuth{}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: 1}, auth)
		_, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
		})
		if err == nil {
			t.Fatal("expected error for unreachable upstream, got nil")
		}
	})

	t.Run("it should forward request headers", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Custom") != "test-value" {
				t.Errorf("got X-Custom %q, want %q", r.Header.Get("X-Custom"), "test-value")
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()

		auth := &staticAuth{}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: portFromURL(backend.URL)}, auth)

		_, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method:  "GET",
			URL:     "http://example.com/",
			Headers: map[string][]string{"X-Custom": {"test-value"}},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("it should include auth header when provided", func(t *testing.T) {
		var gotAuth string
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Proxy-Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()

		auth := &staticAuth{header: "Basic dXNlcjpwYXNz"}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: portFromURL(backend.URL)}, auth)

		_, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAuth != "Basic dXNlcjpwYXNz" {
			t.Errorf("got auth %q, want %q", gotAuth, "Basic dXNlcjpwYXNz")
		}
	})
}

func TestClient_ConnectTunnel(t *testing.T) {
	t.Run("it should establish CONNECT tunnel through upstream", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "CONNECT" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()

		auth := &staticAuth{}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: portFromURL(backend.URL)}, auth)

		conn, err := client.ConnectTunnel(context.Background(), "target.example.com:443")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = conn.Close() }()
	})

	t.Run("it should return error when upstream rejects CONNECT", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "CONNECT" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()

		auth := &staticAuth{}
		client := upstream.New(upstream.Config{Host: "127.0.0.1", Port: portFromURL(backend.URL)}, auth)

		_, err := client.ConnectTunnel(context.Background(), "target.example.com:443")
		if err == nil {
			t.Fatal("expected error for rejected CONNECT, got nil")
		}
	})
}

func portFromURL(rawURL string) int {
	var port int
	if _, err := fmt.Sscanf(rawURL, "http://127.0.0.1:%d", &port); err != nil {
		panic(fmt.Sprintf("cannot parse port from %s: %v", rawURL, err))
	}
	return port
}
