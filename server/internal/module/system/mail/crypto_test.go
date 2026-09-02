package mail

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptSecretRoundTripAndNonce(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	a, _, e := EncryptSecret(key, "123456")
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := EncryptSecret(key, "123456")
	if e != nil || a == b {
		t.Fatal("nonce is not random")
	}
	v, e := DecryptSecret(key, a)
	if e != nil || v != "123456" {
		t.Fatalf("decrypt=%q err=%v", v, e)
	}
	raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(a, "mail:v1:"))
	if e != nil {
		t.Fatal(e)
	}
	raw[len(raw)-1] ^= 1
	tampered := "mail:v1:" + base64.RawURLEncoding.EncodeToString(raw)
	if _, e = DecryptSecret(key, tampered); e == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}
