package config

type UpstreamConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type AuthConfig struct {
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	Domain     string `yaml:"domain"`
	AuthType   string `yaml:"auth_type"`
	PassNT     string `yaml:"pass_nt"`
	PassNTLMv2 string `yaml:"pass_ntlmv2"`
}

type FailoverConfig struct {
	RetryInterval string `yaml:"retry_interval"`
	Standalone    bool   `yaml:"standalone"`
}

type CacheConfig struct {
	MaxConnections int    `yaml:"max_connections"`
	IdleTimeout    string `yaml:"idle_timeout"`
}

type ACLConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

type GatewayConfig struct {
	Enabled bool     `yaml:"enabled"`
	ACL    ACLConfig `yaml:"acl"`
}

type HeaderConfig struct {
	Set    map[string]string `yaml:"set"`
	Remove []string          `yaml:"remove"`
}

type SOCKS5Config struct {
	Listen   string `yaml:"listen"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Config struct {
	Upstream  UpstreamConfig   `yaml:"upstream"`
	Upstreams []UpstreamConfig `yaml:"upstreams"`
	Failover  FailoverConfig   `yaml:"failover"`
	Auth      AuthConfig       `yaml:"auth"`
	Listen    string           `yaml:"listen"`
	Cache     CacheConfig      `yaml:"cache"`
	NoProxy   []string         `yaml:"no_proxy"`
	LogLevel  string           `yaml:"log_level"`
	Gateway   GatewayConfig    `yaml:"gateway"`
	Headers   HeaderConfig     `yaml:"headers"`
	SOCKS5    SOCKS5Config     `yaml:"socks5"`
}
