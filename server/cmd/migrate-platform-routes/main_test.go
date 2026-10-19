package main

import (
	"strings"
	"testing"
)

func TestPreflightRejectsPendingLegacyCallbackOrders(t *testing.T) {
	err := preflight(facts{PendingLegacyOrders: 2})
	if err == nil || !strings.Contains(err.Error(), "2") {
		t.Fatalf("preflight error = %v, want pending-order count", err)
	}
}

func TestPreflightAllowsCutoverWithoutPendingLegacyCallbackOrders(t *testing.T) {
	if err := preflight(facts{}); err != nil {
		t.Fatalf("preflight rejected an empty cutover: %v", err)
	}
}
