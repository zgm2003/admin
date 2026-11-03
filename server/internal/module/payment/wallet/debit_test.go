package wallet

import (
	"math"
	"testing"
)

func TestCheckedDebitAllowsNegativeButRejectsInt64Underflow(t *testing.T) {
	for _, fixture := range []struct{ before, amount, after int64 }{{0, 1, -1}, {-9, 4, -13}, {math.MaxInt64, 1, math.MaxInt64 - 1}} {
		got, err := checkedDebit(fixture.before, fixture.amount)
		if err != nil || got != fixture.after {
			t.Fatalf("%+v got=%d err=%v", fixture, got, err)
		}
	}
	for _, amount := range []int64{0, -1, 2} {
		if _, err := checkedDebit(math.MinInt64, amount); err == nil {
			t.Fatalf("underflow/invalid amount accepted: %d", amount)
		}
	}
	if _, err := checkedDebit(math.MinInt64+1, 1); err != nil {
		t.Fatal(err)
	}
}
