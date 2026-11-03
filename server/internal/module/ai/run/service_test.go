package run

import (
	"admin/server/internal/shared/apperror"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type replayRunRepository struct {
	row     Model
	found   bool
	reads   int
	creates int
}

func (r *replayRunRepository) FindByRequest(context.Context, int64, int64, string) (Model, bool, error) {
	r.reads++
	return r.row, r.found, nil
}
func (r *replayRunRepository) Create(context.Context, CreateInput, RuntimeSnapshot, json.RawMessage, time.Time) (Model, bool, error) {
	r.creates++
	return Model{}, false, errors.New("unexpected create")
}

type unavailableRuntime struct{ calls int }

func (r *unavailableRuntime) Get(context.Context, int64, time.Time) (RuntimeSnapshot, error) {
	r.calls++
	return RuntimeSnapshot{}, errors.New("configuration unavailable")
}
func TestRunCreateReplaysBeforeCurrentConfigPolicyAndGate(t *testing.T) {
	in := fixtureInput()
	fp, err := fingerprint(in)
	if err != nil {
		t.Fatal(err)
	}
	repo := &replayRunRepository{found: true, row: Model{ID: 99, RequestFingerprint: fp, ProviderVersionID: 1}}
	runtime := &unavailableRuntime{}
	svc := NewService(repo, runtime)
	row, created, err := svc.Create(context.Background(), in)
	if err != nil || created || row.ID != 99 || runtime.calls != 0 || repo.creates != 0 {
		t.Fatalf("row=%+v created=%v error=%v config calls=%d creates=%d", row, created, err, runtime.calls, repo.creates)
	}
	in.Input = json.RawMessage(`{"text":"different"}`)
	if _, _, err := svc.Create(context.Background(), in); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict=%v", err)
	}
}

func TestRunCreateRejectsDisabledVideoBeforeAdmission(t *testing.T) {
	repo := &replayRunRepository{}
	svc := NewService(repo, &unavailableRuntime{})
	in := fixtureInput()
	in.Operation = "video.generate"
	_, _, err := svc.Create(context.Background(), in)
	var public *apperror.Error
	if !errors.As(err, &public) || public.HTTPStatus != 400 || public.Code != apperror.CodeInvalidRequest || public.Cause == nil || public.Cause.Error() != "video.generate is disabled" {
		t.Fatalf("error=%+v", err)
	}
	if repo.creates != 0 {
		t.Fatal("disabled video reached repository create")
	}
}
