package pricing

import (
	"errors"
	"testing"
)

func int64Ptr(value int64) *int64 { return &value }

func TestCalculateTokenIncludesCacheWithoutDoubleChargingInput(t *testing.T) {
	got, err := CalculateToken(TokenUsage{
		UncachedInput: 1000,
		CacheRead:     5000,
		CacheWrite:    200,
		Output:        500,
	}, TokenPrices{
		Input:      10_000_000,
		Output:     30_000_000,
		CacheRead:  int64Ptr(1_000_000),
		CacheWrite: int64Ptr(12_500_000),
	}, 6_700_000)
	if err != nil {
		t.Fatal(err)
	}
	// (0.01 + 0.005 + 0.0025 + 0.015) * 6.7 = 0.21775 CNY
	if got != 217750 {
		t.Fatalf("cost = %d micros, want 217750", got)
	}
}

func TestCalculateTokenRequiresCachePrice(t *testing.T) {
	_, err := CalculateToken(TokenUsage{CacheRead: 1}, TokenPrices{Input: 1, Output: 1}, 1000000)
	if !errors.Is(err, ErrMissingPrice) {
		t.Fatalf("error = %v, want ErrMissingPrice", err)
	}
}

func TestCalculateUnit(t *testing.T) {
	cases := []struct {
		name     string
		unit     PriceUnit
		quantity int64
		price    int64
		want     int64
	}{
		{"image", UnitImage, 2, 300000, 600000},
		{"video seconds", UnitSecond, 16, 500000, 8000000},
		{"tts million chars", UnitMillionCharacters, 2500000, 5000000, 12500000},
		{"stt minutes", UnitMinute, 3, 20000, 60000},
		{"request", UnitRequest, 4, 1000, 4000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalculateUnit(tc.unit, tc.quantity, tc.price)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("cost = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCalculateUnitRejectsInvalidInput(t *testing.T) {
	if _, err := CalculateUnit(UnitImage, -1, 1); !errors.Is(err, ErrInvalidUsage) {
		t.Fatalf("error = %v, want ErrInvalidUsage", err)
	}
	if _, err := CalculateUnit(PriceUnit(99), 1, 1); !errors.Is(err, ErrInvalidUsage) {
		t.Fatalf("error = %v, want ErrInvalidUsage", err)
	}
}
