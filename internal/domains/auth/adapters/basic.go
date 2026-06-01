package adapters

import (
	authModel "local-proxy/internal/domains/auth/model"
	"local-proxy/internal/domains/auth/service"
)

type BasicProvider struct {
	svc *service.AuthService
}

func NewBasicProvider(username, password string) *BasicProvider {
	creds := &authModel.Credentials{
		Username: username,
		Password: password,
		AuthType: authModel.AuthBasic,
	}
	return &BasicProvider{svc: service.New(creds)}
}

func (p *BasicProvider) Header() string {
	return p.svc.Header()
}

func (p *BasicProvider) HandleChallenge(_ string) (string, error) {
	return p.svc.Header(), nil
}
