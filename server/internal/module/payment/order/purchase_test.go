package order

import (
	"context"
	"errors"
	"testing"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment"
	"admin/server/internal/shared/apperror"
)

func productionServiceFixture(repo *createRepositoryStub) *Service {
	service := createServiceFixture(repo)
	runtime := service.runtime.(createRuntimeStub)
	runtime.catalog[0].Environment = payment.EnvironmentProduction
	runtime.credential.Environment = payment.EnvironmentProduction
	service.runtime = runtime
	service.environment = payment.EnvironmentProduction
	return service
}

func TestProductionPurchasesStayClosedUntilRecoveryIsDelivered(t *testing.T) {
	ctx := context.Background()
	actor := auth.Identity{UserID: 7, PlatformID: 1}
	input := CreateInput{RequestID: "00000000-0000-4000-8000-000000000001", PackageID: 8, PayMethod: payment.PayMethodWeb, ReturnURL: "https://app.example.test/paid"}
	repo := &createRepositoryStub{}
	service := productionServiceFixture(repo)
	options, err := service.Options(ctx)
	if err != nil || len(options.PayMethods) != 2 || len(options.Packages) != 1 || options.Available {
		t.Errorf("production options=%+v err=%v", options, err)
	}
	got, err := service.Create(ctx, actor, input)
	var public *apperror.Error
	if !errors.As(err, &public) || public.Code != apperror.CodeDependencyUnavailable || got.URL != "" || repo.created != nil {
		t.Errorf("production create=%+v persisted=%v err=%v", got, repo.created != nil, err)
	}
	readRepo := &orderReadRepository{rows: []Model{validReadOrder()}}
	row := &readRepo.rows[0]
	row.ConfigID, row.ConfigVersion, row.PayMethod = 3, 4, int16(payment.PayMethodWeb)
	row.ReturnURL = input.ReturnURL
	service.repository = readRepo
	got, err = service.Pay(ctx, actor, row.ID)
	public = nil
	if !errors.As(err, &public) || public.Code != apperror.CodeDependencyUnavailable || got.URL != "" {
		t.Errorf("production pay=%+v err=%v", got, err)
	}
	value, err := service.GetForIdentity(ctx, actor, row.ID)
	if err != nil || value.ID != row.ID || value.Actions.Pay {
		t.Errorf("production read=%+v err=%v", value, err)
	}
}

func TestProductionPurchaseGateDoesNotBlockHistoricalSettlement(t *testing.T) {
	repo := &confirmerStub{}
	service := NewService(repo)
	service.environment = payment.EnvironmentProduction
	proof := validConfirmation()
	proof.Environment = payment.EnvironmentProduction
	got, err := service.Confirm(context.Background(), proof, nil)
	if err != nil || repo.calls != 1 || got.TransactionID != 2 {
		t.Fatalf("historical production settlement=%+v calls=%d err=%v", got, repo.calls, err)
	}
}
