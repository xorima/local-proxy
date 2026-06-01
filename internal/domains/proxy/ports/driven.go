package ports

import (
	"context"
	"net"

	proxyModel "local-proxy/internal/domains/proxy/model"
)

type UpstreamConnector interface {
	RoundTrip(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error)
	ConnectTunnel(ctx context.Context, target string) (net.Conn, error)
}

type AuthProvider interface {
	Header() string
	HandleChallenge(challenge string) (string, error)
}

type NoProxyMatcher interface {
	Match(target string) bool
}

type CredentialStore interface {
	Get(ctx context.Context, target string) (string, string, error)
}

type ACLMatcher interface {
	Allow(ip string) bool
}

type HeaderModifier interface {
	Modify(headers map[string][]string)
}
