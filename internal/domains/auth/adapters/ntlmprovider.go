package adapters

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	authModel "local-proxy/internal/domains/auth/model"
)

type NTLMProvider struct {
	user     string
	password string
	domain   string
	mode     authModel.NTLMMode
	state    authModel.NTLMState
	ntHash   []byte
}

func NewNTLMProviderWithHash(user, domain string, ntHash []byte, ntlmv2Hash []byte, mode authModel.NTLMMode) *NTLMProvider {
	p := &NTLMProvider{
		user:   user,
		domain: domain,
		mode:   mode,
		state:  authModel.NTLMInit,
		ntHash: ntHash,
	}
	_ = ntlmv2Hash
	return p
}

func (p *NTLMProvider) getNTHash() []byte {
	if len(p.ntHash) > 0 {
		return p.ntHash
	}
	if p.password != "" {
		return NTHash(p.password)
	}
	return nil
}

func hmacMD5(key, data []byte) []byte {
	mac := hmac.New(md5.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func NewNTLMProvider(user, password, domain string, mode authModel.NTLMMode) *NTLMProvider {
	return &NTLMProvider{
		user:   user,
		password: password,
		domain: domain,
		mode:   mode,
		state:  authModel.NTLMInit,
	}
}

func (p *NTLMProvider) Header() string {
	if p.state != authModel.NTLMInit {
		return ""
	}
	p.state = authModel.NTLMNegotiate
	msg := GenerateType1("", p.domain)
	return "NTLM " + base64.StdEncoding.EncodeToString(msg)
}

func (p *NTLMProvider) HandleChallenge(challenge string) (string, error) {
	if !strings.HasPrefix(challenge, "NTLM ") {
		return "", fmt.Errorf("unexpected auth challenge scheme")
	}

	data, err := base64.StdEncoding.DecodeString(challenge[5:])
	if err != nil {
		return "", fmt.Errorf("decode NTLM challenge: %w", err)
	}

	if len(data) < 24 {
		return "", fmt.Errorf("invalid NTLM challenge: too short")
	}

	if data[8] != 2 {
		return "", fmt.Errorf("expected Type 2 message, got type %d", data[8])
	}

	p.state = authModel.NTLMAuthenticate

	msg := p.buildType3(data)
	return "NTLM " + base64.StdEncoding.EncodeToString(msg), nil
}

func (p *NTLMProvider) buildType3(type2Msg []byte) []byte {
	if len(type2Msg) < 32 {
		return nil
	}
	challenge := type2Msg[24:32]

	switch p.mode {
	case authModel.NTLMModeV2:
		return p.buildType3V2(challenge)
	case authModel.NTLMModeSession:
		return p.buildType3Session(challenge)
	default:
		return p.buildType3V1(challenge)
	}
}

func (p *NTLMProvider) buildType3V1(challenge []byte) []byte {
	ntHash := p.getNTHash()
	if ntHash == nil {
		return nil
	}
	lmResp := lmResponse(ntHash, challenge)
	ntResp := lmResponse(ntHash, challenge)
	return buildType3(p.user, p.domain, lmResp, ntResp)
}

func (p *NTLMProvider) buildType3V2(challenge []byte) []byte {
	ntHash := p.getNTHash()
	if ntHash == nil {
		return nil
	}

	clientNonce := make([]byte, 8)
	rand.Read(clientNonce)

	ntowfv2 := hmacMD5(ntHash, utf16le(strings.ToUpper(p.user)+p.domain))
	ntResp := buildNTLMv2Blob(ntowfv2, challenge, clientNonce)

	lmResp := make([]byte, 24)
	copy(lmResp, clientNonce)

	return buildType3(p.user, p.domain, lmResp, ntResp)
}

func (p *NTLMProvider) buildType3Session(challenge []byte) []byte {
	ntHash := p.getNTHash()
	if ntHash == nil {
		return nil
	}

	clientNonce := make([]byte, 8)
	rand.Read(clientNonce)

	mixedChallenge := hmacMD5(challenge, clientNonce)[:8]
	ntResp := lmResponse(ntHash, mixedChallenge)

	lmResp := make([]byte, 24)
	copy(lmResp, clientNonce)

	return buildType3(p.user, p.domain, lmResp, ntResp)
}
