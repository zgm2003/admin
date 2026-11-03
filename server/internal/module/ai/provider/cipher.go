package provider

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// Cipher seals AI credentials under a dedicated HKDF purpose and authenticated envelope.
type Cipher struct {
	key []byte
	id  string
}

func NewCipher(root, keyID string) (*Cipher, error) {
	if len(root) < 64 || len(keyID) == 0 || len(keyID) > 64 {
		return nil, ErrInvalidConfiguration
	}
	key, err := hkdf.Key(sha256.New, []byte(root), nil, "admin:ai:provider-credential-encryption:v1", 32)
	if err != nil {
		return nil, errors.New("derive AI credential key")
	}
	return &Cipher{key: key, id: keyID}, nil
}
func (c *Cipher) Seal(value string) (string, string, error) {
	if c == nil || value == "" {
		return "", "", ErrInvalidConfiguration
	}
	b, err := aes.NewCipher(c.key)
	if err != nil {
		return "", "", ErrInvalidConfiguration
	}
	g, err := cipher.NewGCM(b)
	if err != nil {
		return "", "", ErrInvalidConfiguration
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", errors.New("generate AI credential nonce")
	}
	out := g.Seal(nonce, nonce, []byte(value), []byte("admin:ai:"+c.id))
	return "ai:v1:" + base64.RawURLEncoding.EncodeToString(out), c.id, nil
}
func (c *Cipher) Open(raw, id string) (string, error) {
	if c == nil || id != c.id || !strings.HasPrefix(raw, "ai:v1:") {
		return "", ErrInvalidConfiguration
	}
	b, err := aes.NewCipher(c.key)
	if err != nil {
		return "", ErrInvalidConfiguration
	}
	g, err := cipher.NewGCM(b)
	if err != nil {
		return "", ErrInvalidConfiguration
	}
	d, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, "ai:v1:"))
	if err != nil || len(d) < g.NonceSize() {
		return "", ErrInvalidConfiguration
	}
	v, err := g.Open(nil, d[:g.NonceSize()], d[g.NonceSize():], []byte("admin:ai:"+c.id))
	if err != nil {
		return "", ErrInvalidConfiguration
	}
	return string(v), nil
}
