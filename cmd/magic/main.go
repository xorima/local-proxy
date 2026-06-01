package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	proxyHost := flag.String("proxy", "localhost:13128", "Proxy host:port")
	flag.Parse()

	conn, err := net.DialTimeout("tcp", *proxyHost, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to proxy %s: %v\n", *proxyHost, err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()

	// Send a GET request without auth to trigger 407
	req := "GET http://example.com/ HTTP/1.1\r\nHost: example.com\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		fmt.Fprintf(os.Stderr, "send request: %v\n", err)
		os.Exit(1)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read response: %v\n", err)
		os.Exit(1)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusProxyAuthRequired {
		fmt.Printf("Proxy does not require authentication (status %d)\n", resp.StatusCode)
		return
	}

	authHeader := resp.Header.Get("Proxy-Authenticate")
	if authHeader == "" {
		fmt.Println("No Proxy-Authenticate header found")
		return
	}

	fmt.Println("Proxy-Authenticate:", authHeader)

	if strings.HasPrefix(authHeader, "NTLM ") || authHeader == "NTLM" || strings.Contains(authHeader, "NTLM") {
		fmt.Println("\n# Proxy supports NTLM authentication")
		fmt.Println("# Recommended config:")
		fmt.Println("auth:")
		fmt.Println("  auth_type: ntlmv2  # or ntlm, ntlm-session")

		// Decode NTLM Type 2 challenge to extract flags
		b64 := strings.TrimPrefix(authHeader, "NTLM ")
		if b64 != "" {
			data, err := base64.StdEncoding.DecodeString(b64)
			if err == nil && len(data) >= 20 {
				flags := uint32(data[12]) | uint32(data[13])<<8 | uint32(data[14])<<16 | uint32(data[15])<<24
				fmt.Println("# NTLM flags: 0x" + fmt.Sprintf("%08x", flags))
				if flags&0x00080000 != 0 {
					fmt.Println("# NTLM2 Key exchange supported (NTLM2 Session Response)")
				}
				if flags&0x00020000 != 0 {
					fmt.Println("# NTLMv2 (Target Info) supported")
				}
				if flags&0x00000200 != 0 {
					fmt.Println("# NTLM (v1) supported")
				}
			}
		}
	}

	if strings.Contains(authHeader, "Basic") {
		fmt.Println("\n# Proxy supports Basic authentication")
	}
}
