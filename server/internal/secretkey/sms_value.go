package secretkey

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// SMS keeps its own envelope prefix so SMS ciphertext can never be decrypted by
// the Mail code path even if a key were configured incorrectly.
const smsValuePrefix = "sms:v1:"

func EncryptSMSValue(key []byte, plaintext string) (string, string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return smsValuePrefix + base64.RawURLEncoding.EncodeToString(sealed), "v1", nil
}

func DecryptSMSValue(key []byte, ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, smsValuePrefix) {
		return "", fmt.Errorf("unsupported sms ciphertext version")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, smsValuePrefix))
	if err != nil {
		return "", fmt.Errorf("decode sms ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("sms ciphertext is too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt sms value: %w", err)
	}
	return string(plain), nil
}
