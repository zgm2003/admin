package tool

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

var ErrInvalidConfiguration = errors.New("invalid tool cipher configuration")

type Cipher struct {
	key []byte
	id  string
}

func NewCipher(root, keyID string) (*Cipher, error) {
	if len(root) < 64 || keyID == "" || len(keyID) > 64 {
		return nil, ErrInvalidConfiguration
	}
	key, e := hkdf.Key(sha256.New, []byte(root), nil, "admin:ai:tool-credential-encryption:v1", 32)
	if e != nil {
		return nil, ErrInvalidConfiguration
	}
	return &Cipher{key: key, id: keyID}, nil
}
func (c *Cipher) Seal(value string) (string, string, error) {
	if c == nil || value == "" {
		return "", "", ErrInvalidConfiguration
	}
	b, e := aes.NewCipher(c.key)
	if e != nil {
		return "", "", ErrInvalidConfiguration
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return "", "", ErrInvalidConfiguration
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", "", e
	}
	out := g.Seal(nonce, nonce, []byte(value), []byte("admin:ai:tool:"+c.id))
	return "ait:v1:" + base64.RawURLEncoding.EncodeToString(out), c.id, nil
}
func (c *Cipher) Open(raw, id string) (string, error) {
	if c == nil || id != c.id || !strings.HasPrefix(raw, "ait:v1:") {
		return "", ErrInvalidConfiguration
	}
	b, e := aes.NewCipher(c.key)
	if e != nil {
		return "", ErrInvalidConfiguration
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return "", ErrInvalidConfiguration
	}
	d, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, "ait:v1:"))
	if e != nil || len(d) < g.NonceSize() {
		return "", ErrInvalidConfiguration
	}
	v, e := g.Open(nil, d[:g.NonceSize()], d[g.NonceSize():], []byte("admin:ai:tool:"+c.id))
	if e != nil {
		return "", ErrInvalidConfiguration
	}
	return string(v), nil
}

func (c *Cipher) Headers(ctx context.Context, v Version) (map[string]string, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if v.SecretCiphertext == nil {
		return nil, nil
	}
	if v.EncryptionKeyID == nil {
		return nil, ErrInvalidConfiguration
	}
	raw, e := c.Open(*v.SecretCiphertext, *v.EncryptionKeyID)
	if e != nil {
		return nil, e
	}
	var headers map[string]string
	e = json.Unmarshal([]byte(raw), &headers)
	return headers, e
}
