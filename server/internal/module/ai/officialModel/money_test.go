package officialmodel

import "testing"

func TestParseMicros(t *testing.T) {
	valid := map[string]int64{
		"5":        5_000_000,
		"5.00":     5_000_000,
		"0.075":    75_000,
		"0.000001": 1,
		" 2.5 ":    2_500_000,
		"+1.25":    1_250_000,
	}
	for input, want := range valid {
		got, err := parseMicros(input)
		if err != nil || got != want {
			t.Fatalf("parseMicros(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
	invalid := []string{"", "-1", "1.", ".5", "1.1234567", "abc", "1e3", "1,000"}
	for _, input := range invalid {
		if _, err := parseMicros(input); err == nil {
			t.Fatalf("parseMicros(%q) succeeded, want error", input)
		}
	}
}

func TestFormatMicros(t *testing.T) {
	cases := map[int64]string{
		0:           "0.00",
		1:           "0.000001",
		75_000:      "0.075",
		5_000_000:   "5.00",
		10_000_000:  "10.00",
		1_250_000:   "1.25",
		30_000_000:  "30.00",
		180_000_000: "180.00",
	}
	for input, want := range cases {
		if got := formatMicros(input); got != want {
			t.Fatalf("formatMicros(%d) = %q, want %q", input, got, want)
		}
	}
}
