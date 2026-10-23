package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"admin/server/internal/module/payment"
	"admin/server/internal/payment/alipay"
)

func TestReconcilerSchedulesNoLaterThanExpiryAndRetainsFailedRetry(t *testing.T) {
	now := time.Date(2026, 10, 23, 7, 0, 0, 0, time.UTC)
	for _, scenario := range []struct {
		name                 string
		age, remaining, want time.Duration
		failure              bool
	}{
		{"young order near expiry", time.Minute, 30 * time.Second, 30 * time.Second, false},
		{"older order near expiry", 29 * time.Minute, time.Minute, time.Minute, false},
		{"failed query before expiry", 29 * time.Minute, time.Minute, time.Minute, true},
		{"ordinary young order", time.Minute, 20 * time.Minute, 2 * time.Minute, false},
		{"ordinary older order", 10 * time.Minute, 20 * time.Minute, 5 * time.Minute, false},
		{"failed query at expiry", 30 * time.Minute, 0, 5 * time.Minute, true},
		{"failed query after expiry", 35 * time.Minute, -5 * time.Minute, 5 * time.Minute, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			claim := claimedFixture(now)
			claim.CreatedAt, claim.ExpiresAt = now.Add(-scenario.age), now.Add(scenario.remaining)
			repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claim}}
			queryErr := errors.New("gateway unavailable")
			gateway := &reconcileGatewayStub{queryHook: func(context.Context, string) (alipay.Trade, error) {
				if scenario.failure {
					return alipay.Trade{}, queryErr
				}
				return alipay.Trade{}, alipay.ErrTradeNotFound
			}}
			r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
			r.now = func() time.Time { return now }
			err := r.Run(context.Background())
			if scenario.failure != errors.Is(err, queryErr) {
				t.Fatalf("unexpected query error %v", err)
			}
			if len(repo.advanced) != 1 || !repo.advanced[0].Equal(now.Add(scenario.want)) || repo.closed != 0 {
				t.Fatalf("next=%v want=%v closed=%d", repo.advanced, now.Add(scenario.want), repo.closed)
			}
		})
	}
}

func TestExpiryDeadlineBecomesDueAndFailedRetryIsNotReclaimed(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, nil)
	now := time.Now().UTC()
	expiry := now.Add(30 * time.Second)
	if err := db.WithContext(ctx).Model(&Model{}).Where("id=1").Updates(map[string]any{
		"created_at": now.Add(-29 * time.Minute), "expires_at": expiry, "next_check_at": now.Add(-time.Second),
	}).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	gateway := &reconcileGatewayStub{queryHook: func(context.Context, string) (alipay.Trade, error) { return alipay.Trade{}, alipay.ErrTradeNotFound }}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	r.now = func() time.Time { return now }
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	claims, err := repo.ClaimDue(ctx, expiry, 1, 15*time.Second)
	if err != nil || len(claims) != 1 {
		t.Fatalf("expiry check was postponed: claims=%+v err=%v", claims, err)
	}
	queryErr := errors.New("gateway unavailable")
	gateway.queryHook = func(context.Context, string) (alipay.Trade, error) { return alipay.Trade{}, queryErr }
	r.now = func() time.Time { return expiry }
	if err := r.process(ctx, claims[0], expiry); !errors.Is(err, queryErr) {
		t.Fatalf("failure lost: %v", err)
	}
	if again, err := repo.ClaimDue(ctx, expiry.Add(time.Minute), 1, 15*time.Second); err != nil || len(again) != 0 {
		t.Fatalf("failed expired order was reclaimed before its retry: %+v err=%v", again, err)
	}
	if due, err := repo.ClaimDue(ctx, expiry.Add(5*time.Minute), 1, 15*time.Second); err != nil || len(due) != 1 {
		t.Fatalf("failed expired order was not retried: %+v err=%v", due, err)
	}
}

func TestReconcilerClosesWhenExpiryPassesDuringQuery(t *testing.T) {
	now := time.Date(2026, 10, 23, 7, 0, 0, 0, time.UTC)
	claim := claimedFixture(now)
	claim.ExpiresAt = now.Add(time.Second)
	clock := now
	repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claim}}
	gateway := &reconcileGatewayStub{
		queryHook: func(context.Context, string) (alipay.Trade, error) {
			clock = claim.ExpiresAt.Add(time.Second)
			return alipay.Trade{OrderNo: claim.OrderNo, TradeNo: "T-1", AppID: "app", Status: "WAIT_BUYER_PAY", AmountCents: 1000}, nil
		},
		closeTrade: alipay.Trade{OrderNo: claim.OrderNo, TradeNo: "T-1", AppID: "app"},
	}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	r.now = func() time.Time { return clock }
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gateway.closeCall != 1 || repo.closed != 1 || len(repo.advanced) != 0 {
		t.Fatalf("expiry during query was deferred: close=%d closed=%d next=%v", gateway.closeCall, repo.closed, repo.advanced)
	}
}
