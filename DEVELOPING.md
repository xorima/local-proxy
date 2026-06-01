# Developing local-proxy

## Architecture

Hexagonal (ports & adapters) architecture with Domain-Driven Design. Each domain has its own bounded context with isolated model, ports, service, and adapters.

```
┌──────────────┐     ┌──────────────────────────────────────────┐     ┌──────────────┐
│   Browser    │────▶│              local-proxy                 │────▶│    Squid     │
│   curl, etc  │     │  ┌────────────────────────────────────┐  │     │  (test-env)  │
└──────────────┘     │  │        controller/http.go          │  │     └──────────────┘
                     │  │        (driving adapter)            │  │
                     │  └──────────────┬─────────────────────┘  │
                     │                 │                        │
                     │  ┌──────────────▼─────────────────────┐  │
                     │  │        service/proxy.go            │  │
                     │  │        ProxyService — business     │  │
                     │  │        logic                       │  │
                     │  └──────────────┬─────────────────────┘  │
                     │                 │                        │
                     │  ┌──────────────▼─────────────────────┐  │
                     │  │    ports/ — interfaces             │  │
                     │  │    driven.go: UpstreamConnector,   │  │
                     │  │    AuthProvider, NoProxyMatcher    │  │
                     │  └──────────────┬─────────────────────┘  │
                     │                 │                        │
                     │  ┌──────────────▼─────────────────────┐  │
                     │  │     adapters / clients              │  │
                     │  │  ┌──────────┐ ┌──────────────────┐  │  │
                     │  │  │upstream/ │ │auth/adapters/    │  │  │
                     │  │  │client.go │ │ntlmprovider.go  │  │  │
                     │  │  └──────────┘ │basic.go          │  │  │
                     │  │               └──────────────────┘  │  │
                     │  │  ┌──────────┐ ┌──────────────────┐  │  │
                     │  │  │direct/   │ │proxy/adapters/   │  │  │
                     │  │  │client.go │ │noproxy.go        │  │  │
                     │  │  └──────────┘ │acl.go            │  │  │
                     │  │  ┌──────────┐ │headers.go        │  │  │
                     │  │  │failover/ │ │socks5.go         │  │  │
                     │  │  │client.go │ └──────────────────┘  │  │
                     │  │  └──────────┘                       │  │
                     │  │  ┌──────────┐                       │  │
                     │  │  │ntlmmock/ │ (test mock server)    │  │
                     │  │  └──────────┘                       │  │
                     │  └────────────────────────────────────┘  │
                     │                                          │
                     │  internal/app/app.go — composition root  │
                     │  cmd/serve/main.go — entry point          │
                     └──────────────────────────────────────────┘
```

## Package layout

