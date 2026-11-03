package tool

import (
	"strings"
	"testing"
)

func TestCipherUsesIndependentAuthenticatedEnvelope(t *testing.T) {
	c, e := NewCipher(strings.Repeat("x", 64), "k1")
	if e != nil {
		t.Fatal(e)
	}
	a, key, e := c.Seal(`{"Authorization":"Bearer SECRET"}`)
	if e != nil || strings.Contains(a, "SECRET") {
		t.Fatalf("secret not sealed %v", e)
	}
	b, _, _ := c.Seal(`{"Authorization":"Bearer SECRET"}`)
	if a == b {
		t.Fatal("nonce reused")
	}
	raw, e := c.Open(a, key)
	if e != nil || raw != `{"Authorization":"Bearer SECRET"}` {
		t.Fatalf("decrypt %s %v", raw, e)
	}
	if _, e := c.Open(a, "wrong"); e == nil {
		t.Fatal("wrong key accepted")
	}
	if _, e := c.Open(a[:len(a)-3]+"abc", key); e == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}
