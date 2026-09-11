package account

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"admin/server/internal/shared/phone"
)

const (
	PermissionView   = "user:account:view"
	PermissionList   = "user:account:list"
	PermissionUpdate = "user:account:update"
	PermissionStatus = "user:account:status"
	PermissionDelete = "user:account:delete"
	PermissionRoles  = "user:account:authorize"
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

// NormalizePhone keeps the account-level nil semantics and delegates the
// mainland mobile number rules to the shared phone package.
func NormalizePhone(value *string) (*string, error) {
	return phone.NormalizeOptional(value)
}
