package upstream

import "time"

type Config struct {
	Host        string
	Port        int
	MaxConns    int
	IdleTimeout time.Duration
}
