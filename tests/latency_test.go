//go:build integration

package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	squidAddr = "127.0.0.1:13128"
	proxyPort = "3128"
	proxyAddr = "127.0.0.1:3128"
	squidUser = "user"
	squidPass = "pass"
)

func TestLatencyOverhead(t *testing.T) {
	ensureSquidRunning(t)

	// write config for local-proxy (points at Squid)
	cfg := fmt.Sprintf(`
upstream:
  host: 127.0.0.1
  port: 13128
auth:
  username: %s
  password: %s
  auth_type: basic
listen: "127.0.0.1:%s"
`, squidUser, squidPass, proxyPort)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "local-proxy.yaml")
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	binaryPath := filepath.Join(tmpDir, "proxy")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/serve")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build local-proxy: %v\n%s", err, out)
	}

	cmd := exec.Command(binaryPath, "-c", configPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start local-proxy: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
	})

	waitForPort(t, proxyAddr, 10*time.Second)

	targetURL := "http://example.com/"
	const iterations = 20

	warmup(squidAddr, targetURL, squidUser, squidPass, 5)
	warmup(proxyAddr, targetURL, "", "", 5)

	directTimes := measureLatency(squidAddr, targetURL, squidUser, squidPass, iterations)
	proxyTimes := measureLatency(proxyAddr, targetURL, "", "", iterations)

	directAvg := avg(directTimes)
	proxyAvg := avg(proxyTimes)
	delta := proxyAvg - directAvg

	t.Logf("Direct (client → Squid):          avg %.2f ms", directAvg)
	t.Logf("Proxied (client → proxy → Squid): avg %.2f ms", proxyAvg)
	t.Logf("Overhead delta:                   %.2f ms", delta)

	if delta > 10 {
		t.Errorf("latency overhead %.2f ms exceeds 10 ms threshold", delta)
	}
}

func warmup(proxyAddr, targetURL, user, pass string, n int) {
	client := buildClient(proxyAddr, user, pass, 3*time.Second)
	for i := 0; i < n; i++ {
		req, _ := http.NewRequest("GET", targetURL, nil)
		if resp, err := client.Do(req); err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
}

func measureLatency(proxyAddr, targetURL, user, pass string, n int) []float64 {
	client := buildClient(proxyAddr, user, pass, 10*time.Second)
	var times []float64

	for i := 0; i < n; i++ {
		req, _ := http.NewRequest("GET", targetURL, nil)
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		elapsed := time.Since(start).Seconds() * 1000
		times = append(times, elapsed)
	}

	return times
}

func buildClient(proxyAddr, user, pass string, timeout time.Duration) *http.Client {
	c := &http.Client{Timeout: timeout}
	if proxyAddr != "" {
		var rawURL string
		if user != "" {
			rawURL = fmt.Sprintf("http://%s:%s@%s", user, pass, proxyAddr)
		} else {
			rawURL = fmt.Sprintf("http://%s", proxyAddr)
		}
		proxyURLParsed, _ := url.Parse(rawURL)
		c.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
		}
	}
	return c
}

func ensureSquidRunning(t *testing.T) {
	t.Helper()
	client := buildClient(squidAddr, squidUser, squidPass, 3*time.Second)
	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("Squid not reachable at %s (start with: docker compose up -d in test-env/): %v", squidAddr, err)
	}
	resp.Body.Close()
}

func waitForPort(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.Get(fmt.Sprintf("http://%s/", addr))
		if err == nil {
			req.Body.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", addr)
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
