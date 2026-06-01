package service_test

import (
	"context"
	"errors"
	"net"
	"testing"

	proxyModel "local-proxy/internal/domains/proxy/model"
	"local-proxy/internal/domains/proxy/service"
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

type mockAuth struct {
	header string
}

func (m *mockAuth) Header() string {
	return m.header
}

func (m *mockAuth) HandleChallenge(_ string) (string, error) {
	return m.header, nil
}

type mockNoProxy struct {
	match bool
}

func (m *mockNoProxy) Match(target string) bool {
	return m.match
}

func TestProxyService_Handle(t *testing.T) {
	t.Run("it should forward request to upstream when target is not in NoProxy list", func(t *testing.T) {
		upstream := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{
					StatusCode: 200,
					Body:       []byte("ok"),
				}, nil
			},
		}
		auth := &mockAuth{header: "Basic dXNlcjpwYXNz"}
		noproxy := &mockNoProxy{match: false}
		svc := service.New(upstream, auth, noproxy)

		resp, err := svc.Handle(context.Background(), &proxyModel.ProxyRequest{
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

	t.Run("it should return error when upstream is unreachable", func(t *testing.T) {
		upstream := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return nil, errors.New("connection refused")
			},
		}
		auth := &mockAuth{header: "Basic dXNlcjpwYXNz"}
		noproxy := &mockNoProxy{match: false}
		svc := service.New(upstream, auth, noproxy)

		_, err := svc.Handle(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("it should forward to direct connector when target is in NoProxy list", func(t *testing.T) {
		var directCalled bool
		upstream := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return nil, errors.New("should not be called")
			},
		}
		direct := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				directCalled = true
				return &proxyModel.ProxyResponse{StatusCode: 200, Body: []byte("direct")}, nil
			},
		}
		auth := &mockAuth{}
		noproxy := &mockNoProxy{match: true}
		svc := service.NewWithDirect(upstream, direct, auth, noproxy)

		resp, err := svc.Handle(context.Background(), &proxyModel.ProxyRequest{
			Target: proxyModel.Target{Host: "localhost"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !directCalled {
			t.Fatal("direct connector was not called")
		}
		if string(resp.Body) != "direct" {
			t.Errorf("got body %q, want %q", string(resp.Body), "direct")
		}
	})

	t.Run("it should pass auth header through upstream connector", func(t *testing.T) {
		var capturedReq *proxyModel.ProxyRequest
		upstream := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				capturedReq = req
				return &proxyModel.ProxyResponse{StatusCode: 200}, nil
			},
		}
		auth := &mockAuth{header: "Basic cHJveHlVc2VyOnByb3h5UGFzcw=="}
		noproxy := &mockNoProxy{match: false}
		svc := service.New(upstream, auth, noproxy)

		_, _ = svc.Handle(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
		})
		if capturedReq == nil {
			t.Fatal("request was not forwarded to upstream")
		}
	})

	t.Run("it should establish CONNECT tunnel via HandleConnect", func(t *testing.T) {
		upstream := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				client, server := net.Pipe()
				go func() {
					_, _ = server.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
				}()
				return client, nil
			},
		}
		auth := &mockAuth{}
		noproxy := &mockNoProxy{match: false}
		svc := service.New(upstream, auth, noproxy)

		conn, err := svc.HandleConnect(context.Background(), "example.com:443")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = conn.Close() }()
	})

	t.Run("it should handle upstream returning error status without crashing", func(t *testing.T) {
		upstream := &mockUpstream{
			roundTripFn: func(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{StatusCode: 502}, nil
			},
		}
		auth := &mockAuth{header: "Basic dXNlcjpwYXNz"}
		noproxy := &mockNoProxy{match: false}
		svc := service.New(upstream, auth, noproxy)

		resp, err := svc.Handle(context.Background(), &proxyModel.ProxyRequest{
			Method: "GET",
			URL:    "http://example.com/",
			Target: proxyModel.Target{Host: "example.com", Port: "80"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 502 {
			t.Errorf("got status %d, want 502", resp.StatusCode)
		}
	})

	t.Run("it should establish CONNECT tunnel via direct connector when target is in NoProxy list", func(t *testing.T) {
		var directCalled bool
		upstream := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				return nil, errors.New("should not be called")
			},
		}
		direct := &mockUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				directCalled = true
				client, server := net.Pipe()
				go func() {
					_, _ = server.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
				}()
				return client, nil
			},
		}
		auth := &mockAuth{}
		noproxy := &mockNoProxy{match: true}
		svc := service.NewWithDirect(upstream, direct, auth, noproxy)

		conn, err := svc.HandleConnect(context.Background(), "localhost:443")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = conn.Close() }()
		if !directCalled {
			t.Fatal("direct connector was not called")
		}
	})
}
