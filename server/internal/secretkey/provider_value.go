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

const providerValuePrefix = "ai-provider:v1:"

func EncryptProviderValue(key []byte, plaintext string) (string, string, error) {
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
	return providerValuePrefix + base64.RawURLEncoding.EncodeToString(sealed), "v1", nil
}

func DecryptProviderValue(key []byte, ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, providerValuePrefix) {
		return "", fmt.Errorf("unsupported AI provider ciphertext version")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, providerValuePrefix))
	if err != nil {
		return "", fmt.Errorf("decode AI provider ciphertext: %w", err)
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
		return "", fmt.Errorf("AI provider ciphertext is too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt AI provider value: %w", err)
	}
	return string(plain), nil
}
