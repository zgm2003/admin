package order

import (
	"context"
	"errors"
	"testing"
	"time"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment"
	"admin/server/internal/payment/alipay"
)

func TestCreateUsesConfiguredChannelAndRequestReturnURLWithoutDeploymentSettings(t *testing.T) {
	for _, returnURL := range []string{"http://localhost:16300/payment/recharge?from=wallet", "https://another-frontend.example.test/payment/recharge?from=wallet"} {
		t.Run(returnURL, func(t *testing.T) {
			repo := &createRepositoryStub{}
			fixture := createServiceFixture(repo)
			service := NewService(repo)
			service.runtime, service.packages = fixture.runtime, fixture.packages
			service.SetSettings(fixture.settings)
			service.now, service.newOrderNo = fixture.now, fixture.newOrderNo
			var gotCredentials alipay.Credentials
			var gotInput alipay.CheckoutInput
			service.SetCheckout(func(_ context.Context, credentials alipay.Credentials, input alipay.CheckoutInput) (string, error) {
				gotCredentials, gotInput = credentials, input
				return "https://pay.example.test/checkout", nil
			})
			options, err := service.Options(context.Background())
			if err != nil || !options.Available {
				t.Fatalf("configured sandbox options=%+v err=%v", options, err)
			}
			got, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, CreateInput{RequestID: "00000000-0000-4000-8000-000000000001", PackageID: 8, PayMethod: payment.PayMethodWeb, ReturnURL: returnURL})
			if err != nil || got.URL == "" || repo.created == nil {
				t.Fatalf("checkout=%+v order=%+v err=%v", got, repo.created, err)
			}
			if gotCredentials.Environment != payment.EnvironmentSandbox || gotCredentials.NotifyURL != "https://pay.example.test/callback" || gotInput.ReturnURL != returnURL || repo.created.ReturnURL != returnURL {
				t.Fatalf("configured channel or caller return URL was replaced: environment=%d notify=%q return=%q", gotCredentials.Environment, gotCredentials.NotifyURL, gotInput.ReturnURL)
			}
		})
	}
}

func TestConfirmDoesNotRequireProcessPaymentEnvironment(t *testing.T) {
	for _, environment := range []payment.Environment{payment.EnvironmentSandbox, payment.EnvironmentProduction} {
		repo := &confirmerStub{}
		proof := validConfirmation()
		proof.Environment = environment
		got, err := NewService(repo).Confirm(context.Background(), proof, nil)
		if err != nil || got.TransactionID != 2 || repo.calls != 1 {
			t.Fatalf("environment=%d settlement=%+v calls=%d err=%v", environment, got, repo.calls, err)
		}
	}
}

func TestConfirmRejectsEvidenceFromAnotherConfiguredEnvironment(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, nil)
	proof := validConfirmation()
	proof.Environment = payment.EnvironmentProduction
	_, err := NewRepository(db).Confirm(ctx, proof, nil, time.Now().UTC())
	if !errors.Is(err, ErrConfigMismatch) {
		t.Fatalf("cross-environment evidence err=%v", err)
	}
	var count int64
	if err := db.WithContext(ctx).Table("payment_transaction").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("cross-environment evidence wrote a ledger: count=%d err=%v", count, err)
	}
}
