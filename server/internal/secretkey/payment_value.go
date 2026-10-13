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

const paymentValuePrefix = "payment:v1:"

func EncryptPaymentValue(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", fmt.Errorf("payment value must not be empty")
	}
	if len(key) != 32 {
		return "", fmt.Errorf("payment key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return paymentValuePrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func DecryptPaymentValue(key []byte, ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, paymentValuePrefix) {
		return "", fmt.Errorf("unsupported payment ciphertext version")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, paymentValuePrefix))
	if err != nil {
		return "", fmt.Errorf("decode payment ciphertext: %w", err)
	}
	if len(key) != 32 {
		return "", fmt.Errorf("payment key must be 32 bytes")
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
		return "", fmt.Errorf("payment ciphertext is too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt payment value: %w", err)
	}
	return string(plain), nil
}
