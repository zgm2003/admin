package run

import (
	"encoding/json"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestRecoveryPreservesLiveEffectsAndUnknownReservation(t *testing.T) {
	r, db, ctx := runDB(t)
	now := time.Now().UTC()
	row, _, err := r.Create(ctx, fixtureInput(), fixtureSnapshot(), completeClaimPolicy(t), now)
	if err != nil {
		t.Fatal(err)
	}
	token := uuid.NewString()
	if _, err = r.Claim(ctx, row.ID, token, now); err != nil {
		t.Fatal(err)
	}
	request, err := r.StartRequest(ctx, row.ID, token, 1, json.RawMessage(`{"model":"fixture"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Exec(`UPDATE ai_provider_request SET updated_at=? WHERE id=?`, now.Add(-3*time.Minute), request.ID).Error; err != nil {
		t.Fatal(err)
	}
	if n, e := r.RecoverStaleEffects(ctx, now, 100); e != nil || n != 0 {
		t.Fatalf("live owner recovered n=%d err=%v", n, e)
	}
	if err = db.WithContext(ctx).Exec(`UPDATE ai_run SET lease_until=? WHERE id=?`, now.Add(-time.Second), row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if n, e := r.RecoverExpired(ctx, now, 100); e != nil || n != 1 {
		t.Fatalf("expired recovery n=%d err=%v", n, e)
	}
	got, err := r.Find(ctx, row.ID)
	if err != nil || got.ExecutionStatus != 4 || got.UsageStatus != 2 || got.ReservationStatus != 1 {
		t.Fatalf("unknown run=%+v err=%v", got, err)
	}
	var n int64
	if err = db.WithContext(ctx).Table("ai_usage_charge").Where("run_id=? AND status=4 AND actual_units IS NULL", row.ID).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("missing reconciliation charge n=%d err=%v", n, err)
	}
	if n, e := r.RecoverExpired(ctx, now, 100); e != nil || n != 0 {
		t.Fatalf("replay n=%d err=%v", n, e)
	}
}

func TestRecoverQueuedCancellationCreatesZeroCharge(t *testing.T) {
	r, db, ctx := runDB(t)
	row, _, err := r.Create(ctx, fixtureInput(), fixtureSnapshot(), completeClaimPolicy(t), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Cancel(ctx, row.ID, 1, 1, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = r.RecoverQueued(ctx, time.Now().UTC().Add(time.Second), 100); err != nil {
		t.Fatal(err)
	}
	got, err := r.Find(ctx, row.ID)
	if err != nil || got.ExecutionStatus != 5 {
		t.Fatalf("run=%+v err=%v", got, err)
	}
	var n int64
	if err = db.WithContext(ctx).Table("ai_usage_charge").Where("run_id=? AND usage_complete=1 AND actual_units=0", row.ID).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("missing zero charge n=%d err=%v", n, err)
	}
}
