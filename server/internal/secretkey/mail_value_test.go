package secretkey

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestMailValueRoundTripUsesRandomNonceAndRejectsTampering(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	first, _, err := EncryptMailValue(key, "123456")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := EncryptMailValue(key, "123456")
	if err != nil || first == second {
		t.Fatal("mail value nonce is not random")
	}
	value, err := DecryptMailValue(key, first)
	if err != nil || value != "123456" {
		t.Fatalf("decrypt=%q err=%v", value, err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(first, mailValuePrefix))
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	tampered := mailValuePrefix + base64.RawURLEncoding.EncodeToString(raw)
	if _, err = DecryptMailValue(key, tampered); err == nil {
		t.Fatal("tampered mail value accepted")
	}
}
