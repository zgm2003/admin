package order

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"admin/server/internal/module/payment"
	"admin/server/internal/payment/alipay"
)

func testReconciler(repository reconcileRepository, runtime reconcileRuntime, confirmer reconcileConfirmer, gateway reconcileGateway) *Reconciler {
	r := NewReconciler(repository, runtime, confirmer, gateway)
	r.SetPendingRecovery(&pendingRecoveryStub{})
	r.SetChannelLimiter(&channelLimiterStub{})
	return r
}

func TestReconcilerRejectsMissingDistributedControls(t *testing.T) {
	r := NewReconciler(&reconcileRepositoryStub{}, reconcileRuntimeStub{}, &reconcileConfirmerStub{}, &reconcileGatewayStub{})
	if err := r.Run(context.Background()); err == nil {
		t.Fatal("missing pending recovery and limiter accepted")
	}
	r.SetPendingRecovery(&pendingRecoveryStub{})
	if err := r.Run(context.Background()); err == nil {
		t.Fatal("missing distributed limiter accepted")
	}
}

type reconcileRepositoryStub struct {
	claims       []ClaimedOrder
	advanced     []time.Time
	closed       int
	advanceError error
	renewError   error
	mu           sync.Mutex
	renewCalls   int
	renewHook    func(int) error
}

func (r *reconcileRepositoryStub) ClaimDue(context.Context, time.Time, int, time.Duration) ([]ClaimedOrder, error) {
	return r.claims, nil
}
func (r *reconcileRepositoryStub) RenewCheck(context.Context, int64, string, time.Time, time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renewCalls++
	if r.renewHook != nil {
		return r.renewHook(r.renewCalls)
	}
	return r.renewError
}
func (r *reconcileRepositoryStub) AdvanceCheck(_ context.Context, _ int64, _ string, next, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.advanced = append(r.advanced, next)
	return r.advanceError
}
func (r *reconcileRepositoryStub) CloseClaimed(context.Context, int64, string, time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed++
	return nil
}

type reconcileRuntimeStub struct{ credentials alipay.Credentials }

func (r reconcileRuntimeStub) Credentials(context.Context, int64, int64) (alipay.Credentials, error) {
	return r.credentials, nil
}

type reconcileGatewayStub struct {
	trade      alipay.Trade
	queryCall  int
	closeCall  int
	closeTrade alipay.Trade
	queryHook  func(context.Context, string) (alipay.Trade, error)
	mu         sync.Mutex
}

func (r *reconcileGatewayStub) Query(ctx context.Context, _ alipay.Credentials, orderNo string) (alipay.Trade, error) {
	r.mu.Lock()
	r.queryCall++
	r.mu.Unlock()
	if r.queryHook != nil {
		return r.queryHook(ctx, orderNo)
	}
	return r.trade, nil
}
func (r *reconcileGatewayStub) Close(context.Context, alipay.Credentials, string) (alipay.Trade, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeCall++
	return r.closeTrade, nil
}

type reconcileConfirmerStub struct {
	called int
	proof  ConfirmedPayment
	mu     sync.Mutex
}

type pendingRecoveryStub struct {
	calls int
	err   error
}

func (r *pendingRecoveryStub) RecoverPending(context.Context) error { r.calls++; return r.err }

func (r *reconcileConfirmerStub) Confirm(_ context.Context, proof ConfirmedPayment, _ *int64) (SettlementResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called++
	r.proof = proof
	return SettlementResult{OrderID: 9}, nil
}

func claimedFixture(now time.Time) ClaimedOrder {
	return ClaimedOrder{Model: Model{ID: 9, OrderNo: "PO202610160000000000000001", UserID: 7, PlatformID: 1, ConfigID: 3, ConfigVersion: 4, AmountCents: 1000, Status: StatusPending, ExpiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-10 * time.Minute)}, Token: "lease-token"}
}

