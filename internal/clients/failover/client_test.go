package failover_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	proxyModel "local-proxy/internal/domains/proxy/model"
	"local-proxy/internal/clients/failover"
)

type mockUpstream struct {
	roundTripFn func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error)
	connectFn   func(ctx context.Context, target string) (net.Conn, error)
}

func (m *mockUpstream) RoundTrip(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
	return m.roundTripFn(ctx, req)
}

func (m *mockUpstream) ConnectTunnel(ctx context.Context, target string) (net.Conn, error) {
	return m.connectFn(ctx, target)
}

func TestFailover_RoundTrip(t *testing.T) {
	t.Run("it should try first proxy and succeed", func(t *testing.T) {
		p1 := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{StatusCode: 200, Body: []byte("ok")}, nil
			},
		}
		client := failover.New([]failover.UpstreamClient{p1}, []string{"proxy1"}, 30*time.Second, nil)

		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})

	t.Run("it should fail over to second proxy when first fails", func(t *testing.T) {
		var tried []int
		p1 := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				tried = append(tried, 1)
				return nil, errors.New("connection refused")
			},
		}
		p2 := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				tried = append(tried, 2)
				return &proxyModel.ProxyResponse{StatusCode: 200}, nil
			},
		}
		client := failover.New([]failover.UpstreamClient{p1, p2}, []string{"p1", "p2"}, 30*time.Second, nil)
		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		if len(tried) != 2 || tried[0] != 1 || tried[1] != 2 {
			t.Errorf("expected tries [1,2], got %v", tried)
		}
	})

	t.Run("it should return error when all proxies fail", func(t *testing.T) {
		p1 := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return nil, errors.New("connection refused")
			},
		}
		client := failover.New([]failover.UpstreamClient{p1}, []string{"p1"}, 30*time.Second, nil)
		_, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("it should fall back to direct connection when standalone is set", func(t *testing.T) {
		p1 := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return nil, errors.New("connection refused")
			},
		}
		direct := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{StatusCode: 200, Body: []byte("direct")}, nil
			},
		}
		client := failover.New([]failover.UpstreamClient{p1}, []string{"p1"}, 30*time.Second, direct)
		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(resp.Body) != "direct" {
			t.Errorf("got body %q, want %q", string(resp.Body), "direct")
		}
	})

	t.Run("it should skip recently failed proxies", func(t *testing.T) {
		var tried []int
		p1 := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				tried = append(tried, 1)
				return nil, errors.New("fail")
			},
		}
		p2 := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				tried = append(tried, 2)
				return &proxyModel.ProxyResponse{StatusCode: 200}, nil
			},
		}
		client := failover.New([]failover.UpstreamClient{p1, p2}, []string{"p1", "p2"}, 1*time.Hour, nil)

		// First call fails p1, succeeds on p2
		_, _ = client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{})

		// Second call should skip p1 (still in cooldown) and use p2 directly
		tried = nil
		resp, err := client.RoundTrip(context.Background(), &proxyModel.ProxyRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})
}

func TestFailover_ConnectTunnel(t *testing.T) {
	t.Run("it should establish tunnel through first available proxy", func(t *testing.T) {
		p1 := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				client, server := net.Pipe()
				go func() { _, _ = server.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); _ = server.Close() }()
				return client, nil
			},
		}
		client := failover.New([]failover.UpstreamClient{p1}, []string{"p1"}, 30*time.Second, nil)
		conn, err := client.ConnectTunnel(context.Background(), "example.com:443")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = conn.Close()
	})

	t.Run("it should fall back to direct tunnel when standalone is set", func(t *testing.T) {
		p1 := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				return nil, errors.New("connection refused")
			},
		}
		direct := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				client, server := net.Pipe()
				go func() { _, _ = server.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); _ = server.Close() }()
				return client, nil
			},
		}
		client := failover.New([]failover.UpstreamClient{p1}, []string{"p1"}, 30*time.Second, direct)
		conn, err := client.ConnectTunnel(context.Background(), "example.com:443")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = conn.Close()
	})
}


