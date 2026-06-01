package ntlmmock

import (
	"bufio"
	"crypto/des"
	"crypto/hmac"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"local-proxy/internal/domains/auth/adapters"
)

type Server struct {
	password string
	user     string
	domain   string
	ntlmv2   bool

	listener net.Listener
	Port     int
	URL      string

	mu        sync.Mutex
	challenge []byte
}

func New(password, user, domain string, ntlmv2 bool) *Server {
	return &Server{
		password: password,
		user:     user,
		domain:   domain,
		ntlmv2:   ntlmv2,
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = listener
	s.Port = listener.Addr().(*net.TCPAddr).Port
	s.URL = fmt.Sprintf("http://127.0.0.1:%d", s.Port)
	go s.acceptLoop()
	return nil
}

func (s *Server) Close() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	br := bufio.NewReader(conn)
	handshakeDone := false

	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}

		authHeader := req.Header.Get("Proxy-Authorization")
		challenge := s.getOrCreateChallenge()

		if authHeader == "" {
			write407(conn, challenge)
			_ = req.Body.Close()
			return
		}

		if !strings.HasPrefix(authHeader, "NTLM ") {
			write407(conn, challenge)
			_ = req.Body.Close()
			return
		}

		data, err := base64.StdEncoding.DecodeString(authHeader[5:])
		if err != nil || len(data) < 12 {
			write407(conn, challenge)
			_ = req.Body.Close()
			return
		}

		msgType := data[8]

		switch msgType {
		case 1:
			s.newChallenge()
			write407(conn, s.getOrCreateChallenge())
			_ = req.Body.Close()

		case 3:
			if req.Method == "CONNECT" {
				if s.validateType3(data, challenge) {
					_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
				} else {
					_, _ = conn.Write([]byte("HTTP/1.1 407 Proxy Auth Required\r\n\r\n"))
				}
				_ = req.Body.Close()
				_, _ = io.Copy(io.Discard, conn)
				return
			}

			if s.validateType3(data, challenge) {
				resp := "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nContent-Type: text/plain\r\n\r\nOK"
				_, _ = conn.Write([]byte(resp))
				handshakeDone = true
			} else {
				write407(conn, challenge)
			}
			_ = req.Body.Close()

			if handshakeDone {
				_, _ = io.Copy(io.Discard, conn)
				return
			}

		default:
			write407(conn, challenge)
			_ = req.Body.Close()
			return
		}
	}
}

func (s *Server) newChallenge() {
	nonce := make([]byte, 8)
	for i := range nonce {
		nonce[i] = byte(time.Now().UnixNano() >> (i * 8))
	}
	s.mu.Lock()
	s.challenge = adapters.GenerateType2Challenge(nonce)
	s.mu.Unlock()
}

func (s *Server) getOrCreateChallenge() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.challenge == nil {
		nonce := make([]byte, 8)
		for i := range nonce {
			nonce[i] = byte(time.Now().UnixNano() >> (i * 8))
		}
		s.challenge = adapters.GenerateType2Challenge(nonce)
	}
	return s.challenge
}

func (s *Server) validateType3(type3Msg, type2Msg []byte) bool {
	if len(type3Msg) < 64 || len(type2Msg) < 32 {
		return false
	}

	serverChallenge := type2Msg[24:32]

	if s.ntlmv2 {
		return s.validateNTLMv2(type3Msg, serverChallenge)
	}

	if s.validateNTLMv1(type3Msg, serverChallenge) {
		return true
	}
	return s.validateNTLMSession(type3Msg, serverChallenge)
}

func (s *Server) validateNTLMv1(type3Msg, serverChallenge []byte) bool {
	// NT/LM response at offset 12 (security buffer): 2 len, 2 max, 4 offset
	lmLen := int(binary.LittleEndian.Uint16(type3Msg[12:14]))
	lmOffset := int(binary.LittleEndian.Uint32(type3Msg[16:20]))

	if lmLen < 24 || lmOffset+24 > len(type3Msg) {
		return false
	}

	lmResp := type3Msg[lmOffset : lmOffset+24]
	lmResp = lmResp[:24]

	// Re-compute expected LM response
	ntHash := adapters.NTHash(s.password)
	expectedLM := make([]byte, 24)
	copy(expectedLM, lmResponseBytes(ntHash[:7], serverChallenge))
	copy(expectedLM[8:], lmResponseBytes(ntHash[7:14], serverChallenge))

	return hmac.Equal(expectedLM[:16], lmResp[:16])
}

