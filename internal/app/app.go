package app

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	authAdapters "local-proxy/internal/domains/auth/adapters"
	authModel "local-proxy/internal/domains/auth/model"
	"local-proxy/internal/domains/proxy/ports"
	proxyAdapters "local-proxy/internal/domains/proxy/adapters"
	proxyController "local-proxy/internal/domains/proxy/controller"
	proxyService "local-proxy/internal/domains/proxy/service"
	"local-proxy/internal/domains/tunnel"
	configClient "local-proxy/internal/clients/config"
	directClient "local-proxy/internal/clients/direct"
	failoverClient "local-proxy/internal/clients/failover"
	upstreamClient "local-proxy/internal/clients/upstream"
)

func Run(cfg *AppConfig) {
	conf, err := configClient.Load(cfg.ConfigPath)
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	setupLogging(conf.LogLevel)

	if cfg.Interactive {
		pw, err := readPassword(fmt.Sprintf("Password for %s@%s: ", conf.Auth.Username, conf.Upstream.Host))
		if err != nil {
			slog.Error("password prompt failed", "error", err)
			os.Exit(1)
		}
		conf.Auth.Password = pw
	}

	var auth ports.AuthProvider
	if conf.Auth.PassNT != "" {
		ntHash, err := hex.DecodeString(conf.Auth.PassNT)
		if err != nil {
			slog.Error("invalid pass_nt hash", "error", err)
			os.Exit(1)
		}
		var ntlmv2Hash []byte
		if conf.Auth.PassNTLMv2 != "" {
			ntlmv2Hash, err = hex.DecodeString(conf.Auth.PassNTLMv2)
			if err != nil {
				slog.Error("invalid pass_ntlmv2 hash", "error", err)
				os.Exit(1)
			}
		}
		switch strings.ToLower(conf.Auth.AuthType) {
		case "ntlmv2":
			auth = authAdapters.NewNTLMProviderWithHash(conf.Auth.Username, conf.Auth.Domain, ntHash, ntlmv2Hash, authModel.NTLMModeV2)
		case "ntlm-session":
			auth = authAdapters.NewNTLMProviderWithHash(conf.Auth.Username, conf.Auth.Domain, ntHash, ntlmv2Hash, authModel.NTLMModeSession)
		default:
			auth = authAdapters.NewNTLMProviderWithHash(conf.Auth.Username, conf.Auth.Domain, ntHash, ntlmv2Hash, authModel.NTLMModeV1)
		}
	} else {
		switch strings.ToLower(conf.Auth.AuthType) {
		case "ntlm":
			auth = authAdapters.NewNTLMProvider(conf.Auth.Username, conf.Auth.Password, conf.Auth.Domain, authModel.NTLMModeV1)
		case "ntlmv2":
			auth = authAdapters.NewNTLMProvider(conf.Auth.Username, conf.Auth.Password, conf.Auth.Domain, authModel.NTLMModeV2)
		case "ntlm-session":
			auth = authAdapters.NewNTLMProvider(conf.Auth.Username, conf.Auth.Password, conf.Auth.Domain, authModel.NTLMModeSession)
		default:
			auth = authAdapters.NewBasicProvider(conf.Auth.Username, conf.Auth.Password)
		}
	}

	conf.Auth.Password = ""
	conf.Auth.PassNT = ""
	conf.Auth.PassNTLMv2 = ""

	idleTimeout, _ := time.ParseDuration(conf.Cache.IdleTimeout)

	var upstreamConn ports.UpstreamConnector
	upstreams := conf.Upstreams
	if len(upstreams) == 0 {
		upstreams = []configClient.UpstreamConfig{conf.Upstream}
	}

	var upstreamClients []failoverClient.UpstreamClient
	var hosts []string
	for _, u := range upstreams {
		cl := upstreamClient.New(upstreamClient.Config{
			Host:        u.Host,
			Port:        u.Port,
			MaxConns:    conf.Cache.MaxConnections,
			IdleTimeout: idleTimeout,
		}, auth)
		upstreamClients = append(upstreamClients, cl)
		hosts = append(hosts, fmt.Sprintf("%s:%d", u.Host, u.Port))
	}

	retryInterval, _ := time.ParseDuration(conf.Failover.RetryInterval)
	var standalone failoverClient.UpstreamClient
	if conf.Failover.Standalone {
		standalone = directClient.New()
	}
	upstreamConn = upstreamClients[0]
	if len(upstreamClients) > 1 || standalone != nil {
		upstreamConn = failoverClient.New(upstreamClients, hosts, retryInterval, standalone)
	}

	noproxy := proxyAdapters.NewNoProxyMatcher(conf.NoProxy)
	direct := directClient.New()
	header := proxyAdapters.NewHeaderModifier(conf.Headers.Set, conf.Headers.Remove)
	svc := proxyService.NewFull(upstreamConn, direct, auth, noproxy, header)

	if conf.SOCKS5.Listen != "" {
		socksServer, err := proxyAdapters.NewSOCKS5Server(upstreamConn, conf.SOCKS5.Listen, conf.SOCKS5.Username, conf.SOCKS5.Password)
		if err != nil {
			slog.Error("SOCKS5 start failed", "error", err)
			os.Exit(1)
		}
		defer socksServer.Close()
	}

	var ctrl *proxyController.HTTPController
	if conf.Gateway.Enabled && (len(conf.Gateway.ACL.Allow) > 0 || len(conf.Gateway.ACL.Deny) > 0) {
		acl, err := proxyAdapters.NewACLMatcher(conf.Gateway.ACL.Allow, conf.Gateway.ACL.Deny)
		if err != nil {
			slog.Error("invalid ACL config", "error", err)
			os.Exit(1)
		}
		ctrl = proxyController.NewHTTPWithACL(svc, acl)
	} else {
		ctrl = proxyController.NewHTTP(svc)
	}

	slog.Info("proxy started", "listen", conf.Listen, "upstreams", hosts)

	if cfg.Daemonize {
		daemonize()
	}

	if cfg.PidFile != "" {
		if err := writePidFile(cfg.PidFile); err != nil {
			slog.Error("pid file write failed", "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := os.Remove(cfg.PidFile); err != nil {
				slog.Error("pid file remove failed", "error", err)
			}
		}()
	}

	var tunnelListeners []*tunnel.Listener
	if len(cfg.Tunnels) > 0 {
		tunnelCfgs := make([]tunnel.Config, len(cfg.Tunnels))
		for i, t := range cfg.Tunnels {
			tunnelCfgs[i] = tunnel.Config{
				LocalPort:  t.LocalPort,
				RemoteHost: t.RemoteHost,
				RemotePort: t.RemotePort,
			}
		}
		var err error
		tunnelListeners, err = tunnel.StartAll(tunnelCfgs, upstreamConn)
		if err != nil {
			slog.Error("tunnel start failed", "error", err)
			os.Exit(1)
		}
	}

	srv := &http.Server{Addr: conf.Listen, Handler: ctrl}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown", "error", err)
	}
	for _, l := range tunnelListeners {
		l.Close()
	}
	slog.Info("shutdown complete")
}

func setupLogging(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(handler))
}

func writePidFile(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644)
}

func daemonize() {
	if os.Getppid() != 1 {
		// Already running in background
		return
	}
	// In a real daemonization, we'd fork/exec. For now, just log.
	slog.Info("daemon mode enabled (running in foreground)")
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(pw), nil
}
