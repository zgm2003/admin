package origins

import "testing"

func TestParseAcceptsMultipleExactOrigins(t *testing.T) {
	got, err := Parse("http://localhost:16300, http://127.0.0.1:16300,http://localhost:16300")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got) != 2 || got[0] != "http://localhost:16300" || got[1] != "http://127.0.0.1:16300" {
		t.Fatalf("Parse() = %#v", got)
	}
}

func TestParseRejectsWildcardAndPaths(t *testing.T) {
	for _, value := range []string{"*", "http://*.example.com", "http://localhost:16300/admin", ""} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("Parse(%q) succeeded, want error", value)
		}
	}
}

func TestContainsRequiresExactMatch(t *testing.T) {
	allowed, err := Parse("http://localhost:16300,http://127.0.0.1:16300")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range allowed {
		if !Contains(allowed, value) {
			t.Fatalf("Contains(%q) = false", value)
		}
	}
	for _, value := range []string{"http://localhost:16301", "http://127.0.0.1:16301", "", "http://localhost:16300/"} {
		if Contains(allowed, value) {
			t.Fatalf("Contains(%q) = true", value)
		}
	}
}
