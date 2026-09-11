package phone

import (
	"strings"
	"testing"
)

func TestNormalizeMainlandPhoneAcceptsSupportedForms(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "eleven digits", input: "15671628271", want: "+8615671628271"},
		{name: "86 prefix with separators", input: "86 156-7162-8271", want: "+8615671628271"},
		{name: "plus 86 prefix with separators", input: "+86 156-7162-8271", want: "+8615671628271"},
		{name: "ascii spaces only", input: "156 7162 8271", want: "+8615671628271"},
		{name: "hyphens only", input: "156-7162-8271", want: "+8615671628271"},
		{name: "already e164", input: "+8615671628271", want: "+8615671628271"},
		{name: "surrounding whitespace", input: "  +86 156-7162-8271  ", want: "+8615671628271"},
		{name: "surrounding ascii spaces", input: "   15671628271   ", want: "+8615671628271"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Normalize(test.input)
			if err != nil {
				t.Fatalf("Normalize(%q) returned error %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("Normalize(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestNormalizeMainlandPhoneRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "whitespace only", input: "   "},
		{name: "leading tab", input: "\t15671628271"},
		{name: "trailing newline", input: "15671628271\n"},
		{name: "surrounding tabs and newlines", input: "\t15671628271\n"},
		{name: "trailing carriage return", input: "15671628271\r"},
		{name: "leading vertical tab", input: "\v15671628271"},
		{name: "leading form feed", input: "\f15671628271"},
		{name: "non breaking space prefix", input: "\u00a015671628271"},
		{name: "non breaking space inside", input: "1567162\u00a08271"},
		{name: "non breaking space only", input: "\u00a0"},
		{name: "too short", input: "1567162827"},
		{name: "too long", input: "156716282712"},
		{name: "second digit zero", input: "10671628271"},
		{name: "second digit two", input: "12671628271"},
		{name: "second digit two with country code", input: "+86 126-7162-8271"},
		{name: "control character inside", input: "156716\n28271"},
		{name: "non digit letter", input: "1567162827a"},
		{name: "non digit symbol", input: "15671628271!"},
		{name: "parentheses are not separators", input: "+86(156)7162-8271"},
		{name: "non mainland country code", input: "+1 4155552671"},
		{name: "hong kong number", input: "+852 1234 5678"},
		{name: "double zero country code", input: "0086 15671628271"},
		{name: "leading dot", input: "156.7162.8271"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Normalize(test.input)
			if err == nil {
				t.Fatalf("Normalize(%q) = %q, want error", test.input, got)
			}
			if got != "" {
				t.Fatalf("Normalize(%q) returned %q with an error", test.input, got)
			}
			if test.input != "" && strings.Contains(err.Error(), test.input) {
				t.Fatalf("Normalize(%q) echoed the raw input in the error", test.input)
			}
		})
	}
}

func TestNormalizeOptionalKeepsNilSemantics(t *testing.T) {
	got, err := NormalizeOptional(nil)
	if err != nil || got != nil {
		t.Fatalf("NormalizeOptional(nil) = %v,%v, want nil,nil", got, err)
	}

	normalized, err := NormalizeOptional(pointerTo(" 86 156-7162-8271 "))
	if err != nil {
		t.Fatalf("NormalizeOptional returned error %v", err)
	}
	if normalized == nil || *normalized != "+8615671628271" {
		t.Fatalf("NormalizeOptional = %v, want +8615671628271", normalized)
	}

	if _, err := NormalizeOptional(pointerTo("  ")); err == nil {
		t.Fatal("NormalizeOptional accepted an empty phone")
	}
}

func TestPhoneHintNeverRevealsTheFullNumber(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "e164", input: "+8615671628271", want: "156****8271"},
		{name: "national", input: "15671628271", want: "156****8271"},
		{name: "with separators", input: "86 156-7162-8271", want: "156****8271"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := Hint(test.input)
			if got != test.want {
				t.Fatalf("Hint(%q) = %q, want %q", test.input, got, test.want)
			}
			if strings.Contains(got, "15671628271") || strings.Contains(got, "+8615671628271") {
				t.Fatalf("Hint(%q) leaked the full number", test.input)
			}
			if strings.Contains(got, "716") {
				t.Fatalf("Hint(%q) leaked the middle digits", test.input)
			}
		})
	}

	for _, test := range []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "whitespace", input: "   "},
		{name: "too short", input: "1567162827"},
		{name: "non mainland", input: "+1 4155552671"},
		{name: "not a phone", input: "not-a-phone"},
	} {
		t.Run("masked "+test.name, func(t *testing.T) {
			if got := Hint(test.input); got != "****" {
				t.Fatalf("Hint(%q) = %q, want ****", test.input, got)
			}
		})
	}
}

func pointerTo(value string) *string {
	return &value
}
