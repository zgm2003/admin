package order

import (
	"context"
	"errors"
	"testing"
	"time"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment"
	paymentconfig "admin/server/internal/module/payment/config"
	rechargepackage "admin/server/internal/module/payment/rechargePackage"
	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type createRepositoryStub struct {
	existing  *Model
	created   *Model
	findErr   error
	createErr error
}

func (r *createRepositoryStub) Confirm(context.Context, ConfirmedPayment, *int64, time.Time) (SettlementResult, error) {
	return SettlementResult{}, errors.New("unused")
}

func (r *createRepositoryStub) FindByRequest(context.Context, int64, int64, string) (Model, error) {
	if r.findErr != nil {
		return Model{}, r.findErr
	}
	if r.existing == nil {
		return Model{}, gorm.ErrRecordNotFound
	}
	return *r.existing, nil
}

func (r *createRepositoryStub) Create(_ context.Context, row *Model) error {
	if r.createErr != nil {
		return r.createErr
	}
	row.ID = 10
	copy := *row
	r.created = &copy
	return nil
}

type createRuntimeStub struct {
	catalog    []paymentconfig.CatalogItem
	credential paymentconfig.BoundCredential
}

func (r createRuntimeStub) Catalog(context.Context) ([]paymentconfig.CatalogItem, error) {
	return r.catalog, nil
}

func (r createRuntimeStub) Credential(context.Context, int64, int64) (paymentconfig.BoundCredential, error) {
	return r.credential, nil
}

func (r createRuntimeStub) Credentials(context.Context, int64, int64) (alipay.Credentials, error) {
	return alipay.Credentials{AppID: r.credential.AppID, SellerID: r.credential.SellerID, Environment: r.credential.Environment, NotifyURL: r.credential.Version.NotifyURL, PrivateKey: "unused", AppCertPEM: "unused", AlipayCertPEM: "unused", RootCertPEM: "unused"}, nil
}

type createPackageStub struct{ rows []rechargepackage.CatalogItem }

func (r createPackageStub) Catalog(context.Context) ([]rechargepackage.CatalogItem, error) {
	return r.rows, nil
}

func createServiceFixture(repo *createRepositoryStub) *Service {
	service := NewService(repo)
	service.SetRuntime(createRuntimeStub{
		catalog:    []paymentconfig.CatalogItem{{ID: 3, Name: "Sandbox channel", Provider: payment.ProviderAlipay, Environment: payment.EnvironmentSandbox, AppID: "20260001", SellerID: "20880001", CurrentVersion: 4, IsEnabled: yesno.Yes}},
		credential: paymentconfig.BoundCredential{ConfigID: 3, Provider: payment.ProviderAlipay, AppID: "20260001", SellerID: "20880001", Environment: payment.EnvironmentSandbox, Version: paymentconfig.CredentialVersion{ConfigID: 3, Version: 4, NotifyURL: "https://pay.example.test/callback", PayMethods: paymentconfig.PayMethods{payment.PayMethodWeb, payment.PayMethodH5}}},
	}, createPackageStub{rows: []rechargepackage.CatalogItem{{ID: 8, Name: "100 credits", AmountCents: 1000}}}, payment.EnvironmentSandbox)
	service.SetReturnOrigins([]string{"https://app.example.test"})
	service.SetCheckout(func(context.Context, alipay.Credentials, alipay.CheckoutInput) (string, error) {
		return "https://pay.example.test/checkout", nil
	})
	service.now = func() time.Time { return time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC) }
	service.newOrderNo = func(time.Time) (string, error) { return "PO2026101503000000000000000001", nil }
	return service
}

func TestCreateUsesServerPackageAndHistoricalCredentialSnapshot(t *testing.T) {
	repo := &createRepositoryStub{}
	service := createServiceFixture(repo)
	got, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, CreateInput{RequestID: "00000000-0000-4000-8000-000000000001", PackageID: 8, PayMethod: payment.PayMethodH5, ReturnURL: "https://app.example.test/paid?order=1"})
	if err != nil {
		t.Fatalf("%v: %v", err, errors.Unwrap(err))
	}
	if got.URL != "https://pay.example.test/checkout" || got.Order.Amount != "10.00" || got.Order.Status != StatusPending {
		t.Fatalf("checkout = %+v", got)
	}
	if repo.created == nil || repo.created.AmountCents != 1000 || repo.created.ConfigID != 3 || repo.created.ConfigVersion != 4 || repo.created.PayMethod != int16(payment.PayMethodH5) || repo.created.UserID != 7 || repo.created.PlatformID != 1 {
		t.Fatalf("created order = %+v", repo.created)
	}
	if repo.created.NextCheckAt == nil || !repo.created.NextCheckAt.Equal(time.Date(2026, 10, 15, 3, 1, 0, 0, time.UTC)) {
		t.Fatalf("created order next check = %v", repo.created.NextCheckAt)
	}
}

func TestCreateReplaysSameRequestAndRejectsDifferentFingerprint(t *testing.T) {
	base := Model{ID: 10, OrderNo: "PO2026101503000000000000000001", UserID: 7, PlatformID: 1, PackageID: 8, PackageName: "100 credits", AmountCents: 1000, ConfigID: 3, ConfigVersion: 4, PayMethod: int16(payment.PayMethodWeb), RequestID: "00000000-0000-4000-8000-000000000001", RequestFingerprint: requestFingerprint(CreateInput{RequestID: "00000000-0000-4000-8000-000000000001", PackageID: 8, PayMethod: payment.PayMethodWeb, ReturnURL: "https://app.example.test/paid"}), ReturnURL: "https://app.example.test/paid", Status: StatusPending, ExpiresAt: time.Date(2026, 10, 15, 3, 30, 0, 0, time.UTC), CreatedAt: time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)}
	t.Run("same input replays", func(t *testing.T) {
		repo := &createRepositoryStub{existing: &base}
		service := createServiceFixture(repo)
		got, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, CreateInput{RequestID: base.RequestID, PackageID: 8, PayMethod: payment.PayMethodWeb, ReturnURL: base.ReturnURL})
		if err != nil {
			t.Fatal(err)
		}
		if got.Order.ID != base.ID || got.URL == "" || repo.created != nil {
			t.Fatalf("replay = %+v created=%+v", got, repo.created)
		}
	})
	t.Run("different input conflicts", func(t *testing.T) {
		repo := &createRepositoryStub{existing: &base}
		service := createServiceFixture(repo)
		_, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, CreateInput{RequestID: base.RequestID, PackageID: 8, PayMethod: payment.PayMethodH5, ReturnURL: base.ReturnURL})
		if err == nil {
			t.Fatal("request fingerprint conflict accepted")
		}
	})
}
