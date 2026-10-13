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

var ErrInvalidAmount = errors.New("amount must be non-negative and within decimal precision and int64 range")

// CentsToUnits converts payment cents to wallet units without rounding or overflow.
func CentsToUnits(cents int64) (int64, error) {
	if cents < 0 || cents > math.MaxInt64/UnitsPerCent {
		return 0, ErrInvalidAmount
	}
	return cents * UnitsPerCent, nil
}

// ParseRMBUnits accepts unsigned ASCII decimals with at most eight fractional digits.
// Whitespace, signs, exponent notation and rounding are not accepted.
func ParseRMBUnits(value string) (int64, error) {
	return parseDecimal(value, 8)
}

// ParseRMBCents accepts unsigned ASCII decimals with at most two fractional digits.
func ParseRMBCents(value string) (int64, error) {
	return parseDecimal(value, 2)
}

func parseDecimal(value string, precision int) (int64, error) {
	if value == "" {
		return 0, ErrInvalidAmount
	}
	var amount int64
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
		digit := int64(character - '0')
		if amount > (math.MaxInt64-digit)/10 {
			return 0, ErrInvalidAmount
		}
		amount = amount*10 + digit
	}
	for fractionDigits < precision {
		if amount > math.MaxInt64/10 {
			return 0, ErrInvalidAmount
		}
		amount *= 10
		fractionDigits++
	}
	return amount, nil
}

// FormatRMBUnits returns a canonical RMB decimal without trailing fractional zeros.
func FormatRMBUnits(units int64) (string, error) {
	if units < 0 {
		return "", ErrInvalidAmount
	}
	whole := strconv.FormatInt(units/UnitsPerRMB, 10)
	fraction := strings.TrimRight(strconv.FormatInt(units%UnitsPerRMB+UnitsPerRMB, 10)[1:], "0")
	if fraction == "" {
		return whole, nil
	}
	return whole + "." + fraction, nil
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
