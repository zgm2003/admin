package user

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
