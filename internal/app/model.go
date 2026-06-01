package app

type TunnelConfig struct {
	LocalPort  int
	RemoteHost string
	RemotePort int
}

type AppConfig struct {
	ConfigPath  string
	Verbose     bool
	Foreground  bool
	Interactive bool
	Daemonize   bool
	PidFile     string
	Tunnels     []TunnelConfig
}
