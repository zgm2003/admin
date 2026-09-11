// Package phone owns the mainland China mobile number rules shared by the
// user, auth and message/sms modules.
package phone

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	countryCode  = "+86"
	nationalCode = "86"
	maskedTail   = "****"
	fullyMasked  = "****"
	hintPrefix   = 3
	hintSuffixAt = 7
)

var mainlandNationalPattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

var separatorReplacer = strings.NewReplacer(" ", "", "-", "")

// Normalize validates a mainland China mobile number and returns E.164. Only
// the ASCII separators space and hyphen are stripped, together with an optional
// 86 / +86 country code; every other whitespace or control character is
// rejected, including leading and trailing ones. Errors never echo the input.
func Normalize(value string) (string, error) {
	candidate := separatorReplacer.Replace(value)
	if strings.HasPrefix(candidate, countryCode) {
		candidate = strings.TrimPrefix(candidate, countryCode)
	} else {
		candidate = strings.TrimPrefix(candidate, nationalCode)
	}
	if !mainlandNationalPattern.MatchString(candidate) {
		return "", fmt.Errorf("phone number must be a mainland China mobile number")
	}
	return countryCode + candidate, nil
}

// NormalizeOptional keeps nil as SQL NULL and rejects empty or invalid values.
func NormalizeOptional(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized, err := Normalize(*value)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

// Hint returns a masked representation that never exposes the middle digits or
// the full number. Values outside the mainland mobile format are fully masked.
func Hint(value string) string {
	normalized, err := Normalize(value)
	if err != nil {
		return fullyMasked
	}
	national := strings.TrimPrefix(normalized, countryCode)
	return national[:hintPrefix] + maskedTail + national[hintSuffixAt:]
}