func TestReconcilerClosesExpiredOrderOnlyAfterUnpaidQuery(t *testing.T) {
	now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC)
	repository := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	gateway := &reconcileGatewayStub{trade: alipay.Trade{OrderNo: "PO202610160000000000000001", TradeNo: "T-1", AppID: "app", Status: "WAIT_BUYER_PAY", AmountCents: 1000}, closeTrade: alipay.Trade{OrderNo: "PO202610160000000000000001", TradeNo: "T-1", AppID: "app"}}
	confirmer := &reconcileConfirmerStub{}
	reconciler := testReconciler(repository, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, confirmer, gateway)
	reconciler.now = func() time.Time { return now }
	if err := reconciler.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gateway.queryCall != 1 || gateway.closeCall != 1 || repository.closed != 1 || confirmer.called != 0 {
		t.Fatalf("query=%d close=%d closed=%d confirmed=%d", gateway.queryCall, gateway.closeCall, repository.closed, confirmer.called)
	}
}

func TestReconcilerClosesMissingTradeOnlyAtExpiry(t *testing.T) {
	now := time.Date(2026, 10, 23, 4, 0, 0, 0, time.UTC)
	for _, scenario := range []struct {
		name         string
		expires      time.Time
		wantClosed   int
		wantDeferred int
	}{
		{"expired", now.Add(-time.Minute), 1, 0},
		{"expiry boundary", now, 1, 0},
		{"not expired", now.Add(time.Minute), 0, 1},
		{"missing expiry", time.Time{}, 0, 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			claim := claimedFixture(now)
			claim.ExpiresAt = scenario.expires
			repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claim}}
			gateway := &reconcileGatewayStub{queryHook: func(context.Context, string) (alipay.Trade, error) {
				return alipay.Trade{}, fmt.Errorf("verified query: %w", alipay.ErrTradeNotFound)
			}}
			confirmer := &reconcileConfirmerStub{}
			r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, confirmer, gateway)
			r.now = func() time.Time { return now }
			if err := r.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if repo.closed != scenario.wantClosed || len(repo.advanced) != scenario.wantDeferred || gateway.queryCall != 1 || gateway.closeCall != 0 || confirmer.called != 0 {
				t.Fatalf("closed=%d deferred=%d query=%d close=%d confirmed=%d", repo.closed, len(repo.advanced), gateway.queryCall, gateway.closeCall, confirmer.called)
			}
		})
	}
}

func TestReconcilerMissingTradePreservesLimiterFailure(t *testing.T) {
	now := time.Now().UTC()
	repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	gateway := &reconcileGatewayStub{queryHook: func(context.Context, string) (alipay.Trade, error) {
		return alipay.Trade{}, alipay.ErrTradeNotFound
	}}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	r.now = func() time.Time { return now }
	releaseErr := errors.New("release failed")
	r.SetChannelLimiter(&channelLimiterStub{releaseErr: releaseErr})
	if err := r.Run(context.Background()); !errors.Is(err, releaseErr) || repo.closed != 1 || len(repo.advanced) != 0 {
		t.Fatalf("trusted missing-trade fact lost: closed=%d deferred=%d err=%v", repo.closed, len(repo.advanced), err)
	}
}

func TestReconcilerSettlesPaidQueryBeforeConsideringExpiry(t *testing.T) {
	now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC)
	repository := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	gateway := &reconcileGatewayStub{trade: alipay.Trade{OrderNo: "PO202610160000000000000001", TradeNo: "T-1", AppID: "app", Status: "TRADE_SUCCESS", AmountCents: 1000}}
	confirmer := &reconcileConfirmerStub{}
	reconciler := testReconciler(repository, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, confirmer, gateway)
	reconciler.now = func() time.Time { return now }
	if err := reconciler.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if confirmer.called != 1 || confirmer.proof.ProofSource != ProofSourceQuery || confirmer.proof.AmountCents != 1000 || gateway.closeCall != 0 || repository.closed != 0 {
		t.Fatalf("confirmed=%d proof=%+v close=%d closed=%d", confirmer.called, confirmer.proof, gateway.closeCall, repository.closed)
	}
}

func TestReconcilerRetainsLeaseOnProviderFailureAndSchedulesBackoff(t *testing.T) {
	now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC)
	repository := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	reconciler := testReconciler(repository, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, &reconcileGatewayErrorStub{})
	reconciler.now = func() time.Time { return now }
	if err := reconciler.Run(context.Background()); err == nil {
		t.Fatal("provider failure returned nil")
	}
	if len(repository.advanced) != 1 || !repository.advanced[0].After(now) {
		t.Fatalf("next check = %+v", repository.advanced)
	}
}

