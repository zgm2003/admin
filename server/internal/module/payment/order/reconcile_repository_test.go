package order

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRepositoryRejectsInvalidReconcileLeaseInput(t *testing.T) {
	repository := &Repository{}
	if _, err := repository.ClaimDue(context.Background(), time.Now(), 0, time.Second); err == nil {
		t.Fatal("ClaimDue accepted zero batch size")
	}
	if err := repository.AdvanceCheck(context.Background(), 0, "token", time.Now().Add(time.Minute), time.Now()); err == nil {
		t.Fatal("AdvanceCheck accepted zero order id")
	}
	if err := repository.CloseClaimed(context.Background(), 0, "token", time.Now()); err == nil {
		t.Fatal("CloseClaimed accepted zero order id")
	}
}

func TestClaimDueRecoversHistoricalNullSchedule(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, nil)
	now := time.Now().UTC().Add(2 * time.Minute)
	claims, err := NewRepository(db).ClaimDue(ctx, now, 8, 15*time.Second)
	if err != nil || len(claims) != 1 || claims[0].ID != 1 || claims[0].Token == "" {
		t.Fatalf("historical order claims=%+v err=%v", claims, err)
	}
}

func TestClaimDueUsesIndividualTokensAndRejectsExpiredOwners(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, nil)
	var second Model
	if err := db.WithContext(ctx).First(&second, 1).Error; err != nil {
		t.Fatal(err)
	}
	second.ID, second.OrderNo, second.RequestID = 2, "PO-SECOND", "00000000-0000-4000-8000-000000000002"
	if err := db.WithContext(ctx).Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.WithContext(ctx).Model(&Model{}).Where("status = ?", StatusPending).Update("next_check_at", now.Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	claims, err := repo.ClaimDue(ctx, now, 8, 15*time.Second)
	if err != nil || len(claims) != 2 {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	if claims[0].Token == claims[1].Token {
		t.Fatal("different orders share a lease token")
	}
	late := now.Add(16 * time.Second)
	if err := repo.CloseClaimed(ctx, claims[0].ID, claims[0].Token, late); !errors.Is(err, ErrReconcileLeaseLost) {
		t.Fatalf("expired close: %v", err)
	}
	if err := repo.AdvanceCheck(ctx, claims[1].ID, claims[1].Token, late.Add(time.Minute), late); !errors.Is(err, ErrReconcileLeaseLost) {
		t.Fatalf("expired advance: %v", err)
	}
	next, err := repo.ClaimDue(ctx, late, 8, 15*time.Second)
	if err != nil || len(next) != 2 || next[0].Token == claims[0].Token {
		t.Fatalf("takeover=%+v err=%v", next, err)
	}
	if err := repo.CloseClaimed(ctx, claims[0].ID, claims[0].Token, late); !errors.Is(err, ErrReconcileLeaseLost) {
		t.Fatalf("old token closed successor: %v", err)
	}
	if err := repo.CloseClaimed(ctx, next[0].ID, next[0].Token, late); err != nil {
		t.Fatalf("current owner close: %v", err)
	}
}

func TestRenewCheckRequiresCurrentUnexpiredToken(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, nil)
	now := time.Now().UTC()
	if err := db.WithContext(ctx).Model(&Model{}).Where("id = 1").Update("next_check_at", now).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	renewer, ok := any(repo).(interface {
		RenewCheck(context.Context, int64, string, time.Time, time.Duration) error
	})
	if !ok {
		t.Fatal("order repository has no token-checked renewal")
	}
	claims, err := repo.ClaimDue(ctx, now, 1, 15*time.Second)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	claim := claims[0]
	if err := renewer.RenewCheck(ctx, claim.ID, claim.Token, now.Add(10*time.Second), 15*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ClaimDue(ctx, now.Add(16*time.Second), 1, 15*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := repo.AdvanceCheck(ctx, claim.ID, claim.Token, now.Add(time.Minute), now.Add(20*time.Second)); err != nil {
		t.Fatalf("renewal was not retained: %v", err)
	}
	if err := renewer.RenewCheck(ctx, claim.ID, claim.Token, now.Add(21*time.Second), 15*time.Second); !errors.Is(err, ErrReconcileLeaseLost) {
		t.Fatalf("released owner renewed: %v", err)
	}
}

func TestConcurrentClaimsDoNotShareAnOrder(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, nil)
	now := time.Now().UTC()
	if err := db.WithContext(ctx).Model(&Model{}).Where("id = 1").Update("next_check_at", now).Error; err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan []ClaimedOrder, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			<-start
			rows, err := NewRepository(db).ClaimDue(ctx, now, 1, 15*time.Second)
			if err != nil {
				t.Error(err)
			}
			results <- rows
		})
	}
	close(start)
	wg.Wait()
	close(results)
	count := 0
	for rows := range results {
		count += len(rows)
	}
	if count != 1 {
		t.Fatalf("claimed same order %d times", count)
	}
}
