# local-proxy

A Go rewrite of [cntlm](https://cntlm.sourceforge.net/) — an NTLM/NTLMv2 authenticating HTTP(S) proxy with TCP tunneling, connection caching, and multi-proxy failover.

Sits between local applications and a corporate proxy requiring authentication, handling the NTLM handshake transparently.

## Quick start

```bash
# Generate password hashes (safer than plaintext in config)
go run ./cmd/hashgen -u myuser -d MYDOMAIN

# Start the proxy
go run ./cmd/serve -c config.yaml

# Use it
curl -x http://127.0.0.1:3128 http://example.com
```

## Features

- **HTTP forward proxy** with `CONNECT` HTTPS tunneling
- **NTLMv1 / NTLMv2 / NTLM Session** authentication — transparent handshake with parent proxy
- **Basic auth** fallback
- **Multiple parent proxies** with circular failover and cooldown
- **Standalone fallback** — connect directly when all proxies are down
- **SOCKS5 proxy** with optional username/password auth
- **NoProxy bypass** — skip the proxy for matching hosts (wildcards supported)
- **Gateway mode** with CIDR-based allow/deny ACLs
- **Header modification** — set or remove headers before forwarding
- **TCP port forwarding** (`-L local:remote_host:remote_port`) via CONNECT tunnel
- **Connection caching** — configurable pool size and idle timeout
- **Pre-computed password hashes** — no plaintext in config files
- **Interactive password prompt** (`-I` flag)
- **Health endpoint** (`/__health`) and **metrics endpoint** (`/metrics`)
- **Graceful shutdown** on SIGINT/SIGTERM
- **PID file** support

## Installation

### From source

```bash
git clone https://github.com/xorima/local-proxy.git
cd local-proxy
go build -o local-proxy ./cmd/serve
```

### Pre-built binaries

Download the latest release from the [releases page](https://github.com/xorima/local-proxy/releases) for your platform.

## Configuration

### Minimal config

```yaml
upstream:
  host: proxy.corp.com
  port: 3128

auth:
  username: myuser
  password: mypass
  auth_type: ntlmv2

listen: "127.0.0.1:3128"
```

### Multi-proxy with failover

```yaml
upstreams:
  - host: proxy1.corp.com
    port: 3128
  - host: proxy2.corp.com
    port: 3128

failover:
  retry_interval: 30s
  standalone: true
```

When `standalone: true`, if all proxies are unreachable the proxy forwards requests directly (no parent proxy).

### Password hashes (no plaintext)

```bash
go run ./cmd/hashgen -u myuser -d MYDOMAIN
```

Paste the output into your config:

```yaml
auth:
  username: myuser
  domain: MYDOMAIN
  auth_type: ntlmv2
  pass_nt: EFFC3456...
  pass_ntlmv2: ABC123...
```

### SOCKS5

```yaml
socks5:
  listen: "127.0.0.1:1080"
  username: socksuser   # optional
  password: sockspass
```

### Gateway mode with ACLs

```yaml
listen: "0.0.0.0:3128"

gateway:
  enabled: true
  acl:
    allow:
      - 10.0.0.0/8
      - 192.168.0.0/16
    deny:
      - 10.0.0.0/24
```

### Header modification

```yaml
headers:
  set:
    User-Agent: local-proxy/1.0
  remove:
    - X-Forwarded-For
```

### NoProxy bypass

```yaml
no_proxy:
  - localhost
  - 10.*
  - *.corp.com
```

### Connection caching

```yaml
cache:
  max_connections: 20
  idle_timeout: "5m"
```

### Full config reference

```yaml
upstream:
  host: proxy.corp.com
  port: 3128

upstreams:
  - host: proxy1.corp.com
    port: 3128
  - host: proxy2.corp.com
    port: 3128

failover:
  retry_interval: 30s
  standalone: false

auth:
  username: myuser
  domain: CORP
  password: mypass
  # auth_type: auto | basic | ntlm | ntlmv2 | ntlm-session
  auth_type: ntlmv2
  pass_nt: ""
  pass_ntlmv2: ""

listen: "127.0.0.1:3128"

cache:
  max_connections: 20
  idle_timeout: "5m"

no_proxy:
  - localhost
  - 127.0.0.*
  - 10.*

gateway:
  enabled: false
  acl:
    allow: []
    deny: []

headers:
  set: {}
  remove: []

socks5:
  listen: ""
  username: ""
  password: ""

log_level: info
```

## CLI usage

```text
Commands:
  serve          Start the HTTP/SOCKS5 proxy
  hashgen        Generate NTLM password hashes
  magic          Probe proxy and detect optimal auth settings

Flags (serve):
  -c string     Config file path (default "local-proxy.yaml")
  -I            Prompt for password interactively
  -L value      TCP tunnel: local_port:remote_host:remote_port
  -d            Daemonize (run in background)
  -p string     PID file path
  -v            Verbose logging
  -f            Run in foreground
```

### Examples

```bash
# Start with config
local-proxy serve -c my-config.yaml

# Prompt for password
local-proxy serve -I

# TCP tunnels
local-proxy serve -L 8080:internal-web:80 -L 8443:internal-api:443

# Daemon with PID file
local-proxy serve -d -p /var/run/proxy.pid

# Generate hashes
local-proxy hashgen -u myuser -d CORP

# Probe upstream proxy
local-proxy magic -proxy proxy.corp.com:3128
```

## Health & metrics

Access these endpoints directly (no proxy target):

```bash
curl http://127.0.0.1:3128/__health
curl http://127.0.0.1:3128/metrics
```

## Development

See [DEVELOPING.md](DEVELOPING.md) for architecture, package layout, and contributing guidelines.

## License

MIT
