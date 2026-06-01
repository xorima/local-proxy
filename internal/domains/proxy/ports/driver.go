package ports

import (
	"context"
	"net"

	proxyModel "local-proxy/internal/domains/proxy/model"
)

type ProxyHandler interface {
	Handle(ctx context.Context, req *proxyModel.ProxyRequest) (*proxyModel.ProxyResponse, error)
	HandleConnect(ctx context.Context, target string) (net.Conn, error)
}
