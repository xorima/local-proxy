package tunnel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
)

type Config struct {
	LocalPort int
	RemoteHost string
	RemotePort int
}

type UpstreamConnector interface {
	ConnectTunnel(ctx context.Context, target string) (net.Conn, error)
}

type Listener struct {
	cfg    Config
	upstream UpstreamConnector
	ln     net.Listener
	wg     sync.WaitGroup
}

func StartAll(configs []Config, upstream UpstreamConnector) ([]*Listener, error) {
	var listeners []*Listener
	for _, cfg := range configs {
		l, err := Start(cfg, upstream)
		if err != nil {
			// Close any listeners we already started
			for _, started := range listeners {
				started.Close()
			}
			return nil, fmt.Errorf("start tunnel %d -> %s:%d: %w", cfg.LocalPort, cfg.RemoteHost, cfg.RemotePort, err)
		}
		listeners = append(listeners, l)
	}
	return listeners, nil
}

func Start(cfg Config, upstream UpstreamConnector) (*Listener, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.LocalPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	l := &Listener{
		cfg:      cfg,
		upstream: upstream,
		ln:       ln,
	}

	l.wg.Add(1)
	go l.acceptLoop()

	slog.Info("tunnel started", "local", addr, "remote", fmt.Sprintf("%s:%d", cfg.RemoteHost, cfg.RemotePort))
	return l, nil
}

func (l *Listener) acceptLoop() {
	defer l.wg.Done()
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			return
		}
		go l.handleConn(conn)
	}
}

func (l *Listener) handleConn(local net.Conn) {
	defer func() { _ = local.Close() }()

	target := fmt.Sprintf("%s:%d", l.cfg.RemoteHost, l.cfg.RemotePort)
	remote, err := l.upstream.ConnectTunnel(context.Background(), target)
	if err != nil {
		slog.Warn("tunnel connect failed", "target", target, "error", err)
		return
	}
	defer func() { _ = remote.Close() }()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(remote, local)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(local, remote)
	}()
	wg.Wait()
}

func (l *Listener) Addr() net.Addr {
	return l.ln.Addr()
}

func (l *Listener) Close() {
	_ = l.ln.Close()
	l.wg.Wait()
}
