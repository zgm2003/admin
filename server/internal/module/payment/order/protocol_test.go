package order

import "testing"

func TestStatusValuesAndTransitionsPreserveLatePayment(t *testing.T) {
	if StatusPending != 1 || StatusPaid != 2 || StatusClosed != 3 {
		t.Fatalf("payment order statuses = %d/%d/%d", StatusPending, StatusPaid, StatusClosed)
	}
	for _, test := range []struct {
		from Status
		to   Status
		want bool
	}{
		{StatusPending, StatusPaid, true},
		{StatusPending, StatusClosed, true},
		{StatusClosed, StatusPaid, true},
		{StatusPaid, StatusClosed, false},
		{StatusPaid, StatusPending, false},
		{StatusClosed, StatusPending, false},
	} {
		if got := canTransition(test.from, test.to); got != test.want {
			t.Errorf("canTransition(%d,%d) = %v, want %v", test.from, test.to, got, test.want)
		}
	}
}

func TestProofSourceIsClosedNumericProtocol(t *testing.T) {
	if ProofSourceCallback != 1 || ProofSourceQuery != 2 {
		t.Fatalf("payment proof sources = %d/%d", ProofSourceCallback, ProofSourceQuery)
	}
	if ProofSource(0).Valid() || ProofSource(3).Valid() || !ProofSourceCallback.Valid() || !ProofSourceQuery.Valid() {
		t.Fatal("proof source validation does not enforce the closed protocol")
	}
}
