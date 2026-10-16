package transaction

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type fakeTransactionRepository struct {
	value Model
	err   error
}

func (f *fakeTransactionRepository) Find(ctx context.Context, id int64) (Model, error) {
	return f.value, f.err
}

func (f *fakeTransactionRepository) Count(ctx context.Context, query ListQuery) (int64, error) {
	return 1, f.err
}

func (f *fakeTransactionRepository) List(ctx context.Context, query ListQuery) ([]Model, error) {
	if f.value.ID == 0 && f.value.UserID == 0 && f.value.PlatformID == 0 && f.value.AmountUnits == 0 {
		return []Model{}, f.err
	}
	return []Model{f.value}, f.err
}

func TestServiceGetProjectsSignedBalanceBeforeAndDecimalStrings(t *testing.T) {
	repo := &fakeTransactionRepository{value: Model{
		ID: 12, TransactionNo: "PT2026101500000000000000000001", UserID: 7, PlatformID: 2,
		AmountUnits: 3_000_000, BalanceAfterUnits: -700_000_000, OrderID: ptrInt64(99),
		CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}}
	got, err := NewService(repo).Get(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != "0.03" || got.BalanceAfter != "-7" || got.BalanceBefore != "-7.03" || got.SourceType != SourcePayment {
		t.Fatalf("unexpected projection: %+v", got)
	}
}

func TestServiceRejectsOverflowingBalanceBefore(t *testing.T) {
	baseline := Model{
		ID: 1, TransactionNo: "PToverflowfixture", UserID: 7, PlatformID: 2,
		AmountUnits: 1, BalanceAfterUnits: math.MinInt64 + 1, OrderID: ptrInt64(9),
		Username: "named-user", PlatformCode: "fixture-platform", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}
	got, err := NewService(&fakeTransactionRepository{value: baseline}).Get(context.Background(), 1)
	if err != nil || got.Amount != "0.00000001" || got.BalanceBefore != "-92233720368.54775808" || got.BalanceAfter != "-92233720368.54775807" {
		t.Fatalf("safe minimum balance-before baseline=%+v err=%v", got, err)
	}
	for _, boundary := range []struct {
		name          string
		amount, after int64
	}{{"lower-after-one-unit", 1, math.MinInt64}, {"raise-amount-one-unit", 2, math.MinInt64 + 1}} {
		t.Run(boundary.name, func(t *testing.T) {
			row := baseline
			row.AmountUnits, row.BalanceAfterUnits = boundary.amount, boundary.after
			_, err := NewService(&fakeTransactionRepository{value: row}).Get(context.Background(), 1)
			cause := errors.Unwrap(err)
			if cause == nil || cause.Error() != "stored payment transaction balance before overflows" {
				t.Fatalf("arithmetic boundary must fail specifically on overflow: row=%+v err=%v cause=%v", row, err, cause)
			}
		})
	}
}

func TestServiceListPreservesEmptyListAndPagination(t *testing.T) {
	repo := &fakeTransactionRepository{}
	got, err := NewService(repo).List(context.Background(), ListQuery{UserID: 7, Page: 2, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if got.List == nil || got.Page != 2 || got.PageSize != 20 || got.Total != 1 {
		t.Fatalf("unexpected list: %+v", got)
	}
}

func TestServiceMapsRepositoryFailure(t *testing.T) {
	want := errors.New("database unavailable")
	_, err := NewService(&fakeTransactionRepository{err: want}).Get(context.Background(), 1)
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("error = %v, want wrapped repository error", err)
	}
}

func ptrInt64(value int64) *int64 { return &value }
