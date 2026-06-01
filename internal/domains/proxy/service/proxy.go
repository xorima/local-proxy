package service

import (
	"context"
	"net"

	proxyModel "local-proxy/internal/domains/proxy/model"
	"local-proxy/internal/domains/proxy/ports"
)

var _ ports.ProxyHandler = (*ProxyService)(nil)

type ProxyService struct {
	upstream ports.UpstreamConnector
	direct   ports.UpstreamConnector
	auth     ports.AuthProvider
	noproxy  ports.NoProxyMatcher
	header   ports.HeaderModifier
}

func New(upstream ports.UpstreamConnector, auth ports.AuthProvider, noproxy ports.NoProxyMatcher) *ProxyService {
	return &ProxyService{
		upstream: upstream,
		auth:     auth,
		noproxy:  noproxy,
	}
}

func NewWithDirect(upstream, direct ports.UpstreamConnector, auth ports.AuthProvider, noproxy ports.NoProxyMatcher) *ProxyService {
	return &ProxyService{
		upstream: upstream,
		direct:   direct,
		auth:     auth,
		noproxy:  noproxy,
	}
}

func NewFull(upstream, direct ports.UpstreamConnector, auth ports.AuthProvider, noproxy ports.NoProxyMatcher, header ports.HeaderModifier) *ProxyService {
	return &ProxyService{
		upstream: upstream,
		direct:   direct,
		auth:     auth,
		noproxy:  noproxy,
		header:   header,
	}
}

func (s *ProxyService) connector(req *proxyModel.ProxyRequest) ports.UpstreamConnector {
	if s.noproxy.Match(req.Target.Host) && s.direct != nil {
		return s.direct
	}
	return s.upstream
}

func (s *ProxyService) Handle(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error) {
	if s.header != nil {
		s.header.Modify(req.Headers)
	}
	return s.connector(req).RoundTrip(ctx, req)
}

func (s *ProxyService) HandleConnect(ctx context.Context, target string) (net.Conn, error) {
	if s.direct != nil {
		host, _, _ := net.SplitHostPort(target)
		if host != "" && s.noproxy.Match(host) {
			return s.direct.ConnectTunnel(ctx, target)
		}
	}
	return s.upstream.ConnectTunnel(ctx, target)
}