func TestReconcilerRejectsUnpaidAndClosedMismatchedFacts(t *testing.T) {
	for _, status := range []string{"WAIT_BUYER_PAY", "TRADE_CLOSED", "TRADE_SUCCESS"} {
		for _, field := range []string{"amount", "app", "seller", "trade"} {
			t.Run(status+"/"+field, func(t *testing.T) {
				now := time.Now().UTC()
				repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
				trade := alipay.Trade{OrderNo: claimedFixture(now).OrderNo, TradeNo: "T-1", AppID: "app", Status: status, AmountCents: 1000}
				switch field {
				case "amount":
					trade.AmountCents++
				case "app":
					trade.AppID = "other"
				case "seller":
					other := "other"
					trade.SellerID = &other
				case "trade":
					trade.TradeNo = ""
				}
				gateway := &reconcileGatewayStub{trade: trade}
				confirmer := &reconcileConfirmerStub{}
				r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", SellerID: "seller", Environment: payment.EnvironmentSandbox}}, confirmer, gateway)
				if err := r.Run(context.Background()); err == nil {
					t.Fatal("mismatched provider facts accepted")
				}
				if confirmer.called != 0 || repo.closed != 0 || gateway.closeCall != 0 || len(repo.advanced) != 1 {
					t.Fatalf("unsafe mismatch handling: confirmed=%d closed=%d closeCalls=%d next=%v", confirmer.called, repo.closed, gateway.closeCall, repo.advanced)
				}
			})
		}
	}
}

func TestReconcilerRejectsMismatchedCloseResponse(t *testing.T) {
	now := time.Now().UTC()
	repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	gateway := &reconcileGatewayStub{trade: alipay.Trade{OrderNo: claimedFixture(now).OrderNo, TradeNo: "T-1", AppID: "app", Status: "WAIT_BUYER_PAY", AmountCents: 1000}, closeTrade: alipay.Trade{OrderNo: "ANOTHER", TradeNo: "T-2", AppID: "app"}}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	if err := r.Run(context.Background()); err == nil || repo.closed != 0 || len(repo.advanced) != 1 {
		t.Fatalf("close response accepted: closed=%d next=%v err=%v", repo.closed, repo.advanced, err)
	}
}

