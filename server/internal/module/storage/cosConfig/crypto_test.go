package cosconfig

import (
	"strings"
	"testing"
)

func TestCredentialCipherRoundTripAndRandomNonce(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	a, err := encryptCredential(key, "secret-id")
	if err != nil {
		t.Fatal(err)
	}
	b, err := encryptCredential(key, "secret-id")
	if err != nil {
		t.Fatal(err)
	}
	if a == b || !strings.HasPrefix(a, "v1:") {
		t.Fatal("ciphertext is not versioned and randomized")
	}
	if got, err := decryptCredential(key, a); err != nil || got != "secret-id" {
		t.Fatalf("decrypt = %q,%v", got, err)
	}
	if _, err := decryptCredential([]byte(strings.Repeat("x", 32)), a); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := encryptCredential(key, ""); err == nil {
		t.Fatal("empty credential accepted")
	}
}
