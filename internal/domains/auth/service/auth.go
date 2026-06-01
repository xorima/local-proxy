package service

import (
	"fmt"

	authModel "local-proxy/internal/domains/auth/model"
)

type AuthService struct {
	creds *authModel.Credentials
}

func New(creds *authModel.Credentials) *AuthService {
	return &AuthService{creds: creds}
}

func (s *AuthService) Header() string {
	switch s.creds.AuthType {
	case authModel.AuthBasic:
		return basicHeader(s.creds.Username, s.creds.Password)
	case authModel.AuthNTLM:
		return ntlmHeader()
	case authModel.AuthNTLMv2:
		return ntlmv2Header()
	default:
		return basicHeader(s.creds.Username, s.creds.Password)
	}
}

func basicHeader(username, password string) string {
	raw := fmt.Sprintf("%s:%s", username, password)
	return "Basic " + base64Encode([]byte(raw))
}

func ntlmHeader() string {
	return "NTLM TlRMTVNTUAABAAAAB4IIAAAAAAAAAAAAAAAAAAAAAAA="
}

func ntlmv2Header() string {
	return "NTLM TlRMTVNTUAABAAAAB4IIAAAAAAAAAAAAAAAAAAAAAAA="
}

func base64Encode(data []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte
	for i := 0; i < len(data); i += 3 {
		var b [3]byte
		for j := 0; j < 3 && i+j < len(data); j++ {
			b[j] = data[i+j]
		}
		result = append(result, chars[b[0]>>2])
		result = append(result, chars[(b[0]&0x03)<<4|b[1]>>4])
		if i+1 < len(data) {
			result = append(result, chars[(b[1]&0x0f)<<2|b[2]>>6])
		} else {
			result = append(result, '=')
		}
		if i+2 < len(data) {
			result = append(result, chars[b[2]&0x3f])
		} else {
			result = append(result, '=')
		}
	}
	return string(result)
}
