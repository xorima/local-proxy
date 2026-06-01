package adapters

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
)

type UpstreamConnector interface {
	ConnectTunnel(ctx context.Context, target string) (net.Conn, error)
}

type SOCKS5Server struct {
	ln       net.Listener
	upstream UpstreamConnector
	username string
	password string
	wg       sync.WaitGroup
}

func NewSOCKS5Server(upstream UpstreamConnector, listen, username, password string) (*SOCKS5Server, error) {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, fmt.Errorf("socks5 listen on %s: %w", listen, err)
	}
	s := &SOCKS5Server{
		ln:       ln,
		upstream: upstream,
		username: username,
		password: password,
	}
	s.wg.Add(1)
	go s.acceptLoop()
	slog.Info("SOCKS5 proxy started", "listen", listen)
	return s, nil
}

func (s *SOCKS5Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *SOCKS5Server) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	buf := make([]byte, 256)

	// Read auth methods
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	if buf[0] != 0x05 {
		return
	}
	nMethods := int(buf[1])
	if nMethods < 1 || nMethods > 255 {
		return
	}
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil {
		return
	}

	if s.username != "" {
		// Require username/password auth
		if _, err := conn.Write([]byte{0x05, 0x02}); err != nil {
			return
		}
		if !s.authPassword(conn, buf) {
			return
		}
	} else {
		// No auth
		if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
			return
		}
	}

	// Read request
	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return
	}
	ver, cmd, _, atyp := buf[0], buf[1], buf[2], buf[3]
	if ver != 0x05 || cmd != 0x01 {
		sendReply(conn, 0x07)
		return
	}

	target, err := readAddr(conn, atyp, buf)
	if err != nil {
		sendReply(conn, 0x08)
		return
	}

	remote, err := s.upstream.ConnectTunnel(context.Background(), target)
	if err != nil {
		slog.Warn("socks5 upstream connect failed", "target", target, "error", err)
		sendReply(conn, 0x04)
		return
	}
	defer func() { _ = remote.Close() }()

	sendReply(conn, 0x00)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(remote, conn)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(conn, remote)
	}()
	wg.Wait()
}

func (s *SOCKS5Server) authPassword(conn net.Conn, buf []byte) bool {
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return false
	}
	if buf[0] != 0x01 {
		sendReply(conn, 0xFF)
		return false
	}
	ulen := int(buf[1])
	if ulen < 1 || ulen > 255 {
		return false
	}
	if _, err := io.ReadFull(conn, buf[:ulen]); err != nil {
		return false
	}
	user := string(buf[:ulen])
	if _, err := io.ReadFull(conn, buf[:1]); err != nil {
		return false
	}
	plen := int(buf[0])
	if plen < 1 || plen > 255 {
		return false
	}
	if _, err := io.ReadFull(conn, buf[:plen]); err != nil {
		return false
	}
	pass := string(buf[:plen])

	if user != s.username || pass != s.password {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return false
	}
	_, _ = conn.Write([]byte{0x01, 0x00})
	return true
}

func readAddr(conn net.Conn, atyp byte, buf []byte) (string, error) {
	switch atyp {
	case 0x01: // IPv4
		if _, err := io.ReadFull(conn, buf[:6]); err != nil {
			return "", err
		}
		ip := net.IP(buf[:4])
		port := binary.BigEndian.Uint16(buf[4:6])
		return fmt.Sprintf("%s:%d", ip, port), nil
	case 0x03: // Domain name
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return "", err
		}
		hlen := int(buf[0])
		if hlen < 1 || hlen > 255 {
			return "", fmt.Errorf("invalid hostname length %d", hlen)
		}
		if _, err := io.ReadFull(conn, buf[:hlen+2]); err != nil {
			return "", err
		}
		host := string(buf[:hlen])
		port := binary.BigEndian.Uint16(buf[hlen : hlen+2])
		return fmt.Sprintf("%s:%d", host, port), nil
	case 0x04: // IPv6
		if _, err := io.ReadFull(conn, buf[:18]); err != nil {
			return "", err
		}
		ip := net.IP(buf[:16])
		port := binary.BigEndian.Uint16(buf[16:18])
		return fmt.Sprintf("[%s]:%d", ip, port), nil
	default:
		return "", fmt.Errorf("unknown address type %d", atyp)
	}
}

func sendReply(conn net.Conn, reply byte) {
	// version, reply, reserved, address type, bind address (4 bytes), bind port (2 bytes)
	_, _ = conn.Write([]byte{0x05, reply, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
}

func (s *SOCKS5Server) Addr() net.Addr {
	return s.ln.Addr()
}

func (s *SOCKS5Server) Close() {
	_ = s.ln.Close()
	s.wg.Wait()
}
