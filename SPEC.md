# local-proxy — Spec-Driven Development

## Overview

`local-proxy` is a Go rewrite of [cntlm](https://cntlm.sourceforge.net/) — an NTLM/NTLMv2 authenticating HTTP(S) proxy with TCP/IP tunneling, connection caching, and parent-proxy failover. It sits between local applications and a corporate (or test) proxy that requires authentication, handling the auth handshake transparently.

## Test Environment

A Docker-based Squid proxy requiring Basic auth (`user:pass`) runs alongside development to validate behavior.

- **Location:** `test-env/`
- **Start:** `docker compose up -d` in `test-env/`
- **Proxy URL:** `http://localhost:13128`
- **Credentials:** `user:pass`
- **Verify:** `./test.sh`

---

## Capability Spec (Implementation Order)

### Phase 1 — Core Proxy (MVP)

| # | Capability | Priority | Description |
|---|-----------|----------|-------------|
| 1.1 | **HTTP Forward Proxy** | P0 | Listen on localhost:3128, forward HTTP requests to an upstream (parent) proxy. Handle `CONNECT` for HTTPS tunneling. |
| 1.2 | **Parent Proxy Config** | P0 | Single parent proxy defined via config file (`Proxy host:port`) or CLI args. |
| 1.3 | **Basic Auth Upstream** | P0 | Authenticate to the parent proxy using `Proxy-Authorization: Basic` with configurable username/password. |
| 1.4 | **Config File** | P0 | TOML or YAML config file. Default path `~/.local-proxy.toml` or `local-proxy.toml` in CWD. Override with `-c`. |
| 1.5 | **Connection Caching** | P1 | Reuse authenticated connections to the parent proxy. Cache by upstream host:port. Idle timeout and max connections configurable. |
| 1.6 | **Logging** | P1 | Structured logging (JSON) to stdout/stderr. Levels: debug, info, warn, error. |

### Phase 2 — Authentication & Security

| # | Capability | Priority | Description |
|---|-----------|----------|-------------|
| 2.1 | **NTLM Authentication** | P0 | NTLM handshake (Type 1/2/3 messages) for authenticating to the parent proxy. |
| 2.2 | **NTLMv2 Authentication** | P0 | NTLMv2 handshake (more secure than NTLM). Default if not specified. |
| 2.3 | **NTLM Session Response** | P1 | Legacy NTLM session response mode. |
| 2.4 | **Auth Type Selection** | P1 | Configurable auth type: `ntlm`, `ntlmv2`, `ntlm-session`, `basic`. Auto-detect using magic mode. |
| 2.5 | **Password Hashing** | P1 | Store passwords as hashes (PassLM, PassNT, PassNTLMv2) instead of plaintext in config. |
| 2.6 | **Interactive Password Prompt** | P1 | Prompt for password at startup (-I flag), ignore password from config. |
| 2.7 | **Password Memory Protection** | P2 | Zero out plaintext password from memory after hashing. |

### Phase 3 — Multi-Proxy & Failover

| # | Capability | Priority | Description |
|---|-----------|----------|-------------|
| 3.1 | **Multiple Parent Proxies** | P1 | Define multiple `Proxy` lines in config. Try each in order until one succeeds. |
| 3.2 | **Circular Failover** | P1 | Rotate through parent proxies in a circular buffer. Mark failed proxies with a cooldown period. |
| 3.3 | **Automatic Standalone Fallback** | P2 | When all parent proxies are unreachable, serve requests directly (no parent proxy). Recheck parents periodically. |

### Phase 4 — Access Control & Routing

| # | Capability | Priority | Description |
|---|-----------|----------|-------------|
| 4.1 | **NoProxy List** | P1 | Bypass parent proxy for matching destination hosts/wildcards. Support `*` and `?` patterns. Connect directly instead. |
| 4.2 | **Gateway Mode** | P2 | Listen on all network interfaces (not just loopback). Allow LAN clients. |
| 4.3 | **Allow/Deny ACLs** | P2 | IP-based access control for gateway mode. Allow and Deny rules with CIDR support. |
| 4.4 | **Header Replacement** | P2 | Replace or modify request headers before forwarding (e.g., `User-Agent`). |

### Phase 5 — Tunneling & SOCKS

| # | Capability | Priority | Description |
|---|-----------|----------|-------------|
| 5.1 | **TCP/IP Tunneling** | P2 | Port forwarding through the proxy: `-L local_port:remote_host:remote_port`. Opens a local listener and tunnels connections through the authenticated proxy. |
| 5.2 | **Multiple Tunnels** | P2 | Support multiple simultaneous tunnels. |
| 5.3 | **SOCKS5 Proxy** | P3 | SOCKS5 interface alongside HTTP proxy. Configurable port. |
| 5.4 | **SOCKS5 Authentication** | P3 | Username/password authentication for SOCKS5 connections. |

### Phase 6 — Diagnostics & Tooling

| # | Capability | Priority | Description |
|---|-----------|----------|-------------|
| 6.1 | **Hash Generation Mode** | P1 | `local-proxy -H -u user -d domain` — generate password hashes (PassLM, PassNT, PassNTLMv2) from interactive password prompt and print config snippet. |
| 6.2 | **Magic Detection Mode** | P2 | `local-proxy -M` — probe the parent proxy to detect optimal NTLM settings (auth type, flags). |
| 6.3 | **Verbose/Debug Logging** | P1 | `-v` flag for verbose output showing each request, auth handshake, and upstream response. |
| 6.4 | **Foreground Mode** | P1 | `-f` flag — run in foreground, log to stderr (default: daemonize/background). |
| 6.5 | **Health Check Endpoint** | P2 | HTTP endpoint (e.g., `/__health`) on the proxy port for monitoring. |
| 6.6 | **Prometheus Metrics** | P3 | `/metrics` endpoint with request counts, cache hit rates, upstream response times. |

### Phase 7 — Platform & Hardening

| # | Capability | Priority | Description |
|---|-----------|----------|-------------|
| 7.1 | **Daemonization** | P2 | Fork to background on Linux/macOS. Windows service support. |
| 7.2 | **Signal Handling** | P2 | Graceful shutdown on SIGTERM/SIGINT. Reload config on SIGHUP. |
| 7.3 | **Pid File** | P2 | Write PID file to prevent multiple instances. |
| 7.4 | **Cross-Platform** | P0 | Build and run on macOS (arm64/amd64) and Windows (amd64). One codebase. |

---

## Architecture

```
┌──────────────┐     ┌─────────────────────────────────────────────────────────┐     ┌──────────────┐
│   Browser    │────▶│                    local-proxy                         │────▶│    Squid     │
│   curl, etc  │     │  ┌─────────────────────────────────────────────────┐   │     │  (test-env)  │
└──────────────┘     │  │  ┌──────────────┐                               │   │     │   :13128     │
                     │  │  │ controller/  │  ports/driver.go              │   │     └──────────────┘
                     │  │  │  http.go ────▶│  ProxyHandler interface      │   │
                     │  │  │  (driving     │  ┌────────────────────────┐  │   │
                     │  │  │   adapter)    │  │   service/proxy.go    │  │   │
                     │  │  └──────────────┘  │   ProxyService{        │  │   │
                     │  │                     │     upstream,          │  │   │
                     │  │     ports/driven.go │     auth,              │  │   │
                     │  │  ┌────────────────┐ │     noproxy            │  │   │
                     │  │  │ UpstreamConn   │ │   }                    │  │   │
                     │  │  │ AuthProvider   │◀┘                        │  │   │
                     │  │  │ NoProxyMatcher │    └────────────────────────┘  │   │
                     │  │  │ CredentialStore│                               │   │
                     │  │  └───────┬────────┘                               │   │
                     │  │          │                                         │   │
                     │  │  ┌───────▼────────────────────────────┐            │   │
                     │  │  │        Adapters / Clients          │            │   │
                     │  │  │  ┌───────────┐ ┌────────────────┐  │            │   │
                     │  │  │  │clients/   │ │domains/auth/   │  │            │   │
                     │  │  │  │upstream   │ │adapters/basic  │  │            │   │
                     │  │  │  │  client.go│ │  BasicProvider │  │            │   │
                     │  │  │  └───────────┘ └────────────────┘  │            │   │
                     │  │  │  ┌──────────────────────────────┐  │            │   │
                     │  │  │  │domains/proxy/adapters/       │  │            │   │
                     │  │  │  │  noproxy.go (NoProxyMatcher) │  │            │   │
                     │  │  │  └──────────────────────────────┘  │            │   │
                     │  │  └────────────────────────────────────┘            │   │
                     │  └─────────────────────────────────────────────────┘   │
                     │                                                       │
                     │  internal/app/app.go — composition root (wiring)      │
                     │  cmd/serve/main.go  — entry point (no logic)           │
                     └─────────────────────────────────────────────────────────┘
```

### Package Layout

```
local-proxy/
├── main.go                                # Root dispatcher (delegates to cmd/*)
├── go.mod / go.sum
├── cmd/
│   ├── serve/main.go                      # Entry point — calls internal/app
│   ├── hashgen/main.go                    # Hash generation subcommand stub
│   └── magic/main.go                      # Magic detection subcommand stub
├── internal/
│   ├── app/
│   │   ├── app.go                         # Composition root (wires all deps)
│   │   └── model.go                       # App-level config types
│   ├── domains/
│   │   ├── proxy/
│   │   │   ├── model/                     # Target, ProxyRequest, ProxyResponse
│   │   │   ├── ports/
│   │   │   │   ├── driver.go              # ProxyHandler (driving port)
│   │   │   │   └── driven.go              # UpstreamConnector, AuthProvider, etc.
│   │   │   ├── service/
│   │   │   │   └── proxy.go              # ProxyService — business logic
│   │   │   ├── controller/
│   │   │   │   └── http.go               # HTTP driving adapter
│   │   │   └── adapters/
│   │   │       └── noproxy.go            # NoProxy pattern matcher
│   │   ├── auth/
│   │   │   ├── model/                     # Credentials, AuthType
│   │   │   ├── ports/
│   │   │   │   └── driven.go              # CredentialStore interface
│   │   │   ├── service/
│   │   │   │   └── auth.go               # AuthService — header generation
│   │   │   ├── adapters/
│   │   │   │   └── basic.go              # BasicProvider adapter
│   │   │   └── controller/               # (future: auth UI/CLI)
│   │   │
│   │   └── tunnel/                        # Phase 5: TCP tunneling stubs
│   │       ├── model/
│   │       ├── ports/
│   │       ├── service/
│   │       ├── adapters/
│   │       └── controller/
│   │
│   └── clients/
│       ├── upstream/
│       │   ├── client.go                  # Upstream proxy HTTP client (driven adapter)
│       │   ├── client_test.go
│       │   └── model.go
│       └── config/
│           ├── client.go                  # Config file reader
│           ├── client_test.go
│           └── model.go
├── test-env/                              # Docker test environment
│   ├── docker-compose.yml
│   ├── Dockerfile.squid
│   ├── squid.conf
│   ├── htpasswd
│   ├── test.sh
│   └── local-proxy.yaml                   # Default config pointing at test Squid
├── local-proxy.yaml                       # Default config (used if -c not given)
└── SPEC.md                                # This file
```

---

## Key Design Decisions

### Architecture Style
- **Domain-Driven Design (DDD)** — bounded contexts: `proxy`, `auth`, `tunnel`
- **Hexagonal Architecture** — ports (interfaces) + adapters (implementations)
- **Test-Driven Development (TDD)** — red-green-refactor per unit; BDD-style test naming (`it should do X when Y`)
- All domain logic is testable via mocks at port boundaries

### Layer Definitions per Domain
| Layer | Role |
|-------|------|
| `model/` | Domain entities, value objects |
| `ports/` | Interfaces (driving = what the domain provides, driven = what the domain needs) |
| `service/` | Business logic implementation of the driving port |
| `controller/` | Driving adapter — translates external input (HTTP, CLI) into domain calls |
| `adapters/` | Driven adapter implementations (e.g., Basic auth, NoProxy matcher) |

### Future Considerations
- **Desktop UI/webserver** — Phase 6+ will add a local web UI showing request logs, metrics, and OpenTelemetry export
- **Per-URL credentials** — CredentialStore port to support different creds per target (e.g., nexus vs. general web)
- **OS credential manager** — macOS Keychain / Windows Credential Manager integration (no plaintext passwords in config)
- **Connection caching** — Pooled authenticated connections to upstream (Phase 1.5)

### NTLM Implementation

- Use Go's `crypto/des`, `crypto/md4`, `crypto/hmac` for NTLM hash primitives (no external DLLs).
- Implement NTLM Type 1 (Negotiate), Type 2 (Challenge), Type 3 (Authenticate) message construction.
- NTLMv2 uses HMAC-MD5 with the server challenge and a timestamp-based client nonce.

### Connection Caching

- Cache keyed by `(upstream_host:port, auth_type)`.
- Idle connections kept alive with periodic health checks.
- Configurable pool size and idle timeout.
- On auth failure, invalidate the cached connection and retry.

### Failover Strategy

- Parent proxies tried in order; on failure, move to next.
- After all proxies fail, wait for a configurable retry interval before re-trying from the beginning.
- If `standalone` fallback is enabled and all proxies are down, go direct.

### Config File Format

TOML (simpler, well-supported in Go via `BurntSushi/toml` or `pelletier/go-toml`):

```toml
# Parent proxy
[upstream]
host = "localhost"
port = 13128

# Authentication
[auth]
username = "user"
domain = ""
# Use hashes instead of plaintext when possible
password = "pass"
# Or use pre-computed hashes:
# pass_ntlmv2 = "..."

# Listening
listen = "127.0.0.1:3128"

# Connection caching
[cache]
max_connections = 20
idle_timeout = "5m"

# Bypass list (direct connect)
no_proxy = [
  "localhost",
  "127.0.0.*",
  "10.*",
  "192.168.*"
]
```

### Testing Strategy

1. **Unit tests** for auth handshake, cache logic, config parsing.
2. **Integration tests** using the Docker Squid test environment.
3. **End-to-end tests** — run `local-proxy`, route curl through it, verify auth to Squid and successful HTTP response.

---

## Implementation Roadmap

```
Week 1  Phase 1: Core proxy — HTTP forward, parent proxy, Basic auth, config file
Week 2  Phase 2: NTLM/NTLMv2 auth, password hashing, interactive prompt
Week 3  Phase 3: Multi-proxy failover, standalone fallback
Week 4  Phase 4: NoProxy, gateway mode, ACLs, header replacement
Week 5  Phase 5: TCP tunneling, SOCKS5
Week 6+  Phase 6-7: Diagnostics, daemonization, hardening
```

---

## Acceptance Criteria

- HTTP requests through `local-proxy` to the test Squid return 200.
- HTTPS requests via `CONNECT` tunneling work.
- Unauthenticated requests return 407.
- Wrong credentials return 407.
- NTLM/NTLMv2 authentication works against a real parent proxy.
- Multiple parent proxies with failover work.
- Connection caching reduces auth handshake overhead.
- Password hashes can be generated and used in place of plaintext.
- Cross-platform: builds and runs on macOS (arm64) and Windows (amd64).
