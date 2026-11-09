package secretkey

import (
	"strings"
	"testing"
)

func TestProviderValueRoundTripUsesIndependentEnvelope(t *testing.T) {
	key := []byte(strings.Repeat("p", 32))
	ciphertext, keyID, err := EncryptProviderValue(key, "sk-provider-secret")
	if err != nil {
		t.Fatal(err)
	}
	if keyID != "v1" {
		t.Fatalf("keyID = %q", keyID)
	}
	if !strings.HasPrefix(ciphertext, providerValuePrefix) {
		t.Fatalf("ciphertext = %q", ciphertext)
	}
	plaintext, err := DecryptProviderValue(key, ciphertext)
	if err != nil || plaintext != "sk-provider-secret" {
		t.Fatalf("plaintext = %q, err = %v", plaintext, err)
	}
	other, _, err := EncryptProviderValue([]byte(strings.Repeat("o", 32)), "sk-provider-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecryptProviderValue(key, other); err == nil {
		t.Fatal("provider ciphertext decrypted with a different key")
	}
	if _, err = DecryptPaymentValue(key, ciphertext); err == nil {
		t.Fatal("provider ciphertext accepted by payment decryptor")
	}
}
