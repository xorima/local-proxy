package controller_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	proxyModel "local-proxy/internal/domains/proxy/model"
	"local-proxy/internal/domains/proxy/controller"
	"local-proxy/internal/domains/proxy/ports"
)

var _ ports.ProxyHandler = (*mockHandler)(nil)

type mockHandler struct {
	handleFn       func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error)
	handleConnectFn func(ctx context.Context, target string) (net.Conn, error)
}

func (m *mockHandler) Handle(_ context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
	return m.handleFn(req)
}

func (m *mockHandler) HandleConnect(ctx context.Context, target string) (net.Conn, error) {
	if m.handleConnectFn != nil {
		return m.handleConnectFn(ctx, target)
	}
	return nil, errors.New("connect not implemented")
}

func TestHTTPController_ServeHTTP(t *testing.T) {
	t.Run("it should return 200 for successful proxy request", func(t *testing.T) {
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{
					StatusCode: http.StatusOK,
					Headers:    map[string][]string{"Content-Type": {"text/plain"}},
					Body:       []byte("hello"),
				}, nil
			},
		}
		ctrl := controller.NewHTTP(handler)

		req := httptest.NewRequest("GET", "http://example.com/", nil)
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})

	t.Run("it should return 502 when handler returns error", func(t *testing.T) {
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return nil, errors.New("upstream unreachable")
			},
		}
		ctrl := controller.NewHTTP(handler)

		req := httptest.NewRequest("GET", "http://example.com/", nil)
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadGateway {
			t.Errorf("got status %d, want 502", resp.StatusCode)
		}
	})

	t.Run("it should return upstream status code when upstream returns error status", func(t *testing.T) {
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{StatusCode: http.StatusNotFound}, nil
			},
		}
		ctrl := controller.NewHTTP(handler)

		req := httptest.NewRequest("GET", "http://example.com/notfound", nil)
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want 404", resp.StatusCode)
		}
	})

	t.Run("it should forward response headers to client", func(t *testing.T) {
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{
					StatusCode: http.StatusOK,
					Headers:    map[string][]string{"X-Custom": {"value1"}},
					Body:       []byte("ok"),
				}, nil
			},
		}
		ctrl := controller.NewHTTP(handler)

		req := httptest.NewRequest("GET", "http://example.com/", nil)
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		resp := w.Result()
		if resp.Header.Get("X-Custom") != "value1" {
			t.Errorf("got X-Custom %q, want %q", resp.Header.Get("X-Custom"), "value1")
		}
	})

	t.Run("it should strip Proxy-Authorization and Proxy-Connection from forwarded headers", func(t *testing.T) {
		var captured *proxyModel.ProxyRequest
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				captured = req
				return &proxyModel.ProxyResponse{StatusCode: http.StatusOK}, nil
			},
		}
		ctrl := controller.NewHTTP(handler)

		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.Header.Set("Proxy-Authorization", "Basic abc")
		req.Header.Set("Proxy-Connection", "keep-alive")
		req.Header.Set("User-Agent", "test")
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		if captured == nil {
			t.Fatal("handler was not called")
		}
		if _, ok := captured.Headers["Proxy-Authorization"]; ok {
			t.Error("Proxy-Authorization should be stripped")
		}
		if _, ok := captured.Headers["Proxy-Connection"]; ok {
			t.Error("Proxy-Connection should be stripped")
		}
		if captured.Headers["User-Agent"][0] != "test" {
			t.Errorf("User-Agent should be preserved, got %v", captured.Headers["User-Agent"])
		}
	})

	t.Run("it should parse target host and port correctly", func(t *testing.T) {
		var captured *proxyModel.ProxyRequest
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				captured = req
				return &proxyModel.ProxyResponse{StatusCode: http.StatusOK}, nil
			},
		}
		ctrl := controller.NewHTTP(handler)

		req := httptest.NewRequest("GET", "http://example.com:8080/path?q=1", nil)
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		if captured.Target.Host != "example.com" {
			t.Errorf("got host %q, want %q", captured.Target.Host, "example.com")
		}
		if captured.Target.Port != "8080" {
			t.Errorf("got port %q, want %q", captured.Target.Port, "8080")
		}
		if captured.URL != "http://example.com:8080/path?q=1" {
			t.Errorf("got URL %q, want %q", captured.URL, "http://example.com:8080/path?q=1")
		}
	})

	t.Run("it should return 200 for health endpoint", func(t *testing.T) {
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return nil, errors.New("should not be called")
			},
		}
		ctrl := controller.NewHTTP(handler)

		req := httptest.NewRequest("GET", "http://example.com/__health", nil)
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})

	t.Run("it should return 200 for metrics endpoint", func(t *testing.T) {
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return nil, errors.New("should not be called")
			},
		}
		ctrl := controller.NewHTTP(handler)

		req := httptest.NewRequest("GET", "http://example.com/metrics", nil)
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})
}

type mockACL struct {
	allow bool
}

func (m *mockACL) Allow(_ string) bool {
	return m.allow
}

func TestHTTPController_ACL(t *testing.T) {
	t.Run("it should return 403 when ACL denies the client", func(t *testing.T) {
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{StatusCode: http.StatusOK}, nil
			},
		}
		ctrl := controller.NewHTTPWithACL(handler, &mockACL{allow: false})

		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("got status %d, want 403", resp.StatusCode)
		}
	})

	t.Run("it should allow when ACL permits the client", func(t *testing.T) {
		handler := &mockHandler{
			handleFn: func(req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
				return &proxyModel.ProxyResponse{StatusCode: http.StatusOK}, nil
			},
		}
		ctrl := controller.NewHTTPWithACL(handler, &mockACL{allow: true})

		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		w := httptest.NewRecorder()
		ctrl.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})
}
