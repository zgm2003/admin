package secretkey

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewDerivesStableSeparatedKeys(t *testing.T) {
	first, err := New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}

	if len(first.JWTSigningKey()) != 32 || len(first.RefreshTokenHMACKey()) != 32 {
		t.Fatal("derived keys must be 32 bytes")
	}
	if !bytes.Equal(first.JWTSigningKey(), second.JWTSigningKey()) {
		t.Fatal("JWT derivation is not stable")
	}
	if !bytes.Equal(first.RefreshTokenHMACKey(), second.RefreshTokenHMACKey()) {
		t.Fatal("Refresh HMAC derivation is not stable")
	}
	if bytes.Equal(first.JWTSigningKey(), first.RefreshTokenHMACKey()) {
		t.Fatal("different purposes produced the same key")
	}
}

func TestNewRejectsUnsafeRootSecrets(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{name: "short", secret: "short"},
		{name: "non-ASCII", secret: strings.Repeat("界", 64)},
		{name: "placeholder", secret: "replace_with_at_least_64_random_characters_before_running_api_server"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.secret); err == nil {
				t.Fatal("New() accepted an unsafe root secret")
			}
		})
	}
}

func TestAccessorsReturnKeyCopies(t *testing.T) {
	keys, err := New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}

	jwtKey := keys.JWTSigningKey()
	refreshKey := keys.RefreshTokenHMACKey()
	jwtKey[0] ^= 0xff
	refreshKey[0] ^= 0xff

	if bytes.Equal(jwtKey, keys.JWTSigningKey()) {
		t.Fatal("JWTSigningKey returned internal storage")
	}
	if bytes.Equal(refreshKey, keys.RefreshTokenHMACKey()) {
		t.Fatal("RefreshTokenHMACKey returned internal storage")
	}
}
