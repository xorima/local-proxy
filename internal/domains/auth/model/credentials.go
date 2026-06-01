package model

type AuthType int

const (
	AuthBasic AuthType = iota
	AuthNTLM
	AuthNTLMv2
	AuthNTLMSession
)

type Credentials struct {
	Username string
	Password string
	Domain   string
	AuthType AuthType
}
