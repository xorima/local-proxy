package service_test

import (
	"testing"

	authModel "local-proxy/internal/domains/auth/model"
	"local-proxy/internal/domains/auth/service"
)

func TestAuthService_Header(t *testing.T) {
	t.Run("it should return Basic auth header when auth type is Basic", func(t *testing.T) {
		creds := &authModel.Credentials{
			Username: "user",
			Password: "pass",
			AuthType: authModel.AuthBasic,
		}
		svc := service.New(creds)
		got := svc.Header()
		want := "Basic dXNlcjpwYXNz"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("it should return Basic auth header when auth type is unknown (defaults to Basic)", func(t *testing.T) {
		creds := &authModel.Credentials{
			Username: "testuser",
			Password: "secret123",
			AuthType: authModel.AuthType(99),
		}
		svc := service.New(creds)
		got := svc.Header()
		want := "Basic dGVzdHVzZXI6c2VjcmV0MTIz"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("it should return NTLM negotiate header when auth type is NTLM", func(t *testing.T) {
		creds := &authModel.Credentials{
			Username: "user",
			Password: "pass",
			AuthType: authModel.AuthNTLM,
		}
		svc := service.New(creds)
		got := svc.Header()
		if len(got) < 5 || got[:5] != "NTLM " {
			t.Errorf("expected NTLM prefix, got %q", got)
		}
	})

	t.Run("it should return NTLM negotiate header when auth type is NTLMv2", func(t *testing.T) {
		creds := &authModel.Credentials{
			Username: "user",
			Password: "pass",
			AuthType: authModel.AuthNTLMv2,
		}
		svc := service.New(creds)
		got := svc.Header()
		if len(got) < 5 || got[:5] != "NTLM " {
			t.Errorf("expected NTLM prefix, got %q", got)
		}
	})

	t.Run("it should handle empty username", func(t *testing.T) {
		creds := &authModel.Credentials{
			Username: "",
			Password: "pass",
			AuthType: authModel.AuthBasic,
		}
		svc := service.New(creds)
		got := svc.Header()
		want := "Basic OnBhc3M="
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("it should handle empty password", func(t *testing.T) {
		creds := &authModel.Credentials{
			Username: "user",
			Password: "",
			AuthType: authModel.AuthBasic,
		}
		svc := service.New(creds)
		got := svc.Header()
		want := "Basic dXNlcjo="
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
