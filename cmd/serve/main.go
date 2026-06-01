package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"local-proxy/internal/app"
)

func main() {
	cfg := &app.AppConfig{}
	flag.StringVar(&cfg.ConfigPath, "c", "local-proxy.yaml", "Config file path")
	flag.BoolVar(&cfg.Verbose, "v", false, "Verbose logging")
	flag.BoolVar(&cfg.Foreground, "f", false, "Run in foreground")
	flag.BoolVar(&cfg.Interactive, "I", false, "Prompt for password interactively")
	flag.BoolVar(&cfg.Daemonize, "d", false, "Daemonize (run in background)")
	flag.StringVar(&cfg.PidFile, "p", "", "PID file path")

	var tunnels multiFlag
	flag.Var(&tunnels, "L", "TCP tunnel: local_port:remote_host:remote_port (can be specified multiple times)")

	flag.Parse()

	cfg.Tunnels = parseTunnels(tunnels)
	if err := validateTunnels(cfg.Tunnels); err != nil {
		fmt.Fprintf(os.Stderr, "invalid tunnel config: %v\n", err)
		os.Exit(1)
	}

	app.Run(cfg)
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func parseTunnels(raw multiFlag) []app.TunnelConfig {
	var tunnels []app.TunnelConfig
	for _, t := range raw {
		parts := strings.SplitN(t, ":", 3)
		if len(parts) != 3 {
			continue
		}
		localPort, _ := strconv.Atoi(parts[0])
		remotePort, _ := strconv.Atoi(parts[2])
		if localPort <= 0 || remotePort <= 0 {
			continue
		}
		tunnels = append(tunnels, app.TunnelConfig{
			LocalPort:  localPort,
			RemoteHost: parts[1],
			RemotePort: remotePort,
		})
	}
	return tunnels
}

func validateTunnels(tunnels []app.TunnelConfig) error {
	for _, t := range tunnels {
		if t.LocalPort < 1 || t.LocalPort > 65535 {
			return fmt.Errorf("invalid local port %d", t.LocalPort)
		}
		if t.RemotePort < 1 || t.RemotePort > 65535 {
			return fmt.Errorf("invalid remote port %d", t.RemotePort)
		}
		if t.RemoteHost == "" {
			return fmt.Errorf("remote host cannot be empty")
		}
	}
	return nil
}
