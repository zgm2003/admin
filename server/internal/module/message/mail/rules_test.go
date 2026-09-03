package mail

import "testing"

func TestNormalizeAndDomainBoundary(t *testing.T) {
	v, e := NormalizeRecipient(" User@Example.COM ")
	if e != nil || v != "user@example.com" {
		t.Fatalf("%q %v", v, e)
	}
	if !matchesDomain(v, "example.com") || !matchesDomain("a.b@example.com", "example.com") || matchesDomain("user@badexample.com", "example.com") || matchesDomain("user@example.com.evil", "example.com") {
		t.Fatal("domain boundary mismatch")
	}
	if _, e = NormalizeRule(RuleScopeDomain, "-example.com"); e == nil {
		t.Fatal("invalid domain accepted")
	}
}
