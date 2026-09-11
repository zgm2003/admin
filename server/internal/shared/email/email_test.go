package email

import "testing"

func TestNormalizeCanonicalizesBareEmail(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{"trim and lower", " User@Example.COM ", "user@example.com"},
		{"unicode local", "用户@例子.公司", "用户@例子.公司"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Normalize(test.input)
			if err != nil || got != test.want {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
}

func TestNormalizeRejectsDisplayNamesAndMalformedValues(t *testing.T) {
	for _, input := range []string{"", "a", "a@@b.com", "A <a@example.com>", "a@example.com\n", "a@example.com\t", "a@example.com " + string(rune(0x00a0))} {
		if got, err := Normalize(input); err == nil || got != "" {
			t.Fatalf("input %q accepted as %q", input, got)
		}
	}
}

func TestNormalizeOptionalAndHint(t *testing.T) {
	if got, err := NormalizeOptional(nil); err != nil || got != nil {
		t.Fatalf("nil optional=%v err=%v", got, err)
	}
	value := " USER@Example.COM "
	got, err := NormalizeOptional(&value)
	if err != nil || got == nil || *got != "user@example.com" {
		t.Fatalf("optional=%v err=%v", got, err)
	}
	if hint := Hint(*got); hint != "u***@example.com" {
		t.Fatalf("hint=%q", hint)
	}
	if hint := Hint("用户@例子.公司"); hint != "用***@例子.公司" {
		t.Fatalf("unicode hint=%q", hint)
	}
}