func TestReconcilerStartsEveryClaimImmediately(t *testing.T) {
	now := time.Now().UTC()
	// Seven is below the batch limit, so a single batch proves concurrent I/O
	// without requiring this fake to implement the durable claim query.
	repo := &reconcileRepositoryStub{}
	for i := range 7 {
		claim := claimedFixture(now)
		claim.ID = int64(i + 1)
		claim.OrderNo = fmt.Sprintf("ORDER-%d", i)
		repo.claims = append(repo.claims, claim)
	}
	started := make(chan struct{}, 7)
	release := make(chan struct{})
	gateway := &reconcileGatewayStub{queryHook: func(ctx context.Context, no string) (alipay.Trade, error) {
		started <- struct{}{}
		select {
		case <-release:
			return alipay.Trade{OrderNo: no, TradeNo: "T-1", AppID: "app", Status: "TRADE_CLOSED", AmountCents: 1000}, nil
		case <-ctx.Done():
			return alipay.Trade{}, ctx.Err()
		}
	}}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	for range 7 {
		select {
		case <-started:
		case <-ctx.Done():
			close(release)
			<-done
			t.Fatal("claimed orders waited locally instead of starting together")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReconcilerRecoversPendingCallbacksBeforeOrderChecks(t *testing.T) {
	repo := &reconcileRepositoryStub{}
	recovery := &pendingRecoveryStub{}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, &reconcileGatewayStub{})
	r.SetPendingRecovery(recovery)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if recovery.calls != 1 {
		t.Fatalf("pending recovery calls=%d", recovery.calls)
	}
}

func TestReconcilerReportsPendingRecoveryFailureAndStillProcessesOrders(t *testing.T) {
	now := time.Now().UTC()
	repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	recovery := &pendingRecoveryStub{err: errors.New("pending recovery unavailable")}
	gateway := &reconcileGatewayStub{trade: alipay.Trade{OrderNo: claimedFixture(now).OrderNo, TradeNo: "T-1", AppID: "app", Status: "TRADE_SUCCESS", AmountCents: 1000}}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	r.SetPendingRecovery(recovery)
	if err := r.Run(context.Background()); err == nil || recovery.calls != 1 {
		t.Fatalf("recovery failure hidden: %v", err)
	}
}

type channelLimiterStub struct {
	reserveErr, releaseErr error
	mu                     sync.Mutex
	reserved, released     int
	releaseContextErr      error
}

func (l *channelLimiterStub) Reserve(ctx context.Context, _ payment.Environment, _ string) (func(context.Context) error, error) {
	l.mu.Lock()
	l.reserved++
	l.mu.Unlock()
	if l.reserveErr != nil {
		return nil, l.reserveErr
	}
	return func(cleanup context.Context) error {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.released++
		l.releaseContextErr = cleanup.Err()
		return l.releaseErr
	}, nil
}

func TestReconcilerChannelAdmissionFailureNeverCallsProvider(t *testing.T) {
	now := time.Now().UTC()
	repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	gateway := &reconcileGatewayStub{}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	limiter := &channelLimiterStub{reserveErr: errors.New("Redis unavailable")}
	r.SetChannelLimiter(limiter)
	if err := r.Run(context.Background()); err == nil {
		t.Fatal("channel failure hidden")
	}
	if gateway.queryCall != 0 || gateway.closeCall != 0 || len(repo.advanced) != 1 {
		t.Fatalf("provider called without channel budget: query=%d close=%d next=%v", gateway.queryCall, gateway.closeCall, repo.advanced)
	}
}

func TestReconcilerPaidProofSurvivesChannelReleaseFailure(t *testing.T) {
	now := time.Now().UTC()
	repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	gateway := &reconcileGatewayStub{trade: alipay.Trade{OrderNo: claimedFixture(now).OrderNo, TradeNo: "T-1", AppID: "app", Status: "TRADE_SUCCESS", AmountCents: 1000}}
	confirmer := &reconcileConfirmerStub{}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, confirmer, gateway)
	limiter := &channelLimiterStub{releaseErr: errors.New("Redis release unavailable")}
	r.SetChannelLimiter(limiter)
	if err := r.Run(context.Background()); err == nil {
		t.Fatal("release failure hidden from Scheduler")
	}
	if confirmer.called != 1 || limiter.released != 1 || len(repo.advanced) != 0 {
		t.Fatalf("trusted paid fact was dropped: confirm=%d releases=%d next=%v", confirmer.called, limiter.released, repo.advanced)
	}
}

func TestReconcilerCancellationStillReleasesChannelSlot(t *testing.T) {
	now := time.Now().UTC()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
	gateway := &reconcileGatewayStub{queryHook: func(context.Context, string) (alipay.Trade, error) { cancel(); return alipay.Trade{}, context.Canceled }}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	limiter := &channelLimiterStub{}
	r.SetChannelLimiter(limiter)
	if err := r.Run(ctx); err == nil {
		t.Fatal("canceled task returned success")
	}
	if limiter.released != 1 || limiter.releaseContextErr != nil {
		t.Fatalf("cancellation leaked channel slot: released=%d cleanup=%v", limiter.released, limiter.releaseContextErr)
	}
}

func TestReconcilerStopsBeforeCloseWhenRenewalLosesLease(t *testing.T) {
	now := time.Now().UTC()
	repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}, renewHook: func(call int) error {
		if call >= 2 {
			return ErrReconcileLeaseLost
		}
		return nil
	}}
	gateway := &reconcileGatewayStub{trade: alipay.Trade{OrderNo: claimedFixture(now).OrderNo, TradeNo: "T-1", AppID: "app", Status: "WAIT_BUYER_PAY", AmountCents: 1000}}
	r := testReconciler(repo, reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: payment.EnvironmentSandbox}}, &reconcileConfirmerStub{}, gateway)
	if err := r.Run(context.Background()); !errors.Is(err, ErrReconcileLeaseLost) {
		t.Fatalf("lease loss=%v", err)
	}
	if gateway.queryCall != 1 || gateway.closeCall != 0 || repo.closed != 0 {
		t.Fatal("lost owner continued external I/O")
	}
}

type reconcileGatewayErrorStub struct{}

func (reconcileGatewayErrorStub) Query(context.Context, alipay.Credentials, string) (alipay.Trade, error) {
	return alipay.Trade{}, errors.New("provider unavailable")
}
func (reconcileGatewayErrorStub) Close(context.Context, alipay.Credentials, string) (alipay.Trade, error) {
	return alipay.Trade{}, errors.New("provider unavailable")
}
