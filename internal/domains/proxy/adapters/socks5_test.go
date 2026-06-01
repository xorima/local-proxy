package adapters_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"local-proxy/internal/domains/proxy/adapters"
)

type socksUpstream struct {
	connectFn func(ctx context.Context, target string) (net.Conn, error)
}

func (m *socksUpstream) ConnectTunnel(ctx context.Context, target string) (net.Conn, error) {
	return m.connectFn(ctx, target)
}

func socks5Connect(addr string, target string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Auth negotiation
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	buf := make([]byte, 2)
	if _, err := conn.Read(buf); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected auth response: %v", buf)
	}

	// Connect request (domain name)
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	port := portToUint16(portStr)
	req := append([]byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}, []byte(host)...)
	portBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(portBuf, port)
	req = append(req, portBuf...)

	if _, err := conn.Write(req); err != nil {
		_ = conn.Close()
		return nil, err
	}

	// Read response
	respBuf := make([]byte, 10)
	if _, err := conn.Read(respBuf); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if respBuf[1] != 0x00 {
		_ = conn.Close()
		return nil, fmt.Errorf("connection rejected: %d", respBuf[1])
	}

	return conn, nil
}

func portToUint16(portStr string) uint16 {
	var p int
	_, _ = fmt.Sscanf(portStr, "%d", &p)
	return uint16(p)
}

func TestSOCKS5Server(t *testing.T) {
	t.Run("it should accept SOCKS5 connections and forward via upstream", func(t *testing.T) {
		upstream := &socksUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				client, server := net.Pipe()
				go func() {
					_, _ = server.Write([]byte("hello from remote"))
					_ = server.Close()
				}()
				return client, nil
			},
		}

		server, err := adapters.NewSOCKS5Server(upstream, "127.0.0.1:0", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer server.Close()

		conn, err := socks5Connect(server.Addr().String(), "example.com:443")
		if err != nil {
			t.Fatalf("socks5 connect: %v", err)
		}
		defer func() { _ = conn.Close() }()

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if string(buf[:n]) != "hello from remote" {
			t.Errorf("got %q, want %q", string(buf[:n]), "hello from remote")
		}
	})

	t.Run("it should reject when upstream is unreachable", func(t *testing.T) {
		upstream := &socksUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				return nil, fmt.Errorf("connection refused")
			},
		}

		server, err := adapters.NewSOCKS5Server(upstream, "127.0.0.1:0", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer server.Close()

		_, err = socks5Connect(server.Addr().String(), "example.com:443")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSOCKS5Server_Auth(t *testing.T) {
	t.Run("it should require auth when credentials are configured", func(t *testing.T) {
		upstream := &socksUpstream{
			connectFn: func(ctx context.Context, target string) (net.Conn, error) {
				client, server := net.Pipe()
				go func() {
					_, _ = server.Write([]byte("ok"))
					_ = server.Close()
				}()
				return client, nil
			},
		}

		server, err := adapters.NewSOCKS5Server(upstream, "127.0.0.1:0", "user", "pass")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer server.Close()

		conn, err := net.DialTimeout("tcp", server.Addr().String(), 5*time.Second)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer func() { _ = conn.Close() }()

		// Should see 0x02 (user/pass required)
		if _, err := conn.Write([]byte{0x05, 0x02, 0x00, 0x02}); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 2)
		if _, err := conn.Read(buf); err != nil {
			t.Fatal(err)
		}
		if buf[1] != 0x02 {
			t.Fatalf("expected auth method 0x02, got 0x%02x", buf[1])
		}

		// Send username/password auth
		auth := []byte{0x01, 0x04}
		auth = append(auth, []byte("user")...)
		auth = append(auth, 0x04)
		auth = append(auth, []byte("pass")...)
		if _, err := conn.Write(auth); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Read(buf[:2]); err != nil {
			t.Fatal(err)
		}
		if buf[1] != 0x00 {
			t.Fatalf("auth failed, status 0x%02x", buf[1])
		}
	})
}
