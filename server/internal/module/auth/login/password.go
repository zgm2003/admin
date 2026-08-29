package auth

import (
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	minimumPasswordRunes          = 8
	maximumPasswordBytes          = 72
	missingCredentialPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
)

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < minimumPasswordRunes {
		return fmt.Errorf("password must contain at least %d Unicode characters", minimumPasswordRunes)
	}
	if len(password) > maximumPasswordBytes {
		return fmt.Errorf("password must contain at most %d bytes", maximumPasswordBytes)
	}
	return nil
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	return nil
}
