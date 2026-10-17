package order

import (
	"context"
	"testing"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment"
)

func TestCreateRejectsMalformedReturnURLBeforeWritingOrder(t *testing.T) {
	for _, raw := range []string{
		"", "/payment/recharge", "//app.example.test/payment/recharge", "javascript:alert(1)",
		"ftp://app.example.test/paid", "https://user:password@app.example.test/paid",
		"https://app.example.test/paid#private", "https://:443/paid", "https://*.example.test/paid",
		"https://app.example.test:/paid", "https://app.example.test:0/paid", "https://app.example.test:65536/paid",
		"https://app.example.test:PORT/paid", "https://app.example.test\\evil/paid", " https://app.example.test/paid",
	} {
		t.Run(raw, func(t *testing.T) {
			repo := &createRepositoryStub{}
			service := createServiceFixture(repo)
			if _, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, CreateInput{RequestID: "00000000-0000-4000-8000-000000000001", PackageID: 8, PayMethod: payment.PayMethodWeb, ReturnURL: raw}); err == nil || repo.created != nil {
				t.Fatalf("malformed return URL accepted: raw=%q created=%v err=%v", raw, repo.created != nil, err)
			}
		})
	}
}

func TestRepositoryReadsOrderChannelEnvironmentWithoutDeploymentSetting(t *testing.T) {
	db, ctx := settlementDB(t)
	setupSettlementRows(t, db, ctx, StatusPending, nil)
	repo := NewRepository(db)
	for _, environment := range []payment.Environment{payment.EnvironmentSandbox, payment.EnvironmentProduction} {
		if err := db.WithContext(ctx).Exec(`UPDATE payment_config SET environment = ? WHERE id = 1`, environment).Error; err != nil {
			t.Fatal(err)
		}
		row, err := repo.Find(ctx, 1)
		if err != nil || row.ID != 1 || row.ConfigEnvironment != environment {
			t.Fatalf("find environment=%d row=%+v err=%v", environment, row, err)
		}
		row, err = repo.FindByRequest(ctx, 7, 1, "00000000-0000-4000-8000-000000000001")
		if err != nil || row.ID != 1 || row.ConfigEnvironment != environment {
			t.Fatalf("request lookup environment=%d row=%+v err=%v", environment, row, err)
		}
		list, err := repo.List(ctx, ListQuery{Page: 1, PageSize: 20, UserID: 7, PlatformID: 1, Personal: true})
		if err != nil || len(list) != 1 || list[0].ConfigEnvironment != environment {
			t.Fatalf("list environment=%d rows=%+v err=%v", environment, list, err)
		}
		value, err := NewService(repo).GetForIdentity(ctx, auth.Identity{UserID: 7, PlatformID: 1}, 1)
		if err != nil || value.Actions.Pay != (environment == payment.EnvironmentSandbox) {
			t.Fatalf("environment=%d actions=%+v err=%v", environment, value.Actions, err)
		}
	}
}
