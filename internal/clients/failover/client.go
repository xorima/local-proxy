package failover

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	proxyModel "local-proxy/internal/domains/proxy/model"
)

type UpstreamClient interface {
	RoundTrip(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error)
	ConnectTunnel(ctx context.Context, target string) (net.Conn, error)
}

type Client struct {
	mu            sync.Mutex
	proxies       []UpstreamClient
	hosts         []string
	failedAt      map[int]time.Time
	cooldown      time.Duration
	standalone    UpstreamClient
	currentIdx    int
	slog          *slog.Logger
}

func New(proxies []UpstreamClient, hosts []string, cooldown time.Duration, standalone UpstreamClient) *Client {
	return &Client{
		proxies:    proxies,
		hosts:      hosts,
		failedAt:   make(map[int]time.Time),
		cooldown:   cooldown,
		standalone: standalone,
		slog:       slog.Default(),
	}
}

func (c *Client) RoundTrip(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
	return c.do(ctx, func(p UpstreamClient) (*proxyModel.ProxyResponse, error) {
		return p.RoundTrip(ctx, req)
	})
}

func (c *Client) ConnectTunnel(ctx context.Context, target string) (net.Conn, error) {
	var result net.Conn
	err := c.doConn(ctx, func(p UpstreamClient) (net.Conn, error) {
		return p.ConnectTunnel(ctx, target)
	}, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) do(ctx context.Context, fn func(UpstreamClient) (*proxyModel.ProxyResponse, error)) (*proxyModel.ProxyResponse, error) {
	for i := 0; i < len(c.proxies); i++ {
		idx := c.nextIndex()
		if !c.isAvailable(idx) {
			continue
		}
		resp, err := fn(c.proxies[idx])
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		c.markFailed(idx)
	}

	if c.standalone != nil {
		c.slog.Info("all proxies failed, falling back to direct connection")
		return fn(c.standalone)
	}
	return nil, fmt.Errorf("all proxies failed")
}

func (c *Client) doConn(ctx context.Context, fn func(UpstreamClient) (net.Conn, error), result *net.Conn) error {
	for i := 0; i < len(c.proxies); i++ {
		idx := c.nextIndex()
		if !c.isAvailable(idx) {
			continue
		}
		conn, err := fn(c.proxies[idx])
		if err == nil {
			*result = conn
			return nil
		}
		c.markFailed(idx)
	}

	if c.standalone != nil {
		c.slog.Info("all proxies failed, falling back to direct connection")
		conn, err := fn(c.standalone)
		if err != nil {
			return fmt.Errorf("direct fallback failed: %w", err)
		}
		*result = conn
		return nil
	}
	return fmt.Errorf("all proxies failed")
}

func (c *Client) nextIndex() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := c.currentIdx
	c.currentIdx = (c.currentIdx + 1) % len(c.proxies)
	return idx
}

func (c *Client) isAvailable(idx int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.failedAt[idx]
	if !ok {
		return true
	}
	if time.Since(t) >= c.cooldown {
		delete(c.failedAt, idx)
		slog.Info("proxy available again", "host", c.hosts[idx])
		return true
	}
	return false
}

func (c *Client) markFailed(idx int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failedAt[idx] = time.Now()
	slog.Warn("proxy marked as failed", "host", c.hosts[idx])
}
