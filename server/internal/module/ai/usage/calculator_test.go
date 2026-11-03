package usage

import (
	"math"
	"reflect"
	"testing"
)

func TestCalculateRealPricingUnits(t *testing.T) {
	for _, tc := range []struct {
		name string
		line LineInput
		want int64
	}{
		{"million tokens", LineInput{"input", 500000000, "1000000", "1000000", 1500000}, 750000000},
		{"three images", LineInput{"image", 20000000, "3", "1", 1500000}, 90000000},
		{"fractional audio seconds", LineInput{"audio", 10000000, "1.25", "1", 1000000}, 12500000},
		{"explicit free rate", LineInput{"free", 0, "100", "1", 1000000}, 0},
		{"confirmed zero quantity", LineInput{"zero", 100000000, "0", "1", 1000000}, 0},
		{"exact max units", LineInput{"max", math.MaxInt64, "1", "1", 1000000}, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Calculate([]LineInput{tc.line})
			if err != nil || got.TotalUnits != tc.want || len(got.Items) != 1 || got.Items[0].AmountUnits != tc.want {
				t.Fatalf("got %+v, %v; want %d", got, err, tc.want)
			}
		})
	}
}

func TestCalculateRoundsOnceAndAllocatesRemainderStably(t *testing.T) {
	lines := []LineInput{
		{"b", 1, "1", "3", 1000000},
		{"a", 1, "1", "3", 1000000},
	}
	want := CalculatedCharge{TotalUnits: 1, Items: []CalculatedItem{{"a", 1}, {"b", 0}}}
	for _, input := range [][]LineInput{lines, {lines[1], lines[0]}} {
		got, err := Calculate(input)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, %v; want %+v", got, err, want)
		}
	}
	if lines[0].ItemKey != "b" {
		t.Fatal("Calculate mutated caller input")
	}
}

func TestCalculateAllocatesByLargestRemainderBeforeItemKey(t *testing.T) {
	got, err := Calculate([]LineInput{
		{"a", 1, "1", "10", 1000000},
		{"b", 1, "8", "10", 1000000},
		{"c", 1, "2", "10", 1000000},
	})
	want := CalculatedCharge{TotalUnits: 2, Items: []CalculatedItem{{"a", 0}, {"b", 1}, {"c", 1}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestCalculateRejectsMalformedQuantityAndInvalidRate(t *testing.T) {
	for _, raw := range []string{"", " ", " 1", "1 ", "+1", "-1", "1e2", "1/2", ".5", "1.", "NaN", "1.000000001", "10000000000000000"} {
		t.Run(raw, func(t *testing.T) {
			for _, line := range []LineInput{{"x", 1, raw, "1", 1000000}, {"x", 1, "1", raw, 1000000}} {
				if _, err := Calculate([]LineInput{line}); err == nil {
					t.Fatalf("malformed decimal accepted: %+v", line)
				}
			}
		})
	}
	for _, lines := range [][]LineInput{
		nil,
		{{"x", 1, "1", "0", 1000000}},
		{{"", 1, "1", "1", 1000000}},
		{{"x", -1, "1", "1", 1000000}},
		{{"x", 1, "1", "1", 0}},
		{{"x", 1, "1", "1", 100000001}},
		{{"x", 1, "1", "1", 1000000}, {"x", 1, "1", "1", 1000000}},
		{{"x", math.MaxInt64, "2", "1", 1000000}},
		{{"x", math.MaxInt64, "1", "1", 1000000}, {"y", 1, "1", "1", 1000000}},
	} {
		if _, err := Calculate(lines); err == nil {
			t.Fatalf("invalid charge accepted: %+v", lines)
		}
	}
}

func TestCalculatePreservesDecimalPrecisionBeyondFloatRange(t *testing.T) {
	got, err := Calculate([]LineInput{{"large", 1, "9007199254740993", "1", 1000000}})
	if err != nil || got.TotalUnits != 9007199254740993 {
		t.Fatalf("precision lost: %+v, %v", got, err)
	}
}
