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

const mailValuePrefix = "mail:v1:"

func EncryptMailValue(key []byte, plaintext string) (string, string, error) {
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
	return mailValuePrefix + base64.RawURLEncoding.EncodeToString(sealed), "v1", nil
}

func DecryptMailValue(key []byte, ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, mailValuePrefix) {
		return "", fmt.Errorf("unsupported mail ciphertext version")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, mailValuePrefix))
	if err != nil {
		return "", fmt.Errorf("decode mail ciphertext: %w", err)
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
		return "", fmt.Errorf("mail ciphertext is too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt mail value: %w", err)
	}
	return string(plain), nil
}
