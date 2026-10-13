package secretkey

import (
	"bytes"
	"strings"
	"testing"
)

func TestPaymentEncryptionIsIndependentAndAuthenticated(t *testing.T) {
	keys, err := New(strings.Repeat("p", 64))
	if err != nil {
		t.Fatal(err)
	}
	key := keys.PaymentEncryptionKey()
	if len(key) != 32 || bytes.Equal(key, keys.StorageEncryptionKey()) || bytes.Equal(key, keys.MailEncryptionKey()) {
		t.Fatal("payment key not domain separated")
	}
	key[0] ^= 1
	if bytes.Equal(key, keys.PaymentEncryptionKey()) {
		t.Fatal("key alias")
	}
	encrypted, err := EncryptPaymentValue(keys.PaymentEncryptionKey(), "PRIVATE TEST MATERIAL")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecryptPaymentValue(keys.PaymentEncryptionKey(), encrypted)
	if err != nil || plain != "PRIVATE TEST MATERIAL" {
		t.Fatalf("round trip %q %v", plain, err)
	}
	if _, err := DecryptPaymentValue(keys.StorageEncryptionKey(), encrypted); err == nil {
		t.Fatal("accepted wrong-purpose key")
	}
	for _, raw := range []string{"", "v2:abc", "v1:!", "v1:YWJj"} {
		if _, err := DecryptPaymentValue(keys.PaymentEncryptionKey(), raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if _, err := EncryptPaymentValue(keys.PaymentEncryptionKey(), ""); err == nil {
		t.Fatal("accepted empty key material")
	}
}
