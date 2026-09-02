package mail

import (
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
	if _, e = DecryptSecret(key, a[:len(a)-1]+"x"); e == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}
