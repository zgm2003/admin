package user

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
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
