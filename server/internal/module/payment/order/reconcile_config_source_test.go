package order

import (
	"context"
	"testing"
	"time"

	"admin/server/internal/module/payment"
	"admin/server/internal/payment/alipay"
)

type environmentChannelLimiter struct{ environment payment.Environment }

func (l *environmentChannelLimiter) Reserve(_ context.Context, environment payment.Environment, _ string) (func(context.Context) error, error) {
	l.environment = environment
	return func(context.Context) error { return nil }, nil
}

func TestReconcileUsesOrderCredentialEnvironmentForGatewayBudgetAndProof(t *testing.T) {
	for _, environment := range []payment.Environment{payment.EnvironmentSandbox, payment.EnvironmentProduction} {
		now := time.Now().UTC()
		repo := &reconcileRepositoryStub{claims: []ClaimedOrder{claimedFixture(now)}}
		gateway := &reconcileGatewayStub{trade: alipay.Trade{OrderNo: "PO202610160000000000000001", TradeNo: "T-1", AppID: "app", Status: "TRADE_SUCCESS", AmountCents: 1000}}
		confirmer := &reconcileConfirmerStub{}
		limiter := &environmentChannelLimiter{}
		r := &Reconciler{repository: repo, runtime: reconcileRuntimeStub{credentials: alipay.Credentials{AppID: "app", Environment: environment}}, confirmer: confirmer, gateway: gateway, pending: &pendingRecoveryStub{}, limiter: limiter, now: func() time.Time { return now }}
		if err := r.Run(context.Background()); err != nil {
			t.Fatalf("environment=%d reconcile err=%v", environment, err)
		}
		if gateway.queryCall != 1 || confirmer.called != 1 || confirmer.proof.Environment != environment || limiter.environment != environment {
			t.Fatalf("environment=%d gateway=%d confirms=%d proof=%d limiter=%d", environment, gateway.queryCall, confirmer.called, confirmer.proof.Environment, limiter.environment)
		}
	}
}
