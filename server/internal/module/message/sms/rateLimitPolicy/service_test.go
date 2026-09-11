package rateLimitPolicy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"admin/server/internal/shared/apperror"
	"gorm.io/gorm"
)

type fakeRepository struct {
	catalogs     []Catalog
	findErr      error
	updateErr    error
	updatedLimit int
	updatedWin   int
	updatedKey   string
	provisioned  int64
	deleted      int64
}

func (f *fakeRepository) ListPlatforms(context.Context) ([]Catalog, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.catalogs, nil
}

func (f *fakeRepository) FindPlatform(_ context.Context, platformID int64) (Catalog, error) {
	if f.findErr != nil {
		return Catalog{}, f.findErr
	}
	for _, catalog := range f.catalogs {
		if catalog.PlatformID == platformID {
			return catalog, nil
		}
	}
	return Catalog{}, ErrPlatformNotFound
}

func (f *fakeRepository) UpdatePolicy(_ context.Context, platformID int64, key string, limit, windowSeconds int, now time.Time) (Model, error) {
	if f.updateErr != nil {
		return Model{}, f.updateErr
	}
	f.updatedKey, f.updatedLimit, f.updatedWin = key, limit, windowSeconds
	return Model{
		PlatformID: platformID, Key: key, Mode: ModeBusiness, Dimension: DimensionPhone,
		Limit: limit, WindowSeconds: windowSeconds, Revision: 2, UpdatedAt: now,
	}, nil
}

func (f *fakeRepository) ProvisionDefaults(_ context.Context, platformID int64, now time.Time) error {
	f.provisioned = platformID
	return f.updateErr
}

func (f *fakeRepository) DeleteForPlatform(_ context.Context, platformID int64) error {
	f.deleted = platformID
	return f.updateErr
}

type fakeStore struct {
	calls     int
	loadErr   error
	mutateErr error
}

func (f *fakeStore) Load(ctx context.Context, platformID int64, load func(context.Context) (Catalog, error)) (Catalog, error) {
	f.calls++
	if f.loadErr != nil {
		return Catalog{}, f.loadErr
	}
	return load(ctx)
}

func (f *fakeStore) Mutate(ctx context.Context, platformID int64, change func(context.Context) error) error {
	f.calls++
	if f.mutateErr != nil {
		return f.mutateErr
	}
	return change(ctx)
}

func appErrorCode(err error) int {
	var appError *apperror.Error
	if errors.As(err, &appError) {
		return appError.Code
	}
	return 0
}

func catalogOf(platformID int64, code string, policies ...Model) Catalog {
	if len(policies) == 0 {
		for index, fixed := range FixedPolicies() {
			policies = append(policies, Model{
				PlatformID: platformID, Key: fixed.Key, Mode: fixed.Mode, Dimension: fixed.Dimension,
				Limit: fixed.Limit, WindowSeconds: fixed.WindowSeconds, Revision: int64(index + 1),
			})
		}
	}
	return Catalog{PlatformID: platformID, PlatformCode: code, PlatformName: strings.ToUpper(code), Policies: policies}
}

func TestListReturnsEveryActivePlatformWithItsPolicies(t *testing.T) {
	repository := &fakeRepository{catalogs: []Catalog{catalogOf(1, "admin"), catalogOf(2, "canvas")}}
	platforms, err := NewService(repository, &fakeStore{}).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(platforms) != 2 || platforms[0].PlatformCode != "admin" || len(platforms[0].Policies) != 2 {
		t.Fatalf("platforms = %+v", platforms)
	}
	if platforms[0].Policies[0].Key != KeyTenMin && platforms[0].Policies[0].Key != KeyMinute {
		t.Fatalf("policy key = %q", platforms[0].Policies[0].Key)
	}
}

