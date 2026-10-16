package redeemcode

import (
	"admin/server/internal/secretkey"
	"bytes"
	"strings"
	"testing"
)

func TestRedeemKeysAndCiphertextAreSeparated(t *testing.T) {
	k, e := secretkey.New(strings.Repeat("x", 64))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Equal(k.RedeemCodeEncryptionKey(), k.PaymentEncryptionKey()) || bytes.Equal(k.RedeemCodeHMACKey(), k.RedeemCodeEncryptionKey()) {
		t.Fatal("keys not domain-separated")
	}
	key := k.RedeemCodeHMACKey()
	key[0] ^= 0xff
	if bytes.Equal(key, k.RedeemCodeHMACKey()) {
		t.Fatal("key exposes storage")
	}
	code := "ABCDEFGHJKLMNPQRSTUVWX"
	ct, e := secretkey.EncryptRedeemCodeValue(k.RedeemCodeEncryptionKey(), code)
	if e != nil {
		t.Fatal(e)
	}
	got, e := secretkey.DecryptRedeemCodeValue(k.RedeemCodeEncryptionKey(), ct)
	if e != nil || got != code {
		t.Fatal("cipher round-trip", e)
	}
	if _, e = secretkey.DecryptPaymentValue(k.PaymentEncryptionKey(), ct); e == nil {
		t.Fatal("accepted wrong envelope")
	}
	if _, e = secretkey.DecryptRedeemCodeValue(k.PaymentEncryptionKey(), ct); e == nil {
		t.Fatal("accepted wrong key")
	}
}
