package order

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	auth "admin/server/internal/module/auth/login"
	rechargepackage "admin/server/internal/module/payment/rechargePackage"
	"admin/server/internal/shared/apperror"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type rechargeSettingsStub struct {
	row sharedsetting.Record
	err error
}

func (r rechargeSettingsStub) FindByKey(ctx context.Context, key string) (sharedsetting.Record, error) {
	if err := ctx.Err(); err != nil {
		return sharedsetting.Record{}, err
	}
	return r.row, r.err
}

func TestCreateRequestAcceptsExactlyOneRechargeSource(t *testing.T) {
	const envelope = `"requestId":"00000000-0000-4000-8000-000000000001","payMethod":1,"returnUrl":"https://app.example.test/paid"`
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"package", `"packageId":8`, true},
		{"custom minimum", `"amount":"10.00"`, true},
		{"custom decimal", `"amount":"25.39"`, true},
		{"custom maximum", `"amount":"92233720368.54"`, true},
		{"both sources", `"packageId":8,"amount":"10.00"`, false},
		{"neither source", `"amount":null`, false},
		{"empty amount", `"amount":""`, false},
		{"negative amount", `"amount":"-10"`, false},
		{"zero amount", `"amount":"0"`, false},
		{"numeric amount", `"amount":10`, false},
		{"too many decimals", `"amount":"10.001"`, false},
		{"overflow amount", `"amount":"92233720368.55"`, false},
		{"exponent", `"amount":"1e2"`, false},
		{"invalid package", `"packageId":0`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req createRequest
			err := json.Unmarshal([]byte(`{`+envelope+`,`+tc.source+`}`), &req)
			if err == nil {
				_, err = req.input()
			}
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func customRequest(t *testing.T, amount string) CreateInput {
	t.Helper()
	var req createRequest
	if err := json.Unmarshal([]byte(`{"requestId":"00000000-0000-4000-8000-000000000001","amount":"`+amount+`","payMethod":1,"returnUrl":"https://app.example.test/paid"}`), &req); err != nil {
		t.Fatal(err)
	}
	in, err := req.input()
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestCustomRechargeFreezesAmountWithoutPackage(t *testing.T) {
	repo := &createRepositoryStub{}
	service := createServiceFixture(repo)
	got, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, customRequest(t, "25.39"))
	if err != nil || got.Order.Amount != "25.39" || repo.created == nil || repo.created.AmountCents != 2539 || repo.created.PackageID != nil {
		t.Fatalf("custom checkout=%+v created=%+v err=%v", got, repo.created, err)
	}
}

func TestCustomRechargeRejectsBelowMinimumBeforePersisting(t *testing.T) {
	repo := &createRepositoryStub{}
	_, err := createServiceFixture(repo).Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, customRequest(t, "9.99"))
	var public *apperror.Error
	if !errors.As(err, &public) || public.Code != apperror.CodeInvalidRequest || repo.created != nil {
		t.Fatalf("below minimum created=%+v err=%v", repo.created, err)
	}
}

func TestCustomRechargeReplaysFrozenAmountAndRejectsChangedAmount(t *testing.T) {
	repo := &createRepositoryStub{}
	service := createServiceFixture(repo)
	input := customRequest(t, "25.39")
	if _, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, input); err != nil {
		t.Fatal(err)
	}
	repo.existing = repo.created
	repo.created = nil
	// An existing frozen order must replay even when current settings are unavailable.
	service.SetSettings(rechargeSettingsStub{err: errors.New("settings offline")})
	service.now = func() time.Time { return repo.existing.CreatedAt.Add(time.Minute) }
	got, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, input)
	if err != nil || got.Order.Amount != "25.39" || repo.created != nil {
		t.Fatalf("replay=%+v err=%v", got, err)
	}
	_, err = service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, customRequest(t, "25.40"))
	var public *apperror.Error
	if !errors.As(err, &public) || public.Code != apperror.CodeConflict || repo.created != nil {
		t.Fatalf("changed amount err=%v", err)
	}
}

