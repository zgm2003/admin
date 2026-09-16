package setting

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type fakeRepository struct {
	rows     map[string]Record
	list     []Record
	create   Record
	update   Record
	findErr  error
	writeErr error
	brand    BrandSettings
}

func (f *fakeRepository) List(context.Context, ListQuery) ([]Record, int64, error) {
	return f.list, int64(len(f.list)), nil
}
func (f *fakeRepository) Find(_ context.Context, key string) (Record, error) {
	if f.findErr != nil {
		return Record{}, f.findErr
	}
	row, ok := f.rows[key]
	if !ok {
		return Record{}, ErrNotFound
	}
	return row, nil
}
func (f *fakeRepository) Create(_ context.Context, row *Record) error {
	f.create = *row
	if row.ID == 0 {
		row.ID = 1
	}
	if f.rows == nil {
		f.rows = map[string]Record{}
	}
	f.rows[row.Key] = *row
	return f.writeErr
}
func (f *fakeRepository) Update(_ context.Context, key string, row Record) error {
	f.update = row
	return f.writeErr
}
func (f *fakeRepository) UpdateStatus(_ context.Context, key string, status yesno.Value, now time.Time) error {
	return f.writeErr
}
func (f *fakeRepository) Delete(_ context.Context, key string) error { return f.writeErr }
func (f *fakeRepository) FindBrand(_ context.Context) (BrandSettings, error) {
	if f.findErr != nil {
		return BrandSettings{}, f.findErr
	}
	return BrandSettings{
		TitleZhCN: f.rows[BrandTitleZhCNKey].Value, TitleEnUS: f.rows[BrandTitleEnUSKey].Value, DefaultAvatar: f.rows[BrandDefaultAvatarKey].Value,
	}, nil
}
func (f *fakeRepository) UpdateBrand(_ context.Context, brand BrandSettings, _ time.Time) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.brand = brand
	return nil
}

func TestServiceCreateRejectsUnknownValueTypeAndMalformedJSON(t *testing.T) {
	service := NewService(&fakeRepository{})
	if _, err := service.Create(context.Background(), CreateInput{Key: "auth.captcha.ttl_minutes", Value: "2", ValueType: 99}); err == nil {
		t.Fatal("expected unknown value type to fail")
	}
	if _, err := service.Create(context.Background(), CreateInput{Key: "auth.captcha.policy", Value: "{", ValueType: ValueTypeJSON}); err == nil {
		t.Fatal("expected malformed json to fail")
	}
}

func TestServiceCreateNormalizesAndReturnsSharedRecord(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)
	id, err := service.Create(context.Background(), CreateInput{Key: " auth.captcha.ttl_minutes ", Value: " 2 ", ValueType: ValueTypeNumber, Description: " ttl "})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id != 1 || repo.create.Key != "auth.captcha.ttl_minutes" || repo.create.Value != "2" || repo.create.Description != "ttl" {
		t.Fatalf("unexpected record: %#v", repo.create)
	}
	shared, err := service.FindByKey(context.Background(), "auth.captcha.ttl_minutes")
	if err != nil {
		t.Fatalf("FindByKey() error = %v", err)
	}
	if shared.Key != repo.create.Key {
		t.Fatalf("unexpected shared record: %#v", shared)
	}
}

func TestServiceBuiltinCannotDelete(t *testing.T) {
	repo := &fakeRepository{rows: map[string]Record{"auth.captcha.ttl_minutes": {Key: "auth.captcha.ttl_minutes", IsBuiltin: yesno.Yes}}}
	service := NewService(repo)
	if err := service.Delete(context.Background(), "auth.captcha.ttl_minutes"); err == nil {
		t.Fatal("expected builtin delete to fail")
	}
}

