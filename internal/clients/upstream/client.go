package upstream

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	proxyModel "local-proxy/internal/domains/proxy/model"
	"local-proxy/internal/domains/proxy/ports"
)

type Client struct {
	proxyURL   *url.URL
	httpClient *http.Client
	auth       ports.AuthProvider
}

func New(cfg Config, auth ports.AuthProvider) *Client {
	proxyURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
	}

	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 100
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 90 * time.Second
	}

	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxyURL),
		MaxIdleConns:        cfg.MaxConns,
		IdleConnTimeout:     cfg.IdleTimeout,
		DisableCompression:  true,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &Client{
		proxyURL:  proxyURL,
		auth:      auth,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) RoundTrip(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
	authHeader := c.auth.Header()
	resp, err := c.doRequest(ctx, req, authHeader)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusProxyAuthRequired {
		challenge := resp.Header.Get("Proxy-Authenticate")
		if challenge != "" {
			newHeader, err := c.auth.HandleChallenge(challenge)
			if err == nil && newHeader != "" {
				_ = resp.Body.Close()
				resp, err = c.doRequest(ctx, req, newHeader)
				if err != nil {
					return nil, err
				}
				defer func() { _ = resp.Body.Close() }()
			}
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	headers := make(map[string][]string)
	for k, vs := range resp.Header {
		headers[k] = vs
	}

	return &proxyModel.ProxyResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}

func (c *Client) doRequest(ctx context.Context, req *proxyModel.ProxyRequest, authHeader string) (*http.Response, error) {
	targetURL := req.URL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}

	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = strings.NewReader(string(req.Body))
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	for k, vs := range req.Headers {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	if len(req.Body) > 0 {
		httpReq.ContentLength = int64(len(req.Body))
	}
	if authHeader != "" {
		httpReq.Header.Set("Proxy-Authorization", authHeader)
	}

	return c.httpClient.Do(httpReq)
}

func (c *Client) ConnectTunnel(ctx context.Context, target string) (net.Conn, error) {
	proxyConn, err := net.DialTimeout("tcp", c.proxyURL.Host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to upstream: %w", err)
	}

	authHeader := c.auth.Header()

	for attempt := 0; attempt < 2; attempt++ {
		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: %s\r\n\r\n", target, target, authHeader)
		_ = proxyConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := proxyConn.Write([]byte(connectReq)); err != nil {
			_ = proxyConn.Close()
			return nil, fmt.Errorf("send connect: %w", err)
		}

		_ = proxyConn.SetReadDeadline(time.Now().Add(10 * time.Second))
		br := bufio.NewReader(proxyConn)
		resp, err := http.ReadResponse(br, nil)
		if err != nil {
			_ = proxyConn.Close()
			return nil, fmt.Errorf("read connect response: %w", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			_ = proxyConn.SetDeadline(time.Time{})
			return proxyConn, nil
		}

		if resp.StatusCode == http.StatusProxyAuthRequired && attempt == 0 {
			challenge := resp.Header.Get("Proxy-Authenticate")
			if challenge != "" {
				newHeader, err := c.auth.HandleChallenge(challenge)
				if err == nil && newHeader != "" {
					authHeader = newHeader
					continue
				}
			}
		}

		_ = proxyConn.Close()
		return nil, fmt.Errorf("upstream rejected connect: %s", resp.Status)
	}

	_ = proxyConn.Close()
	return nil, fmt.Errorf("upstream rejected connect after auth challenge")
}