func TestPackageFingerprintPreservesHistoricalBytes(t *testing.T) {
	input := customRequest(t, "10")
	input.PackageID, input.AmountCents = 8, 0
	expected := sha256.Sum256([]byte(input.RequestID + "\x008\x001\x00" + input.ReturnURL))
	if fingerprintText(requestFingerprint(input)) != fingerprintText(expected[:]) {
		t.Fatal("existing package fingerprint changed")
	}
}

func TestCustomRechargeReadsCurrentMinimumAndFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		enabled     yesno.Value
		typeID      int
		missing     bool
	}{
		{"raised minimum", "26", yesno.Yes, sharedsetting.ValueTypeNumber, false},
		{"disabled", "10", yesno.No, sharedsetting.ValueTypeNumber, false},
		{"wrong type", "10", yesno.Yes, sharedsetting.ValueTypeString, false},
		{"invalid", "NaN", yesno.Yes, sharedsetting.ValueTypeNumber, false},
		{"zero", "0", yesno.Yes, sharedsetting.ValueTypeNumber, false},
		{"too large", "92233720369", yesno.Yes, sharedsetting.ValueTypeNumber, false},
		{"unavailable", "10", yesno.Yes, sharedsetting.ValueTypeNumber, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &createRepositoryStub{}
			service := createServiceFixture(repo)
			setting := rechargeSettingsStub{row: sharedsetting.Record{Key: sharedsetting.PaymentRechargeMinimumAmountKey, Value: tc.value, ValueType: tc.typeID, Status: tc.enabled}}
			if tc.missing {
				setting.err = errors.New("settings offline")
			}
			service.SetSettings(setting)
			_, err := service.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, customRequest(t, "25.39"))
			if err == nil || repo.created != nil {
				t.Fatalf("invalid minimum accepted: created=%+v err=%v", repo.created, err)
			}
		})
	}
}

func TestCustomRechargeOptionsExposeLimitsAndBadgesWithoutRequiringPackages(t *testing.T) {
	service := createServiceFixture(&createRepositoryStub{})
	badge := "推荐"
	service.packages = createPackageStub{rows: []rechargepackage.CatalogItem{{ID: 8, Name: "套餐", AmountCents: 1000, Badge: &badge}}}
	opts, err := service.Options(context.Background())
	if err != nil || opts.Limits.MinimumAmount != "10.00" || opts.Limits.MaximumAmount != "92233720368.54" || opts.Limits.AmountScale != 2 || len(opts.Packages) != 1 || opts.Packages[0].Badge == nil || *opts.Packages[0].Badge != badge {
		t.Fatalf("options=%+v err=%v", opts, err)
	}
	service.packages = createPackageStub{}
	opts, err = service.Options(context.Background())
	if err != nil || !opts.Available || len(opts.Packages) != 0 {
		t.Fatalf("empty catalog blocked custom recharge: %+v %v", opts, err)
	}
}

func TestCustomRechargeFingerprintIncludesNormalizedAmount(t *testing.T) {
	parse := func(amount string) CreateInput {
		t.Helper()
		var req createRequest
		if err := json.Unmarshal([]byte(`{"requestId":"00000000-0000-4000-8000-000000000001","amount":"`+amount+`","payMethod":1,"returnUrl":"https://app.example.test/paid"}`), &req); err != nil {
			t.Fatal(err)
		}
		in, err := req.input()
		if err != nil {
			t.Fatal(err)
		}
		return in
	}
	first := fingerprintText(requestFingerprint(parse("10.5")))
	if first != fingerprintText(requestFingerprint(parse("10.50"))) {
		t.Fatal("equivalent amounts did not normalize")
	}
	if first == fingerprintText(requestFingerprint(parse("10.51"))) {
		t.Fatal("changed custom amount reused fingerprint")
	}
}
