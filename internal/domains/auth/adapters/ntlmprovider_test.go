package adapters_test

import (
	"testing"

	authModel "local-proxy/internal/domains/auth/model"
	"local-proxy/internal/domains/auth/adapters"
)

func TestNTLMProvider_Header(t *testing.T) {
	t.Run("it should return NTLM Type 1 negotiate on first call", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeV1)
		h := p.Header()
		if len(h) < 10 || h[:5] != "NTLM " {
			t.Errorf("expected NTLM header, got %q", h)
		}
	})

	t.Run("it should return empty on second call", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeV1)
		p.Header()
		h := p.Header()
		if h != "" {
			t.Errorf("expected empty on second call, got %q", h)
		}
	})
}

func TestNTLMProvider_HandleChallenge(t *testing.T) {
	t.Run("it should reject non-NTLM challenge", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeV1)
		_, err := p.HandleChallenge("Basic realm=test")
		if err == nil {
			t.Fatal("expected error for non-NTLM challenge")
		}
	})

	t.Run("it should process NTLM Type 2 challenge and return Type 3 header", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeV1)

		type2 := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		challenge := "NTLM " + adapters.EncodeBase64(type2)

		header, err := p.HandleChallenge(challenge)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(header) < 10 || header[:5] != "NTLM " {
			t.Errorf("expected NTLM header, got %q", header)
		}
	})

	t.Run("it should process NTLMv2 Type 2 challenge", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeV2)

		type2 := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		challenge := "NTLM " + adapters.EncodeBase64(type2)

		header, err := p.HandleChallenge(challenge)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(header) < 10 || header[:5] != "NTLM " {
			t.Errorf("expected NTLM header, got %q", header)
		}
	})

	t.Run("it should process NTLM Session Type 2 challenge", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeSession)

		type2 := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		challenge := "NTLM " + adapters.EncodeBase64(type2)

		header, err := p.HandleChallenge(challenge)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(header) < 10 || header[:5] != "NTLM " {
			t.Errorf("expected NTLM header, got %q", header)
		}
	})

	t.Run("it should support full handshake: Header + HandleChallenge + Header", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeV1)

		h1 := p.Header()
		if h1[:5] != "NTLM " {
			t.Fatalf("expected NTLM negotiate, got %q", h1)
		}

		type2 := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		h2, err := p.HandleChallenge("NTLM " + adapters.EncodeBase64(type2))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h2[:5] != "NTLM " {
			t.Fatalf("expected NTLM auth, got %q", h2)
		}

		h3 := p.Header()
		if h3 != "" {
			t.Errorf("expected empty after auth, got %q", h3)
		}
	})

	t.Run("it should reject malformed base64 challenge", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeV1)
		_, err := p.HandleChallenge("NTLM !!!invalid!!!")
		if err == nil {
			t.Fatal("expected error for malformed base64")
		}
	})

	t.Run("it should reject challenge data that is too short", func(t *testing.T) {
		p := adapters.NewNTLMProvider("user", "pass", "domain", authModel.NTLMModeV1)
		short := adapters.EncodeBase64([]byte("short"))
		_, err := p.HandleChallenge("NTLM " + short)
		if err == nil {
			t.Fatal("expected error for short challenge data")
		}
	})
}
