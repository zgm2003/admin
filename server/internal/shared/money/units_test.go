package money_test

import (
	"errors"
	"math"
	"testing"

	"admin/server/internal/shared/money"
)

func TestParseRMBUnitsPreservesExactDecimalAmounts(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int64
	}{
		{"0", 0},
		{"0.00000000", 0},
		{"0.00000001", 1},
		{"0.01", 1_000_000},
		{"1", 100_000_000},
		{"001.23000000", 123_000_000},
		{"123.45678901", 12_345_678_901},
		{"92233720368.54775807", math.MaxInt64},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := money.ParseRMBUnits(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("ParseRMBUnits(%q) = %d, %v; want %d", tc.input, got, err, tc.want)
			}
		})
	}
}

func TestParseRMBUnitsRejectsMalformedPrecisionAndOverflow(t *testing.T) {
	for _, input := range []string{
		"", " ", " 1", "1 ", "+1", "-1", "-0", ".01", "1.", "1..00",
		"1e3", "1E-8", "NaN", "Inf", "1,000", "1_000", "1/2", "１.００", "١",
		"0.000000001", "1.000000000", "92233720368.54775808", "92233720369", "18446744073709551616",
	} {
		t.Run(input, func(t *testing.T) {
			got, err := money.ParseRMBUnits(input)
			if !errors.Is(err, money.ErrInvalidAmount) || got != 0 {
				t.Fatalf("ParseRMBUnits(%q) = %d, %v; want zero and ErrInvalidAmount", input, got, err)
			}
		})
	}
}

func TestFormatRMBUnitsReturnsCanonicalExactStrings(t *testing.T) {
	for _, tc := range []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{1, "0.00000001"},
		{1_000_000, "0.01"},
		{100_000_000, "1"},
		{123_000_000, "1.23"},
		{12_345_678_901, "123.45678901"},
		{math.MaxInt64, "92233720368.54775807"},
	} {
		got, err := money.FormatRMBUnits(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("FormatRMBUnits(%d) = %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
	for _, input := range []int64{-1, math.MinInt64} {
		got, err := money.FormatRMBUnits(input)
		if got != "" || !errors.Is(err, money.ErrInvalidAmount) {
			t.Fatalf("FormatRMBUnits(%d) = %q, %v; want empty and ErrInvalidAmount", input, got, err)
		}
	}
}

func TestCentsToUnitsRejectsOverflowBeforeMultiplication(t *testing.T) {
	for _, tc := range []struct {
		input int64
		want  int64
	}{
		{0, 0},
		{1, 1_000_000},
		{100, 100_000_000},
		{123, 123_000_000},
		{9_223_372_036_854, 9_223_372_036_854_000_000},
	} {
		got, err := money.CentsToUnits(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("CentsToUnits(%d) = %d, %v; want %d", tc.input, got, err, tc.want)
		}
	}
	for _, input := range []int64{-1, math.MinInt64, 9_223_372_036_855, math.MaxInt64} {
		got, err := money.CentsToUnits(input)
		if got != 0 || !errors.Is(err, money.ErrInvalidAmount) {
			t.Fatalf("CentsToUnits(%d) = %d, %v; want zero and ErrInvalidAmount", input, got, err)
		}
	}
}

func TestParseRMBCentsPreservesPaymentAmounts(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int64
	}{
		{"0", 0},
		{"0.00", 0},
		{"0.01", 1},
		{"1", 100},
		{"1.2", 120},
		{"001.23", 123},
		{"92233720368547758.07", math.MaxInt64},
	} {
		got, err := money.ParseRMBCents(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("ParseRMBCents(%q) = %d, %v; want %d", tc.input, got, err, tc.want)
		}
	}
}

func TestParseRMBCentsRejectsSubcentAndOverflow(t *testing.T) {
	for _, input := range []string{
		"", " ", " 1", "1 ", "+1", "-1", ".01", "1.", "1..00",
		"1e3", "NaN", "Inf", "1,000", "1_000", "１.００", "0.001", "1.000",
		"92233720368547758.08", "92233720368547759", "18446744073709551616",
	} {
		got, err := money.ParseRMBCents(input)
		if got != 0 || !errors.Is(err, money.ErrInvalidAmount) {
			t.Fatalf("ParseRMBCents(%q) = %d, %v; want zero and ErrInvalidAmount", input, got, err)
		}
	}
}

func TestFormatRMBCentsReturnsTwoDecimalPlaces(t *testing.T) {
	for _, tc := range []struct {
		input int64
		want  string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{10, "0.10"},
		{100, "1.00"},
		{123, "1.23"},
		{math.MaxInt64, "92233720368547758.07"},
	} {
		got, err := money.FormatRMBCents(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("FormatRMBCents(%d) = %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
	for _, input := range []int64{-1, math.MinInt64} {
		got, err := money.FormatRMBCents(input)
		if got != "" || !errors.Is(err, money.ErrInvalidAmount) {
			t.Fatalf("FormatRMBCents(%d) = %q, %v; want empty and ErrInvalidAmount", input, got, err)
		}
	}
}
