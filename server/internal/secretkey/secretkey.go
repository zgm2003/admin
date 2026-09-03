package secretkey

import (
	"crypto/hkdf"
	"crypto/sha256"
	"fmt"
)

const (
	keyLength                = 32
	minimumRootLength        = 64
	rootSecretPlaceholder    = "replace_with_at_least_64_random_characters_before_running_api_server"
	jwtSigningPurpose        = "admin:auth:jwt-signing:v1"
	refreshHMACPurpose       = "admin:auth:refresh-token-hmac:v1"
	storageEncryptionPurpose = "admin:storage:cos-encryption:v1"
	mailEncryptionPurpose    = "admin:message:mail-encryption:v1"
)

type KeyRing struct {
	jwtSigningKey        []byte
	refreshTokenHMACKey  []byte
	storageEncryptionKey []byte
	mailEncryptionKey    []byte
}

func New(rootSecret string) (*KeyRing, error) {
	if err := validateRootSecret(rootSecret); err != nil {
		return nil, err
	}

	jwtSigningKey, err := hkdf.Key(sha256.New, []byte(rootSecret), nil, jwtSigningPurpose, keyLength)
	if err != nil {
		return nil, fmt.Errorf("derive JWT signing key: %w", err)
	}
	refreshTokenHMACKey, err := hkdf.Key(sha256.New, []byte(rootSecret), nil, refreshHMACPurpose, keyLength)
	if err != nil {
		return nil, fmt.Errorf("derive Refresh Token HMAC key: %w", err)
	}
	storageEncryptionKey, err := hkdf.Key(sha256.New, []byte(rootSecret), nil, storageEncryptionPurpose, keyLength)
	if err != nil {
		return nil, fmt.Errorf("derive storage encryption key: %w", err)
	}
	mailEncryptionKey, err := hkdf.Key(sha256.New, []byte(rootSecret), nil, mailEncryptionPurpose, keyLength)
	if err != nil {
		return nil, fmt.Errorf("derive mail encryption key: %w", err)
	}

	return &KeyRing{
		jwtSigningKey:        jwtSigningKey,
		refreshTokenHMACKey:  refreshTokenHMACKey,
		storageEncryptionKey: storageEncryptionKey,
		mailEncryptionKey:    mailEncryptionKey,
	}, nil
}

func (k *KeyRing) MailEncryptionKey() []byte { return append([]byte(nil), k.mailEncryptionKey...) }

func (k *KeyRing) StorageEncryptionKey() []byte {
	return append([]byte(nil), k.storageEncryptionKey...)
}

func (k *KeyRing) JWTSigningKey() []byte {
	return append([]byte(nil), k.jwtSigningKey...)
}

func (k *KeyRing) RefreshTokenHMACKey() []byte {
	return append([]byte(nil), k.refreshTokenHMACKey...)
}

func validateRootSecret(rootSecret string) error {
	if rootSecret == rootSecretPlaceholder {
		return fmt.Errorf("root secret placeholder is not allowed")
	}
	if len(rootSecret) < minimumRootLength {
		return fmt.Errorf("root secret must contain at least %d ASCII characters", minimumRootLength)
	}
	for _, character := range []byte(rootSecret) {
		if character > 0x7f {
			return fmt.Errorf("root secret must contain only ASCII characters")
		}
	}
	return nil
}
