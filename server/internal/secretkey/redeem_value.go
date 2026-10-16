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

const redeemValuePrefix = "payment-redeem:v1:"

func EncryptRedeemCodeValue(key []byte, plaintext string) (string, error) {
	if plaintext == "" || len(key) != 32 {
		return "", fmt.Errorf("redeem code encryption input invalid")
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(b)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := g.Seal(nonce, nonce, []byte(plaintext), nil)
	return redeemValuePrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func DecryptRedeemCodeValue(key []byte, ciphertext string) (string, error) {
	if len(key) != 32 || !strings.HasPrefix(ciphertext, redeemValuePrefix) {
		return "", fmt.Errorf("redeem code ciphertext invalid")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, redeemValuePrefix))
	if err != nil {
		return "", err
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(b)
	if err != nil {
		return "", err
	}
	if len(raw) < g.NonceSize() {
		return "", fmt.Errorf("redeem code ciphertext too short")
	}
	plain, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
