package email

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Normalize returns the canonical account email representation. It deliberately
// accepts only a bare address, never a display name or a second address.
func Normalize(value string) (string, error) {
	value = strings.ToLower(strings.Trim(value, " "))
	if value == "" || len(value) > 254 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("email address is invalid")
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Name != "" || parsed.Address != value {
		return "", fmt.Errorf("email address is invalid")
	}
	parts := strings.Split(value, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("email address is invalid")
	}
	return value, nil
}

func NormalizeOptional(value *string) (*string, error) {
	if value == nil || strings.Trim(*value, " ") == "" {
		return nil, nil
	}
	normalized, err := Normalize(*value)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

// Hint is suitable for list responses and audit rows; it never returns the
// complete local part of an address.
func Hint(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.SplitN(value, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || !utf8.ValidString(value) {
		return "***"
	}
	first, _ := utf8.DecodeRuneInString(parts[0])
	return string(first) + "***@" + parts[1]
}
