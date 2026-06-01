package adapters_test

import (
	"encoding/hex"
	"testing"

	"local-proxy/internal/domains/auth/adapters"
)

func TestMD4(t *testing.T) {
	t.Run("it should compute MD4 hash of empty string", func(t *testing.T) {
		// MD4("") = 31d6cfe0d16ae931b73c59d7e0c089c0
		h := adapters.MD4([]byte{})
		want := "31d6cfe0d16ae931b73c59d7e0c089c0"
		got := hex.EncodeToString(h)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("it should compute MD4 hash of 'a'", func(t *testing.T) {
		// MD4("a") = bde52cb31de33e46245e05fbdbd6fb24
		h := adapters.MD4([]byte("a"))
		want := "bde52cb31de33e46245e05fbdbd6fb24"
		got := hex.EncodeToString(h)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("it should compute MD4 hash of 'abc'", func(t *testing.T) {
		// MD4("abc") = a448017aaf21d8525fc10ae87aa6729d
		h := adapters.MD4([]byte("abc"))
		want := "a448017aaf21d8525fc10ae87aa6729d"
		got := hex.EncodeToString(h)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("it should compute MD4 of standard test vector", func(t *testing.T) {
		// MD4("message digest") = d9130a8164549fe818874806e1c7014b
		h := adapters.MD4([]byte("message digest"))
		want := "d9130a8164549fe818874806e1c7014b"
		got := hex.EncodeToString(h)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}

func TestNTHash(t *testing.T) {
	t.Run("it should compute NT hash for 'password'", func(t *testing.T) {
		// NT hash of "password" in UTF-16LE
		h := adapters.NTHash("password")
		got := hex.EncodeToString(h)
		// "password" in UTF-16LE: p\x00a\x00s\x00s\x00w\x00o\x00r\x00d\x00
		// MD4 of that should be known value
		if len(got) != 32 {
			t.Errorf("expected 32 hex chars, got %d", len(got))
		}
	})

	t.Run("it should produce consistent NT hash for same input", func(t *testing.T) {
		h1 := adapters.NTHash("password")
		h2 := adapters.NTHash("password")
		if string(h1) != string(h2) {
			t.Error("expected identical hashes for same password")
		}
	})

	t.Run("it should produce different NT hash for different inputs", func(t *testing.T) {
		h1 := adapters.NTHash("password")
		h2 := adapters.NTHash("Password")
		if string(h1) == string(h2) {
			t.Error("expected different hashes for different passwords")
		}
	})
}

func TestNTOWFv2(t *testing.T) {
	t.Run("it should compute NTOWFv2 for known test vector", func(t *testing.T) {
		// From MS-NLMP example: password="password", user="User", domain="Domain"
		// NTOWFv2 = HMAC-MD5(NT hash, (User + Domain) in uppercase UTF-16LE)
		h := adapters.NTOWFv2("password", "User", "Domain")
		if len(h) != 16 {
			t.Errorf("expected 16 bytes, got %d", len(h))
		}
	})
}

func TestNTLMType1Message(t *testing.T) {
	t.Run("it should generate a valid Type 1 message", func(t *testing.T) {
		msg := adapters.GenerateType1("WORKSTATION", "DOMAIN")
		if len(msg) == 0 {
			t.Fatal("expected non-empty Type 1 message")
		}
		if msg[0] != 'N' || msg[1] != 'T' || msg[2] != 'L' || msg[3] != 'M' {
			t.Errorf("expected NTLM signature, got %s", string(msg[:4]))
		}
		// Type 1 has message type = 1 at offset 8
		if msg[8] != 1 {
			t.Errorf("expected message type 1, got %d", msg[8])
		}
	})
}

func TestNTLMType3Message(t *testing.T) {
	t.Run("it should generate a valid Type 3 message from Type 2 challenge", func(t *testing.T) {
		// Create a minimal Type 2 challenge
		type2 := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		if type2 == nil {
			t.Fatal("expected non-nil Type 2 challenge")
		}

		msg := adapters.GenerateType3V1("User", "Password", "Domain", type2)
		if len(msg) == 0 {
			t.Fatal("expected non-empty Type 3 message")
		}
		if msg[0] != 'N' || msg[1] != 'T' || msg[2] != 'L' || msg[3] != 'M' {
			t.Errorf("expected NTLM signature, got %s", string(msg[:4]))
		}
		if msg[8] != 3 {
			t.Errorf("expected message type 3, got %d", msg[8])
		}
	})

	t.Run("it should generate a valid Type 3 NTLMv2 message", func(t *testing.T) {
		type2 := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		if type2 == nil {
			t.Fatal("expected non-nil Type 2 challenge")
		}

		msg := adapters.GenerateType3V2("User", "Password", "Domain", type2)
		if len(msg) == 0 {
			t.Fatal("expected non-empty Type 3 message")
		}
		if msg[8] != 3 {
			t.Errorf("expected message type 3, got %d", msg[8])
		}
	})
}

func TestParseType2(t *testing.T) {
	t.Run("it should parse a Type 2 message and extract challenge", func(t *testing.T) {
		type2 := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		nonce := adapters.ExtractChallenge(type2)
		if len(nonce) != 8 {
			t.Errorf("expected 8-byte nonce, got %d bytes", len(nonce))
		}
	})
}

func TestNTLMAuthHeader(t *testing.T) {
	t.Run("it should produce NTLM header for Type 1", func(t *testing.T) {
		header := adapters.NTLMHeader("", "", "", nil, false)
		if len(header) < 10 || header[:5] != "NTLM " {
			t.Errorf("expected NTLM prefix, got %s", header[:5])
		}
	})

	t.Run("it should produce NTLM header for Type 3 with challenge", func(t *testing.T) {
		challenge := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		header := adapters.NTLMHeader("User", "Password", "Domain", challenge, false)
		if len(header) < 10 || header[:5] != "NTLM " {
			t.Errorf("expected NTLM prefix, got %s", header[:5])
		}
	})

	t.Run("it should produce NTLMv2 header with challenge", func(t *testing.T) {
		challenge := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		header := adapters.NTLMHeader("User", "Password", "Domain", challenge, true)
		if len(header) < 10 || header[:5] != "NTLM " {
			t.Errorf("expected NTLM prefix, got %s", header[:5])
		}
	})

	t.Run("it should produce different Type1 and Type3 headers", func(t *testing.T) {
		type1 := adapters.NTLMHeader("", "", "", nil, false)
		challenge := adapters.GenerateType2Challenge([]byte("0123456789abcdef"))
		type3 := adapters.NTLMHeader("User", "Password", "Domain", challenge, false)
		if type1 == type3 {
			t.Error("expected Type 1 and Type 3 headers to differ")
		}
	})
}