```
local-proxy/
├── main.go                        # Root dispatcher
├── cmd/
│   ├── serve/main.go              # Proxy server entry point
│   ├── hashgen/main.go            # Hash generation subcommand
│   └── magic/main.go             # Magic detection subcommand
├── internal/
│   ├── app/
│   │   ├── app.go                 # Composition root (wires all deps)
│   │   └── model.go               # App-level config (CLI flags)
│   ├── domains/
│   │   ├── proxy/                 # Core proxy domain
│   │   │   ├── model/             # ProxyRequest, ProxyResponse, Target
│   │   │   ├── ports/
│   │   │   │   ├── driver.go      # ProxyHandler (driving port)
│   │   │   │   └── driven.go      # UpstreamConnector, AuthProvider, etc.
│   │   │   ├── service/
│   │   │   │   └── proxy.go       # ProxyService — request routing
│   │   │   ├── controller/
│   │   │   │   └── http.go        # HTTP driving adapter (+ health/metrics)
│   │   │   └── adapters/
│   │   │       ├── noproxy.go     # NoProxy pattern matcher
│   │   │       ├── acl.go         # CIDR-based allow/deny ACL
│   │   │       ├── headers.go     # Header set/remove modifier
│   │   │       └── socks5.go      # SOCKS5 server
│   │   ├── auth/
│   │   │   ├── model/             # Credentials, NTLMMode enum
│   │   │   ├── ports/             # CredentialStore interface
│   │   │   ├── service/           # Auth header generation
│   │   │   └── adapters/
│   │   │       ├── ntlm.go        # NTLM hash, Type 1/2/3 messages
│   │   │       ├── ntlmprovider.go # NTLMProvider (Header + HandleChallenge)
│   │   │       └── basic.go       # BasicProvider
│   │   └── tunnel/
│   │       └── tunnel.go          # TCP port forwarding via CONNECT
│   ├── clients/
│   │   ├── config/
│   │   │   ├── client.go          # YAML config loader
│   │   │   └── model.go           # Config structs
│   │   ├── upstream/
│   │   │   ├── client.go          # Upstream proxy HTTP client
│   │   │   └── model.go           # Upstream config
│   │   ├── direct/
│   │   │   └── client.go          # Direct connection (no parent proxy)
│   │   ├── failover/
│   │   │   └── client.go          # Multi-proxy round-robin + cooldown
│   │   └── ntlmmock/
│   │       └── server.go          # Mock NTLM proxy for tests
│   └── ...
├── test-env/                      # Docker test environment (Squid)
├── .github/workflows/
│   ├── ci.yml                     # CI: lint, vulncheck, tests, build
│   └── release.yml                # Release: release-please + artifacts
├── AGENTS.md                      # AI agent rules (lint, test, etc.)
├── README.md
├── DEVELOPING.md
└── SPEC.md                        # Full capability spec
```

## Domain layer conventions

| Layer | Role |
|-------|------|
| `model/` | Domain entities and value objects |
| `ports/` | Interfaces — driving (what the domain provides) and driven (what the domain needs) |
| `service/` | Business logic implementing the driving port |
| `controller/` | Driving adapter — translates external input (HTTP, CLI) into domain calls |
| `adapters/` | Driven adapter implementations (e.g., Basic auth, NoProxy, ACL) |

Clients go in `internal/clients/` when they cross domain boundaries (e.g., `upstream` is used by the `proxy` domain). The composition root in `internal/app/app.go` wires everything together.

## Testing

### Running tests

```bash
# Unit tests
go test ./internal/...

# Integration tests (requires Docker)
cd test-env && docker compose up -d && cd ..
go test -tags=integration -timeout 30s ./tests/

# All tests
go clean -testcache && go test ./...
```

### Pre-submit checklist

```bash
golangci-lint run ./...
govulncheck ./...
go vet ./...
go test ./internal/...
```

These checks are enforced by CI (`.github/workflows/ci.yml`) and documented in `AGENTS.md`.

### Test conventions

- **BDD-style naming**: `it should do X when Y`
- **Unit tests** use mocks at port boundaries
- **Integration tests** use `//go:build integration` tag and require Docker
- **Mock server** in `internal/clients/ntlmmock/` simulates a full NTLM handshake

## Key design decisions

- **NTLMMode enum** (V1, V2, Session) instead of a boolean for clarity
- **AuthProvider interface** with `Header()` + `HandleChallenge(challenge)` for both Basic (stateless) and NTLM (stateful) auth
- **Upstream client** handles 407 retry internally — proxy service never sees auth challenges
- **Failover client** wraps multiple upstream clients with round-robin, cooldown tracking, and optional standalone fallback
- **Direct client** connects without a parent proxy (used for NoProxy bypass and standalone fallback)
- **SOCKS5** supports RFC 1928 with username/password auth (RFC 1929)

## Release process

Commits must follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add TCP tunneling
fix: handle connection reset in upstream client
feat!: change config format (breaking)
```

On push to `main`, `release-please` creates/updates a release PR with auto-generated changelog. Merging that PR triggers a GitHub Release with cross-compiled binaries.
