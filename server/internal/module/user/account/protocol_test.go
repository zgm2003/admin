package account

import (
	"strings"
	"testing"
)

func TestNormalizeUsername(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
		valid bool
	}{
		{input: "  张三_01  ", want: "张三_01", valid: true},
		{input: "alice-01", want: "alice-01", valid: true},
		{input: "ab", valid: false},
		{input: strings.Repeat("a", 65), valid: false},
		{input: "bad name", valid: false},
		{input: "bad@email", valid: false},
	} {
		got, err := NormalizeUsername(test.input)
		if test.valid && (err != nil || got != test.want) {
			t.Fatalf("NormalizeUsername(%q) = %q,%v", test.input, got, err)
		}
		if !test.valid && err == nil {
			t.Fatalf("NormalizeUsername(%q) accepted invalid input", test.input)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	tooLong := strings.Repeat("1", 33)
	for _, test := range []struct {
		name  string
		input *string
		want  *string
		valid bool
	}{
		{name: "nil", valid: true},
		{name: "normalizes separators and country code", input: pointerTo("  +86 138-0000-0000  "), want: pointerTo("+8613800000000"), valid: true},
		{name: "accepts national number", input: pointerTo("13800000000"), want: pointerTo("+8613800000000"), valid: true},
		{name: "rejects empty", input: pointerTo("  ")},
		{name: "rejects non mainland number", input: pointerTo("+1 415-555-2671")},
		{name: "rejects second digit violation", input: pointerTo("12671628271")},
		{name: "rejects more than 32 runes", input: &tooLong},
		{name: "rejects control character", input: pointerTo("138\n0000")},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizePhone(test.input)
			if test.valid {
				if err != nil || (got == nil) != (test.want == nil) || got != nil && *got != *test.want {
					t.Fatalf("NormalizePhone(%v) = %v,%v, want %v,nil", test.input, got, err, test.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("NormalizePhone(%v) accepted invalid input", test.input)
			}
		})
	}
}

func pointerTo(value string) *string {
	return &value
}
