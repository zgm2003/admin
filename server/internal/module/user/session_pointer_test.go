package user

import "testing"

func TestCurrentSessionPointerKey(t *testing.T) {
	if got := CurrentSessionPointerKey(42); got != "auth:current-session:42" {
		t.Fatalf("CurrentSessionPointerKey(42) = %q", got)
	}
}
