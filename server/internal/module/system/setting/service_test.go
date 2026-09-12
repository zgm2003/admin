package setting

import (
	"context"
	"errors"
	"testing"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type fakeRepository struct {
	rows     map[string]Record
	list     []Record
	create   Record
	update   Record
	findErr  error
	writeErr error
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

var _ sharedsetting.Reader = (*Service)(nil)