func (s *Server) validateNTLMv2(type3Msg, serverChallenge []byte) bool {
	// NT response at offset 20 (security buffer)
	ntLen := int(binary.LittleEndian.Uint16(type3Msg[20:22]))
	ntOffset := int(binary.LittleEndian.Uint32(type3Msg[24:28]))

	if ntLen < 32 || ntOffset+ntLen > len(type3Msg) {
		return false
	}

	ntResp := type3Msg[ntOffset : ntOffset+ntLen]

	if ntLen < 16 {
		return false
	}
	clientHMAC := ntResp[:16]

	// Blob starts at offset 16 in the NT response
	blob := ntResp[16:]

	ntowfv2 := adapters.NTOWFv2(s.password, s.user, s.domain)

	mac := hmac.New(md5.New, ntowfv2)
	mac.Write(serverChallenge)
	mac.Write(blob)
	expectedHMAC := mac.Sum(nil)

	return hmac.Equal(expectedHMAC, clientHMAC)
}

func (s *Server) validateNTLMSession(type3Msg, serverChallenge []byte) bool {
	// LM response has client nonce at offset 12
	lmLen := int(binary.LittleEndian.Uint16(type3Msg[12:14]))
	lmOffset := int(binary.LittleEndian.Uint32(type3Msg[16:20]))
	if lmLen < 8 || lmOffset+8 > len(type3Msg) {
		return false
	}

	clientNonce := type3Msg[lmOffset : lmOffset+8]

	// Compute mixed challenge = HMAC-MD5(serverChallenge + clientNonce)[0..7]
	mac := hmac.New(md5.New, serverChallenge)
	mac.Write(clientNonce)
	mixedChallenge := mac.Sum(nil)[:8]

	// Validate NT response against mixed challenge
	ntLen := int(binary.LittleEndian.Uint16(type3Msg[20:22]))
	ntOffset := int(binary.LittleEndian.Uint32(type3Msg[24:28]))
	if ntLen < 24 || ntOffset+24 > len(type3Msg) {
		return false
	}

	ntResp := type3Msg[ntOffset : ntOffset+24]
	ntHash := adapters.NTHash(s.password)

	expected := make([]byte, 24)
	copy(expected, lmResponseBytes(ntHash[:7], mixedChallenge))
	copy(expected[8:], lmResponseBytes(ntHash[7:14], mixedChallenge))

	return hmac.Equal(expected[:16], ntResp[:16])
}

func write407(conn net.Conn, challenge []byte) {
	if challenge == nil {
		nonce := make([]byte, 8)
		for i := range nonce {
			nonce[i] = byte(time.Now().UnixNano() >> (i * 8))
		}
		challenge = adapters.GenerateType2Challenge(nonce)
	}
	b64 := base64.StdEncoding.EncodeToString(challenge)
	resp := fmt.Sprintf("HTTP/1.1 407 Proxy Auth Required\r\nProxy-Authenticate: NTLM %s\r\nContent-Length: 0\r\n\r\n", b64)
	_, _ = conn.Write([]byte(resp))
}

func lmResponseBytes(key7, data []byte) []byte {
	if len(key7) < 7 {
		k := make([]byte, 7)
		copy(k, key7)
		key7 = k
	}
	if len(data) < 8 {
		d := make([]byte, 8)
		copy(d, data)
		data = d
	}
	key := make([]byte, 8)
	key[0] = key7[0]
	key[1] = (key7[0] << 7) | (key7[1] >> 1)
	key[2] = (key7[1] << 6) | (key7[2] >> 2)
	key[3] = (key7[2] << 5) | (key7[3] >> 3)
	key[4] = (key7[3] << 4) | (key7[4] >> 4)
	key[5] = (key7[4] << 3) | (key7[5] >> 5)
	key[6] = (key7[5] << 2) | (key7[6] >> 6)
	key[7] = key7[6] << 1

	block, err := des.NewCipher(key)
	if err != nil {
		return make([]byte, 8)
	}
	dst := make([]byte, 8)
	block.Encrypt(dst, data[:8])
	return dst
}
