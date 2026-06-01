package tunnel_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"local-proxy/internal/domains/tunnel"
)

type mockUpstream struct {
	connectFn func(ctx context.Context, target string) (net.Conn, error)
}

func (m *mockUpstream) ConnectTunnel(ctx context.Context, target string) (net.Conn, error) {
	return m.connectFn(ctx, target)
}

func TestStart(t *testing.T) {
	t.Run("it should start a listener and tunnel connections", func(t *testing.T) {
		upstream := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				client, server := net.Pipe()
				go func() {
					_, _ = server.Write([]byte("hello from remote"))
					_ = server.Close()
				}()
				return client, nil
			},
		}

		ln, err := tunnel.Start(tunnel.Config{LocalPort: 0, RemoteHost: "example.com", RemotePort: 443}, upstream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer ln.Close()

		conn, err := net.DialTimeout("tcp", ln.Addr().String(), 5*time.Second)
		if err != nil {
			t.Fatalf("dial tunnel: %v", err)
		}
		defer func() { _ = conn.Close() }()

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if string(buf[:n]) != "hello from remote" {
			t.Errorf("got %q, want %q", string(buf[:n]), "hello from remote")
		}
	})

	t.Run("it should return error when upstream is unreachable", func(t *testing.T) {
		upstream := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				return nil, errors.New("connection refused")
			},
		}

		ln, err := tunnel.Start(tunnel.Config{LocalPort: 0, RemoteHost: "example.com", RemotePort: 443}, upstream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer ln.Close()

		conn, err := net.DialTimeout("tcp", ln.Addr().String(), 5*time.Second)
		if err != nil {
			t.Fatalf("dial tunnel: %v", err)
		}
		defer func() { _ = conn.Close() }()

		buf := make([]byte, 1024)
		_, err = conn.Read(buf)
		if err == nil {
			t.Error("expected connection to close after upstream failure")
		}
	})
}

func TestStartAll(t *testing.T) {
	t.Run("it should start multiple tunnels", func(t *testing.T) {
		upstream := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				client, server := net.Pipe()
				go func() {
					_, _ = server.Write([]byte("ok"))
					_ = server.Close()
				}()
				return client, nil
			},
		}

		listeners, err := tunnel.StartAll([]tunnel.Config{
			{LocalPort: 0, RemoteHost: "a.com", RemotePort: 443},
			{LocalPort: 0, RemoteHost: "b.com", RemotePort: 443},
		}, upstream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(listeners) != 2 {
			t.Fatalf("got %d listeners, want 2", len(listeners))
		}
		for _, l := range listeners {
			l.Close()
		}
	})
}
