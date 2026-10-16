package rechargepackage

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	configcache "admin/server/internal/shared/configCache"
	"admin/server/internal/shared/money"
	"admin/server/internal/shared/yesno"
)

const (
	MaxSnapshotBytes = 256 * 1024
	MaxCodeLength    = 64
	MaxNameLength    = 128
	MaxBadgeLength   = 32
	MaxCatalogItems  = 1000
)

var (
	packageScope = mustScope()
	codePattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
)

func mustScope() configcache.Scope {
	scope, err := configcache.NewScope("payment.rechargepackage", "global")
	if err != nil {
		panic(err)
	}
	return scope
}

func validateCode(value string) error {
	value = strings.TrimSpace(value)
	if len(value) < 1 || len(value) > MaxCodeLength || !codePattern.MatchString(value) {
		return fmt.Errorf("code is invalid")
	}
	return nil
}

func validateName(value string) error {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > MaxNameLength {
		return fmt.Errorf("name is invalid")
	}
	return nil
}

func normalizeBadge(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	if !utf8.ValidString(*value) || utf8.RuneCountInString(strings.TrimSpace(*value)) > MaxBadgeLength {
		return nil, fmt.Errorf("badge is invalid")
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	return &trimmed, nil
}

func validateAmountCents(value int64) error {
	if value < 1 || value > math.MaxInt64/money.UnitsPerCent {
		return fmt.Errorf("amount is invalid")
	}
	if _, err := money.CentsToUnits(value); err != nil {
		return fmt.Errorf("amount is invalid: %w", err)
	}
	return nil
}

func validateFlags(value yesno.Value) error {
	if !yesno.IsValid(value) {
		return fmt.Errorf("isEnabled is invalid")
	}
	return nil
}
