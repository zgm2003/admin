package yesno

import "testing"

func TestValues(t *testing.T) {
	if No != 0 || Yes != 1 {
		t.Fatalf("No=%d Yes=%d, want 0 and 1", No, Yes)
	}
	if !IsValid(No) || !IsValid(Yes) || IsValid(-1) || IsValid(2) {
		t.Fatal("unexpected Yes/No validation result")
	}
}
