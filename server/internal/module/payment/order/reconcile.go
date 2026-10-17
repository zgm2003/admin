package order

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"admin/server/internal/module/payment"
	"admin/server/internal/payment/alipay"
)

const (
	ReconcileTaskType   = "payment.order.reconcile"
	reconcileBatchSize  = 8
	reconcileMaxBatches = 20
	reconcileLease      = 15 * time.Second
	reconcileWorkBudget = 50 * time.Second
	reconcileIOBudget   = 5 * time.Second
)

// ClaimedOrder is the small immutable coordinate taken in the lease
// transaction. Provider I/O happens after the transaction commits.
type ClaimedOrder struct {
	Model
	Token string
}

type reconcileRepository interface {
	ClaimDue(context.Context, time.Time, int, time.Duration) ([]ClaimedOrder, error)
	RenewCheck(context.Context, int64, string, time.Time, time.Duration) error
	AdvanceCheck(context.Context, int64, string, time.Time, time.Time) error
	CloseClaimed(context.Context, int64, string, time.Time) error
}

type reconcileRuntime interface {
	Credentials(context.Context, int64, int64) (alipay.Credentials, error)
}

type reconcileConfirmer interface {
	Confirm(context.Context, ConfirmedPayment, *int64) (SettlementResult, error)
}

type pendingCallbackRecovery interface {
	RecoverPending(context.Context) error
}

type channelLimiter interface {
	Reserve(context.Context, payment.Environment, string) (func(context.Context) error, error)
}

type reconcileGateway interface {
	Query(context.Context, alipay.Credentials, string) (alipay.Trade, error)
	Close(context.Context, alipay.Credentials, string) (alipay.Trade, error)
}

type Reconciler struct {
	repository reconcileRepository
	runtime    reconcileRuntime
	confirmer  reconcileConfirmer
	gateway    reconcileGateway
	pending    pendingCallbackRecovery
	limiter    channelLimiter
	now        func() time.Time
}

func (r *Reconciler) SetPendingRecovery(value pendingCallbackRecovery) {
	if r != nil {
		r.pending = value
	}
}

func (r *Reconciler) SetChannelLimiter(value channelLimiter) {
	if r != nil {
		r.limiter = value
	}
}

func NewReconciler(repository reconcileRepository, runtime reconcileRuntime, confirmer reconcileConfirmer, gateway reconcileGateway) *Reconciler {
	if gateway == nil {
		gateway = alipayGateway{}
	}
	return &Reconciler{repository: repository, runtime: runtime, confirmer: confirmer, gateway: gateway, now: time.Now}
}

type alipayGateway struct{}

func (alipayGateway) Query(ctx context.Context, credentials alipay.Credentials, orderNo string) (alipay.Trade, error) {
	return alipay.Query(ctx, credentials, orderNo)
}
func (alipayGateway) Close(ctx context.Context, credentials alipay.Credentials, orderNo string) (alipay.Trade, error) {
	return alipay.Close(ctx, credentials, orderNo)
}

type channelCallResult struct {
	trade               alipay.Trade
	callErr, releaseErr error
}

func (r *Reconciler) providerCall(ctx context.Context, credentials alipay.Credentials, call func(context.Context) (alipay.Trade, error)) channelCallResult {
	ioCtx, cancel := context.WithTimeout(ctx, reconcileIOBudget)
	defer cancel()
	release, err := r.limiter.Reserve(ioCtx, credentials.Environment, credentials.AppID)
	if err != nil {
		return channelCallResult{callErr: err}
	}
	if release == nil {
		return channelCallResult{callErr: ErrChannelUnavailable}
	}
	var trade alipay.Trade
	callErr := ioCtx.Err()
	if callErr == nil {
		trade, callErr = call(ioCtx)
	}
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 500*time.Millisecond)
	releaseErr := release(cleanup)
	stop()
	return channelCallResult{trade: trade, callErr: callErr, releaseErr: releaseErr}
}

