package redeemcode

import (
	"admin/server/internal/module/payment/transaction"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"context"
	"errors"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	calls  int
	err    error
	redeem transaction.Model
	rows   []Model
	batch  Batch
}

func (f *fakeRepository) CreateBatch(ctx context.Context, b Batch, r []Model, n time.Time) (Batch, []Model, error) {
	f.calls++
	if f.err != nil {
		return Batch{}, nil, f.err
	}
	if f.batch.ID != 0 {
		return f.batch, f.rows, nil
	}
	b.ID = 1
	for i := range r {
		r[i].ID = int64(i + 1)
		r[i].BatchID = 1
	}
	return b, r, nil
}
func (f *fakeRepository) List(context.Context, ListQuery, time.Time) ([]Model, int64, error) {
	f.calls++
	return f.rows, int64(len(f.rows)), f.err
}
func (f *fakeRepository) Get(context.Context, int64) (Model, error) {
	f.calls++
	if len(f.rows) > 0 {
		return f.rows[0], f.err
	}
	return Model{}, f.err
}
func (f *fakeRepository) Lookup(context.Context, []byte) (Model, error) {
	f.calls++
	if len(f.rows) > 0 {
		return f.rows[0], f.err
	}
	return Model{}, f.err
}
func (f *fakeRepository) Export(context.Context, []int64) ([]Model, error) {
	f.calls++
	return f.rows, f.err
}
func (f *fakeRepository) Void(context.Context, []int64, time.Time) error { f.calls++; return f.err }
func (f *fakeRepository) Redeem(context.Context, int64, int64, []byte, time.Time) (transaction.Model, error) {
	f.calls++
	return f.redeem, f.err
}

type fakeLimiter struct {
	err       error
	finishErr error
	reserved  int
	invalid   bool
	user      int64
}

func (f *fakeLimiter) Reserve(ctx context.Context, user int64) (func(context.Context, bool) error, error) {
	f.reserved++
	f.user = user
	if f.err != nil {
		return nil, f.err
	}
	return func(ctx context.Context, invalid bool) error { f.invalid = invalid; return f.finishErr }, nil
}
func serviceForTest(t *testing.T, r *fakeRepository, l *fakeLimiter) *Service {
	t.Helper()
	k, e := secretkey.New(strings.Repeat("z", 64))
	if e != nil {
		t.Fatal(e)
	}
	return NewService(r, k, l)
}
func errorCode(t *testing.T, e error) int {
	t.Helper()
	var public *apperror.Error
	if !errors.As(e, &public) {
		t.Fatalf("not public error: %v", e)
	}
	return public.Code
}
func TestGenerateRejectsMalformedBeforeRepository(t *testing.T) {
	base := GenerateInput{RequestID: "550e8400-e29b-41d4-a716-446655440000", AmountCents: 100, Quantity: 1}
	for _, change := range []func(*GenerateInput){func(i *GenerateInput) { i.RequestID = "bad" }, func(i *GenerateInput) { i.Quantity = 1001 }, func(i *GenerateInput) { i.Quantity = 0 }, func(i *GenerateInput) { i.AmountCents = -1 }, func(i *GenerateInput) { i.AmountCents = 9223372036855 }, func(i *GenerateInput) { i.Remark = strings.Repeat("中", 256) }} {
		r := &fakeRepository{}
		s := serviceForTest(t, r, &fakeLimiter{})
		in := base
		change(&in)
		_, e := s.Generate(context.Background(), 1, in)
		if e == nil || errorCode(t, e) != apperror.CodeInvalidRequest || r.calls != 0 {
			t.Fatal("invalid generate accepted", e)
		}
	}
}
func TestGenerateFingerprintConflictIs409(t *testing.T) {
	r := &fakeRepository{err: ErrRequestConflict}
	s := serviceForTest(t, r, &fakeLimiter{})
	_, e := s.Generate(context.Background(), 1, GenerateInput{RequestID: "550e8400-e29b-41d4-a716-446655440000", AmountCents: 100, Quantity: 1})
	if errorCode(t, e) != apperror.CodeConflict {
		t.Fatal(e)
	}
}
func TestRedeemFailsClosedAndChargesOnlyInvalidCodes(t *testing.T) {
	for _, x := range []struct {
		name, code        string
		repoErr, limitErr error
		invalid           bool
		repoCalls         int
		public            int
	}{
		{"redis down", "ABCDEFGHJKLMNPQRSTUV", nil, ErrLeaseLost, false, 0, apperror.CodeDependencyUnavailable},
		{"malformed", "", nil, nil, true, 0, apperror.CodeInvalidRequest},
		{"not found", "ABCDEFGHJKLMNPQRSTUV", gorm.ErrRecordNotFound, nil, true, 1, apperror.CodeNotFound},
		{"used", "ABCDEFGHJKLMNPQRSTUV", ErrCodeUnavailable, nil, true, 1, apperror.CodeConflict},
		{"database down", "ABCDEFGHJKLMNPQRSTUV", errors.New("dependency"), nil, false, 1, apperror.CodeDependencyUnavailable},
	} {
		t.Run(x.name, func(t *testing.T) {
			r := &fakeRepository{err: x.repoErr}
			l := &fakeLimiter{err: x.limitErr}
			s := serviceForTest(t, r, l)
			_, e := s.Redeem(context.Background(), 1, 2, x.code)
			if errorCode(t, e) != x.public || r.calls != x.repoCalls || l.invalid != x.invalid || l.user != 1 {
				t.Fatalf("error=%v calls=%d invalid=%t", e, r.calls, l.invalid)
			}
		})
	}
}
func TestRedeemSuccessProjectsExactPersonalFacts(t *testing.T) {
	r := &fakeRepository{redeem: transaction.Model{ID: 3, TransactionNo: "PT3", AmountUnits: 300000000, BalanceAfterUnits: -700000000}}
	l := &fakeLimiter{}
	s := serviceForTest(t, r, l)
	v, e := s.Redeem(context.Background(), 1, 2, "ABCDEFGHJKLMNPQRSTUV")
	if e != nil || v.Amount != "3" || v.Balance != "-7" || l.invalid {
		t.Fatalf("%+v %v", v, e)
	}
}
func TestCorruptCiphertextFailsInsteadOfEmptyMask(t *testing.T) {
	r := &fakeRepository{rows: []Model{{ID: 1, BatchID: 1, Status: StatusUnused, AmountCents: 100, CodeCiphertext: "bad"}}}
	s := serviceForTest(t, r, &fakeLimiter{})
	if _, e := s.List(context.Background(), ListQuery{Page: 1, PageSize: 10}); errorCode(t, e) != apperror.CodeDependencyUnavailable {
		t.Fatal(e)
	}
}
func TestIDsRejectEmptyDuplicateAndOverLimit(t *testing.T) {
	for _, ids := range [][]int64{nil, {1, 1}, {0}, {-1}, make([]int64, 1001)} {
		if validateIDs(ids, 1000) == nil {
			t.Fatal("invalid IDs accepted")
		}
	}
}
func TestOptionsAreStaticWithoutDependencies(t *testing.T) {
	var s *Service
	o, e := s.Options(context.Background())
	if e != nil || len(o.Statuses) != 4 || o.Limits.MaxBatchQuantity != 1000 {
		t.Fatalf("%+v %v", o, e)
	}
}
