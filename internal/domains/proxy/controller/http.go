package controller

import (
	"io"
	"net"
	"net/http"
	"strings"

	proxyModel "local-proxy/internal/domains/proxy/model"
	"local-proxy/internal/domains/proxy/ports"
)

type HTTPController struct {
	handler ports.ProxyHandler
	acl     ports.ACLMatcher
}

func NewHTTP(handler ports.ProxyHandler) *HTTPController {
	return &HTTPController{handler: handler, acl: nil}
}

func NewHTTPWithACL(handler ports.ProxyHandler, acl ports.ACLMatcher) *HTTPController {
	return &HTTPController{handler: handler, acl: acl}
}

func (c *HTTPController) clientAllowed(r *http.Request) bool {
	if c.acl == nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	return c.acl.Allow(host)
}

func (c *HTTPController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !c.clientAllowed(r) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	if r.URL.Path == "/__health" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	if r.URL.Path == "/metrics" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# local-proxy metrics\n# (placeholder)\n"))
		return
	}

	if r.Method == http.MethodConnect {
		c.handleConnect(w, r)
		return
	}
	c.handleProxy(w, r)
}

func (c *HTTPController) handleProxy(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = r.Body.Close() }()

	target := extractTarget(r)
	headers := make(map[string][]string)
	for k, vs := range r.Header {
		if strings.EqualFold(k, "Proxy-Connection") || strings.EqualFold(k, "Proxy-Authorization") {
			continue
		}
		headers[k] = vs
	}

	req := &proxyModel.ProxyRequest{
		Method:  r.Method,
		URL:     r.URL.String(),
		Target:  target,
		Headers: headers,
		Body:    body,
	}

	resp, err := c.handler.Handle(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	for k, vs := range resp.Headers {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

func (c *HTTPController) handleConnect(w http.ResponseWriter, r *http.Request) {
	target := r.Host
	if target == "" {
		http.Error(w, "missing target host", http.StatusBadRequest)
		return
	}

	upstreamConn, err := c.handler.HandleConnect(r.Context(), target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = upstreamConn.Close() }()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = clientConn.Close() }()

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(upstreamConn, clientConn)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(clientConn, upstreamConn)
		errc <- err
	}()
	<-errc
}

func extractTarget(r *http.Request) proxyModel.Target {
	t := proxyModel.Target{
		Host: r.URL.Hostname(),
		Port: r.URL.Port(),
	}
	if t.Port == "" {
		if r.URL.Scheme == "https" {
			t.Port = "443"
		} else {
			t.Port = "80"
		}
	}
	return t
}
