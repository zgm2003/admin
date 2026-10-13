// Package money represents RMB amounts as checked integers and decimal strings.
// Wallet mutation, pricing rules and persistence belong to their business modules.
package money

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

const (
	UnitsPerRMB  int64 = 100_000_000
	UnitsPerCent int64 = 1_000_000
)

var ErrInvalidAmount = errors.New("amount has invalid syntax, sign, decimal precision or int64 range")

// CentsToUnits converts payment cents to wallet units without rounding or overflow.
func CentsToUnits(cents int64) (int64, error) {
	if cents < 0 || cents > math.MaxInt64/UnitsPerCent {
		return 0, ErrInvalidAmount
	}
	return cents * UnitsPerCent, nil
}

// ParseRMBUnits accepts signed ASCII decimals with at most eight fractional digits.
// A leading minus is allowed for negative balances; plus, whitespace and rounding are not.
func ParseRMBUnits(value string) (int64, error) {
	return parseDecimal(value, 8, true)
}

// ParseRMBCents accepts unsigned ASCII decimals with at most two fractional digits.
func ParseRMBCents(value string) (int64, error) {
	return parseDecimal(value, 2, false)
}

func parseDecimal(value string, precision int, allowNegative bool) (int64, error) {
	negative := allowNegative && strings.HasPrefix(value, "-")
	limit := uint64(math.MaxInt64)
	if negative {
		value = value[1:]
		limit++ // MinInt64 has one more unit of magnitude than MaxInt64.
	}
	if value == "" {
		return 0, ErrInvalidAmount
	}
	var amount uint64
	decimal := false
	fractionDigits := 0
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '.' {
			if decimal || index == 0 || index == len(value)-1 {
				return 0, ErrInvalidAmount
			}
			decimal = true
			continue
		}
		if character < '0' || character > '9' {
			return 0, ErrInvalidAmount
		}
		if decimal {
			fractionDigits++
			if fractionDigits > precision {
				return 0, ErrInvalidAmount
			}
		}
		digit := uint64(character - '0')
		if amount > (limit-digit)/10 {
			return 0, ErrInvalidAmount
		}
		amount = amount*10 + digit
	}
	for fractionDigits < precision {
		if amount > limit/10 {
			return 0, ErrInvalidAmount
		}
		amount *= 10
		fractionDigits++
	}
	if negative {
		if amount == uint64(math.MaxInt64)+1 {
			return math.MinInt64, nil
		}
		return -int64(amount), nil
	}
	return int64(amount), nil
}

// FormatRMBUnits returns a canonical RMB decimal without trailing fractional zeros.
func FormatRMBUnits(units int64) (string, error) {
	sign := ""
	whole, remainder := units/UnitsPerRMB, units%UnitsPerRMB
	if units < 0 {
		sign = "-"
		// Negate only the divided components, never the full MinInt64 balance.
		whole, remainder = -whole, -remainder
	}
	wholeText := sign + strconv.FormatInt(whole, 10)
	fraction := strings.TrimRight(strconv.FormatInt(remainder+UnitsPerRMB, 10)[1:], "0")
	if fraction == "" {
		return wholeText, nil
	}
	return wholeText + "." + fraction, nil
}

// FormatRMBCents returns the exact two-decimal RMB amount required by payment APIs.
func FormatRMBCents(cents int64) (string, error) {
	if cents < 0 {
		return "", ErrInvalidAmount
	}
	whole := strconv.FormatInt(cents/100, 10)
	fraction := strconv.FormatInt(cents%100+100, 10)[1:]
	return whole + "." + fraction, nil
}