func (r *Reconciler) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("payment reconcile context is required")
	}
	if r == nil || r.repository == nil || r.runtime == nil || r.confirmer == nil || r.gateway == nil || r.pending == nil || r.limiter == nil {
		return errors.New("payment reconcile dependencies are missing")
	}
	workCtx, cancel := context.WithTimeout(ctx, reconcileWorkBudget)
	defer cancel()
	var runErr error
	if r.pending != nil {
		if err := r.pending.RecoverPending(workCtx); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}
	for batch := 0; batch < reconcileMaxBatches; batch++ {
		if err := workCtx.Err(); err != nil {
			return errors.Join(runErr, err)
		}
		now := r.clockNow()
		claimed, err := r.repository.ClaimDue(workCtx, now, reconcileBatchSize, reconcileLease)
		if err != nil {
			return errors.Join(runErr, err)
		}
		if len(claimed) == 0 {
			return runErr
		}
		var batchErr error
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, order := range claimed {
			order := order
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := r.process(workCtx, order, now); err != nil {
					mu.Lock()
					batchErr = errors.Join(batchErr, fmt.Errorf("reconcile order %d: %w", order.ID, err))
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		runErr = errors.Join(runErr, batchErr)
		if err := workCtx.Err(); err != nil {
			return errors.Join(runErr, err)
		}
		if len(claimed) < reconcileBatchSize {
			return runErr
		}
	}
	return runErr
}

func (r *Reconciler) process(ctx context.Context, claimed ClaimedOrder, now time.Time) error {
	if claimed.ID < 1 || claimed.Token == "" || claimed.OrderNo == "" || claimed.ConfigID < 1 || claimed.ConfigVersion < 1 || claimed.AmountCents < 1 || claimed.Status != StatusPending {
		return errors.New("claimed payment order is invalid")
	}
	credentials, err := r.runtime.Credentials(ctx, claimed.ConfigID, claimed.ConfigVersion)
	if err != nil {
		return r.deferOrder(ctx, claimed, now, err)
	}
	if !credentials.Environment.Valid() || credentials.AppID == "" {
		return r.deferOrder(ctx, claimed, now, ErrConfigMismatch)
	}
	if err := r.repository.RenewCheck(ctx, claimed.ID, claimed.Token, r.clockNow(), reconcileLease); err != nil {
		return err
	}
	query := r.providerCall(ctx, credentials, func(ioCtx context.Context) (alipay.Trade, error) {
		return r.gateway.Query(ioCtx, credentials, claimed.OrderNo)
	})
	if query.callErr != nil {
		return r.deferOrder(ctx, claimed, now, errors.Join(query.callErr, query.releaseErr))
	}
	trade := query.trade
	if trade.OrderNo != claimed.OrderNo || trade.TradeNo == "" {
		return r.deferOrder(ctx, claimed, now, ErrOrderMismatch)
	}
	if trade.AppID != credentials.AppID || (trade.SellerID != nil && *trade.SellerID != credentials.SellerID) {
		return r.deferOrder(ctx, claimed, now, ErrIdentityMismatch)
	}
	if trade.AmountCents != claimed.AmountCents {
		return r.deferOrder(ctx, claimed, now, ErrAmountMismatch)
	}
	if isPaidTrade(trade) {
		if trade.TradeNo == "" || trade.AmountCents < 1 || trade.AppID != credentials.AppID {
			return r.deferOrder(ctx, claimed, now, ErrAmountMismatch)
		}
		_, err := r.confirmer.Confirm(ctx, ConfirmedPayment{ConfigID: claimed.ConfigID, ConfigVersion: claimed.ConfigVersion, Environment: credentials.Environment, OrderNo: trade.OrderNo, TradeNo: trade.TradeNo, AppID: trade.AppID, SellerID: trade.SellerID, ProofSource: ProofSourceQuery, AmountCents: trade.AmountCents, Status: trade.Status, PaidAt: trade.PaidAt}, nil)
		if err != nil {
			return r.deferOrder(ctx, claimed, now, errors.Join(err, query.releaseErr))
		}
		return query.releaseErr
	}
	if trade.Status == "TRADE_CLOSED" {
		return errors.Join(r.closeClaimed(ctx, claimed, now), query.releaseErr)
	}
	if query.releaseErr != nil {
		return r.deferOrder(ctx, claimed, now, query.releaseErr)
	}
	if trade.Status != "WAIT_BUYER_PAY" {
		return r.deferOrder(ctx, claimed, now, errors.New("payment provider status is not actionable"))
	}
	if !claimed.ExpiresAt.After(now) {
		if err := r.repository.RenewCheck(ctx, claimed.ID, claimed.Token, r.clockNow(), reconcileLease); err != nil {
			return err
		}
		close := r.providerCall(ctx, credentials, func(ioCtx context.Context) (alipay.Trade, error) {
			return r.gateway.Close(ioCtx, credentials, claimed.OrderNo)
		})
		if close.callErr != nil {
			return r.deferOrder(ctx, claimed, now, errors.Join(close.callErr, close.releaseErr))
		}
		if close.trade.OrderNo != claimed.OrderNo || close.trade.AppID != credentials.AppID || close.trade.TradeNo != trade.TradeNo {
			return r.deferOrder(ctx, claimed, now, errors.Join(ErrOrderMismatch, close.releaseErr))
		}
		return errors.Join(r.closeClaimed(ctx, claimed, now), close.releaseErr)
	}
	return r.deferOrder(ctx, claimed, now, nil)
}

func (r *Reconciler) closeClaimed(ctx context.Context, claimed ClaimedOrder, now time.Time) error {
	now = r.clockNow()
	if err := r.repository.CloseClaimed(ctx, claimed.ID, claimed.Token, now); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) deferOrder(ctx context.Context, claimed ClaimedOrder, now time.Time, cause error) error {
	now = r.clockNow()
	next := nextCheckAt(now, claimed.CreatedAt)
	if err := r.repository.AdvanceCheck(ctx, claimed.ID, claimed.Token, next, now); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func nextCheckAt(now, created time.Time) time.Time {
	if created.IsZero() || now.Sub(created) < 5*time.Minute {
		return now.Add(2 * time.Minute)
	}
	return now.Add(5 * time.Minute)
}

func (r *Reconciler) clockNow() time.Time {
	if r != nil && r.now != nil {
		return r.now().UTC()
	}
	return time.Now().UTC()
}

func isPaidTrade(trade alipay.Trade) bool {
	return trade.Status == "TRADE_SUCCESS" || trade.Status == "TRADE_FINISHED"
}