func TestUpdateRejectsUnknownKeysAndOutOfRangeValues(t *testing.T) {
	for _, test := range []struct {
		name          string
		key           string
		limit, window int
	}{
		{name: "unknown key", key: "business_phone_ip", limit: 1, window: 60},
		{name: "zero limit", key: KeyMinute, limit: 0, window: 60},
		{name: "limit above maximum", key: KeyMinute, limit: 100001, window: 60},
		{name: "zero window", key: KeyMinute, limit: 1, window: 0},
		{name: "window above one day", key: KeyMinute, limit: 1, window: 86401},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{}
			_, err := NewService(&fakeRepository{}, store).Update(context.Background(), 1, test.key, test.limit, test.window)
			if appErrorCode(err) != apperror.CodeInvalidRequest {
				t.Fatalf("Update() error = %v, want invalid request", err)
			}
			if store.calls != 0 {
				t.Fatal("invalid input reached the store")
			}
		})
	}
}

func TestUpdateRequiresAnActivePlatform(t *testing.T) {
	_, err := NewService(&fakeRepository{}, &fakeStore{}).Update(context.Background(), 9, KeyMinute, 2, 120)
	if appErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("Update() error = %v, want not found", err)
	}
}

func TestUpdateWritesThroughTheTokenizedMutationAndReturnsRevision(t *testing.T) {
	repository := &fakeRepository{catalogs: []Catalog{catalogOf(1, "admin")}}
	store := &fakeStore{}
	platform, err := NewService(repository, store).Update(context.Background(), 1, KeyMinute, 2, 120)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if store.calls != 1 || repository.updatedKey != KeyMinute || repository.updatedLimit != 2 || repository.updatedWin != 120 {
		t.Fatalf("store calls = %d updated = %s/%d/%d", store.calls, repository.updatedKey, repository.updatedLimit, repository.updatedWin)
	}
	if platform.PlatformID != 1 || len(platform.Policies) != 2 {
		t.Fatalf("platform = %+v", platform)
	}
}

func TestUpdateMapsStoreAndRepositoryFailures(t *testing.T) {
	repository := &fakeRepository{catalogs: []Catalog{catalogOf(1, "admin")}}
	if _, err := NewService(repository, &fakeStore{mutateErr: errors.New("redis unavailable")}).
		Update(context.Background(), 1, KeyMinute, 2, 120); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("store failure error = %v", err)
	}

	repository.updateErr = errors.New("database unavailable")
	if _, err := NewService(repository, &fakeStore{}).
		Update(context.Background(), 1, KeyMinute, 2, 120); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("repository failure error = %v", err)
	}

	repository.updateErr = gorm.ErrRecordNotFound
	if _, err := NewService(repository, &fakeStore{}).
		Update(context.Background(), 1, KeyMinute, 2, 120); appErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("missing policy error = %v", err)
	}
}

func TestCatalogFailsClosedOnIncompletePolicies(t *testing.T) {
	incomplete := Catalog{PlatformID: 1, PlatformCode: "admin", Policies: []Model{{Key: KeyMinute}}}
	repository := &fakeRepository{catalogs: []Catalog{incomplete}}
	if _, err := NewService(repository, &fakeStore{}).Catalog(context.Background(), 1); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Catalog() error = %v, want dependency unavailable", err)
	}

	if _, err := NewService(&fakeRepository{catalogs: []Catalog{catalogOf(1, "admin")}}, nil).
		Catalog(context.Background(), 1); appErrorCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("Catalog() without a store error = %v", err)
	}
}

func TestLifecycleHelpersForwardToTheRepository(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, &fakeStore{})
	if err := service.ProvisionDefaults(context.Background(), 5); err != nil {
		t.Fatalf("ProvisionDefaults() error = %v", err)
	}
	if err := service.DeleteForPlatform(context.Background(), 6); err != nil {
		t.Fatalf("DeleteForPlatform() error = %v", err)
	}
	if repository.provisioned != 5 || repository.deleted != 6 {
		t.Fatalf("provisioned=%d deleted=%d", repository.provisioned, repository.deleted)
	}
}
