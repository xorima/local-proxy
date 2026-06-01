package ports

import authModel "local-proxy/internal/domains/auth/model"

type CredentialStore interface {
	Get() (*authModel.Credentials, error)
}
