package account

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	PermissionList   = "account:user:list"
	PermissionUpdate = "account:user:update"
	PermissionStatus = "account:user:status"
	PermissionDelete = "account:user:delete"
	PermissionRoles  = "account:user:roles"
)

func NormalizeUsername(value string) (string, error) {
	value = strings.TrimSpace(value)
	count := utf8.RuneCountInString(value)
	if count < 3 || count > 64 {
		return "", fmt.Errorf("username must contain 3 to 64 Unicode characters")
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '_' && character != '-' {
			return "", fmt.Errorf("username contains an unsupported character")
		}
	}
	return value, nil
}

func NormalizePhone(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" || utf8.RuneCountInString(normalized) > 32 {
		return nil, fmt.Errorf("phone must contain 1 to 32 Unicode characters")
	}
	for _, character := range normalized {
		if unicode.IsControl(character) {
			return nil, fmt.Errorf("phone contains a control character")
		}
	}
	return &normalized, nil
}
