package order

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment"
	"admin/server/internal/shared/apperror"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

const expirySettingKey = "payment.order.expiry_minutes"

type orderSettingsFunc func(context.Context, string) (sharedsetting.Record, error)

func (f orderSettingsFunc) FindByKey(ctx context.Context, key string) (sharedsetting.Record, error) {
	return f(ctx, key)
}

func expiryInput() CreateInput {
	return CreateInput{RequestID: "00000000-0000-4000-8000-000000000001", PackageID: 8,
		PayMethod: payment.PayMethodWeb, ReturnURL: "https://app.example.test/paid"}
}

func TestCreateFreezesConfiguredExpiryForEachRechargeSource(t *testing.T) {
	for _, minutes := range []int{1, 15, 30, 1440} {
		for _, custom := range []bool{false, true} {
			t.Run(strconv.Itoa(minutes)+"/custom="+strconv.FormatBool(custom), func(t *testing.T) {
				repo := &createRepositoryStub{}
				s := createServiceFixture(repo)
				base := s.settings
				type contextKey struct{}
				ctx := context.WithValue(context.Background(), contextKey{}, "request")
				s.SetSettings(orderSettingsFunc(func(got context.Context, key string) (sharedsetting.Record, error) {
					if got.Value(contextKey{}) != "request" {
						t.Fatal("request context was lost")
					}
					if key != expirySettingKey {
						return base.FindByKey(got, key)
					}
					return sharedsetting.Record{Key: key, Value: strconv.Itoa(minutes), ValueType: 2, Status: yesno.Yes}, nil
				}))
				input := expiryInput()
				if custom {
					input.PackageID, input.AmountCents = 0, 1000
				}
				got, err := s.Create(ctx, auth.Identity{UserID: 7, PlatformID: 1}, input)
				want := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
				if err != nil || repo.created == nil || !got.Order.ExpiresAt.Equal(want) || !repo.created.ExpiresAt.Equal(want) {
					t.Fatalf("created=%+v checkout=%+v err=%v want expiry=%v", repo.created, got, err, want)
				}
				if repo.created.NextCheckAt == nil || repo.created.NextCheckAt.After(want) {
					t.Fatal("first query is after expiry")
				}
			})
		}
	}
}

func TestCreateExpiryFailsClosedBeforePersisting(t *testing.T) {
	for _, scenario := range []string{"missing reader", "unavailable", "missing", "wrong key", "disabled", "wrong type", "empty", "zero", "negative", "too large", "overflow", "decimal", "padded", "plus"} {
		t.Run(scenario, func(t *testing.T) {
			repo := &createRepositoryStub{}
			s := createServiceFixture(repo)
			s.SetSettings(orderSettingsFunc(func(context.Context, string) (sharedsetting.Record, error) {
				row := sharedsetting.Record{Key: expirySettingKey, Value: "30", ValueType: 2, Status: yesno.Yes}
				switch scenario {
				case "unavailable":
					return row, errors.New("settings unavailable")
				case "missing":
					return sharedsetting.Record{}, nil
				case "wrong key":
					row.Key = "custom.key"
				case "disabled":
					row.Status = yesno.No
				case "wrong type":
					row.ValueType = 1
				default:
					if value, ok := map[string]string{"empty": "", "zero": "0", "negative": "-1", "too large": "1441", "overflow": "9999999999999999999", "decimal": "1.5", "padded": " 30 ", "plus": "+30"}[scenario]; ok {
						row.Value = value
					}
				}
				return row, nil
			}))
			if scenario == "missing reader" {
				s.SetSettings(nil)
			}
			_, err := s.Create(context.Background(), auth.Identity{UserID: 7, PlatformID: 1}, expiryInput())
			var public *apperror.Error
			if !errors.As(err, &public) || public.Code != apperror.CodeDependencyUnavailable || repo.created != nil {
				t.Fatalf("invalid expiry persisted: created=%+v err=%v", repo.created, err)
			}
		})
	}
}

func TestCreateExpiryChangesOnlyNewOrdersAndReplayNeedsNoSettings(t *testing.T) {
	repo := &createRepositoryStub{}
	s := createServiceFixture(repo)
	minutes := "15"
	reads := 0
	s.SetSettings(orderSettingsFunc(func(context.Context, string) (sharedsetting.Record, error) {
		reads++
		return sharedsetting.Record{Key: expirySettingKey, Value: minutes, ValueType: 2, Status: yesno.Yes}, nil
	}))
	actor := auth.Identity{UserID: 7, PlatformID: 1}
	first, err := s.Create(context.Background(), actor, expiryInput())
	if err != nil {
		t.Fatal(err)
	}
	old := *repo.created
	minutes = "60"
	secondInput := expiryInput()
	secondInput.RequestID = "00000000-0000-4000-8000-000000000002"
	if _, err := s.Create(context.Background(), actor, secondInput); err != nil {
		t.Fatal(err)
	}
	if !repo.created.ExpiresAt.Equal(old.CreatedAt.Add(time.Hour)) {
		t.Fatal("new setting was not applied to new order")
	}
	repo.existing, repo.created = &old, nil
	s.SetSettings(orderSettingsFunc(func(context.Context, string) (sharedsetting.Record, error) {
		t.Fatal("replay reread settings")
		return sharedsetting.Record{}, nil
	}))
	replayed, err := s.Create(context.Background(), actor, expiryInput())
	if err != nil || !replayed.Order.ExpiresAt.Equal(first.Order.ExpiresAt) || repo.created != nil || reads != 2 {
		t.Fatalf("expiry was not frozen: replay=%+v err=%v reads=%d", replayed, err, reads)
	}
}