func TestServicePropagatesRepositoryFailure(t *testing.T) {
	repoErr := errors.New("database down")
	service := NewService(&fakeRepository{findErr: repoErr})
	if _, err := service.FindByKey(context.Background(), "auth.captcha.ttl_minutes"); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestServiceReadsAndAtomicallyUpdatesBrandSettings(t *testing.T) {
	repo := &fakeRepository{rows: map[string]Record{
		BrandTitleZhCNKey:     {Key: BrandTitleZhCNKey, Value: "智澜", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
		BrandTitleEnUSKey:     {Key: BrandTitleEnUSKey, Value: "ZHILAN", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
		BrandDefaultAvatarKey: {Key: BrandDefaultAvatarKey, Value: "", ValueType: ValueTypeString, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes},
	}}
	service := NewService(repo)
	brand, err := service.Brand(context.Background())
	if err != nil || brand.TitleZhCN != "智澜" || brand.TitleEnUS != "ZHILAN" || brand.DefaultAvatar != "" {
		t.Fatalf("brand=%+v error=%v", brand, err)
	}

	err = service.UpdateBrand(context.Background(), BrandSettings{
		TitleZhCN: " 新标题 ", TitleEnUS: " New title ", DefaultAvatar: "avatar/2026/09/15/default.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := BrandSettings{TitleZhCN: "新标题", TitleEnUS: "New title", DefaultAvatar: "avatar/2026/09/15/default.png"}
	if repo.brand != want {
		t.Fatalf("updated brand=%+v want=%+v", repo.brand, want)
	}
}

func TestServiceRejectsInvalidBrandSettings(t *testing.T) {
	service := NewService(&fakeRepository{})
	for _, input := range []BrandSettings{
		{TitleZhCN: "", TitleEnUS: "ZHILAN"},
		{TitleZhCN: "智澜", TitleEnUS: ""},
		{TitleZhCN: "智澜", TitleEnUS: "ZHILAN", DefaultAvatar: "other/default.png"},
		{TitleZhCN: "智澜", TitleEnUS: "ZHILAN", DefaultAvatar: "avatar/../secret.png"},
	} {
		if err := service.UpdateBrand(context.Background(), input); err == nil {
			t.Fatalf("input=%+v should fail", input)
		}
	}
}

func TestRepositoryUpdateBrandRollsBackWhenOneSettingIsMissing(t *testing.T) {
	db, ctx := openSettingDatabase(t)
	now := time.Now().UTC()
	if err := db.WithContext(ctx).Create([]Model{
		{Key: BrandTitleZhCNKey, Value: "旧中文", ValueType: ValueTypeString, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now},
		{Key: BrandTitleEnUSKey, Value: "OLD", ValueType: ValueTypeString, Description: "", IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes, CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}

	err := NewRepository(db).UpdateBrand(ctx, BrandSettings{
		TitleZhCN: "新中文", TitleEnUS: "NEW", DefaultAvatar: "avatar/default.png",
	}, now.Add(time.Minute))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateBrand() error=%v want=%v", err, ErrNotFound)
	}

	var rows []Model
	if err = db.WithContext(ctx).Order("setting_key ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Value != "OLD" || rows[1].Value != "旧中文" {
		t.Fatalf("rows=%+v, transaction partially updated brand settings", rows)
	}
}

func openSettingDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	db, ctx := testschema.Open(t, settings.PostgresDSN, "test_system_setting_repository")
	if err = db.WithContext(ctx).Exec(`
		CREATE TABLE system_setting (
			id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
			setting_key VARCHAR(128) NOT NULL,
			value TEXT NOT NULL,
			value_type SMALLINT NOT NULL,
			description VARCHAR(512) NOT NULL,
			is_enabled SMALLINT NOT NULL,
			is_builtin SMALLINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			deleted_at TIMESTAMPTZ
		);
		CREATE UNIQUE INDEX ux_system_setting_key_active ON system_setting(setting_key) WHERE deleted_at IS NULL;
	`).Error; err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

var _ sharedsetting.Reader = (*Service)(nil)
