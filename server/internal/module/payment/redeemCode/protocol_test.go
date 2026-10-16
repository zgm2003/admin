package redeemcode

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNormalizeCodeBounds(t *testing.T) {
	got, err := normalizeCode("abcd-efgh-jkmn-pqrs-tuvw")
	if err != nil || got != "ABCDEFGHJKMNPQRSTUVW" {
		t.Fatalf("normalization = %q %v", got, err)
	}
	for _, raw := range []string{"", strings.Repeat("A", 129), strings.Repeat("A", 19), "AAAAAAAAAAAAAAAAAAA!"} {
		if _, err := normalizeCode(raw); err == nil {
			t.Errorf("accepted malformed input")
		}
	}
}

func TestGenerateCodesAreBoundedAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		code, err := newCode()
		if err != nil || len(code) != 20 || seen[code] {
			t.Fatal("invalid generated code", err)
		}
		seen[code] = true
	}
}

func TestFingerprintIncludesAllImmutableInputs(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	a := GenerateInput{RequestID: "550e8400-e29b-41d4-a716-446655440000", AmountCents: 100, Quantity: 2, ExpiresAt: &now, Remark: "use"}
	original := fingerprint(a)
	for _, mutate := range []func(*GenerateInput){func(x *GenerateInput) { x.AmountCents++ }, func(x *GenerateInput) { x.Quantity++ }, func(x *GenerateInput) { x.ExpiresAt = nil }, func(x *GenerateInput) { x.Remark = "other" }} {
		b := a
		mutate(&b)
		if bytes.Equal(original, fingerprint(b)) {
			t.Fatal("fingerprint omits field")
		}
	}
}

func TestStatusExpiryProjection(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Second)
	if projectedStatus(Model{Status: StatusUnused, ExpiresAt: &past}, now) != StatusExpired {
		t.Fatal("expiry not derived")
	}
	if projectedStatus(Model{Status: StatusUsed, ExpiresAt: &past}, now) != StatusUsed {
		t.Fatal("used code changed status")
	}
}
