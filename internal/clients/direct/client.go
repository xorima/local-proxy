package direct

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	proxyModel "local-proxy/internal/domains/proxy/model"
)

type Client struct {
	httpClient *http.Client
	dialTimeout time.Duration
}

func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			},
		},
		dialTimeout: 10 * time.Second,
	}
}

func (c *Client) RoundTrip(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("create direct request: %w", err)
	}
	for k, v := range req.Headers {
		httpReq.Header[k] = v
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("direct request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read direct response: %w", err)
	}

	respHeaders := make(map[string][]string, len(httpResp.Header))
	for k, v := range httpResp.Header {
		respHeaders[k] = v
	}

	return &proxyModel.ProxyResponse{
		StatusCode: httpResp.StatusCode,
		Headers:    respHeaders,
		Body:       body,
	}, nil
}

func (c *Client) ConnectTunnel(ctx context.Context, target string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: c.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("direct connect to %s: %w", target, err)
	}
	return conn, nil
}

func (c *Client) ConnectWithPort(ctx context.Context, host string, port int) (net.Conn, error) {
	return c.ConnectTunnel(ctx, net.JoinHostPort(host, strconv.Itoa(port)))
}
