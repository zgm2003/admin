package wallet

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeWalletRepository struct {
	value  Model
	found  bool
	err    error
	called int64
}

func (f *fakeWalletRepository) Find(ctx context.Context, userID int64) (Model, bool, error) {
	f.called = userID
	return f.value, f.found, f.err
}

func TestServiceGetReturnsZeroWalletWhenNoWalletExists(t *testing.T) {
	repo := &fakeWalletRepository{}
	service := NewService(repo)
	got, err := service.Get(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if got.HasWallet || got.UserID != 42 || got.BalanceUnits != 0 {
		t.Fatalf("unexpected empty wallet: %+v", got)
	}
	if repo.called != 42 {
		t.Fatalf("repository called for user %d, want 42", repo.called)
	}
}

func TestServiceGetFormatsSignedBalanceWithoutClamping(t *testing.T) {
	repo := &fakeWalletRepository{
		found: true,
		value: Model{UserID: 7, BalanceUnits: -100_000_000, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(2, 0)},
	}
	got, err := NewService(repo).Get(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasWallet || got.Balance != "-1" {
		t.Fatalf("negative balance was not preserved: %+v", got)
	}
}

func TestServiceGetRejectsInvalidUserID(t *testing.T) {
	for _, userID := range []int64{0, -1} {
		if _, err := NewService(&fakeWalletRepository{}).Get(context.Background(), userID); err == nil {
			t.Fatalf("user id %d was accepted", userID)
		}
	}
}

func TestServiceGetMapsRepositoryFailure(t *testing.T) {
	want := errors.New("database unavailable")
	_, err := NewService(&fakeWalletRepository{err: want}).Get(context.Background(), 9)
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("error = %v, want wrapped repository error", err)
	}
}
