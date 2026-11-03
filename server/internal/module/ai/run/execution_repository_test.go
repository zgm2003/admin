package run

import (
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestRunLeaseCannotBeReclaimedToResendAnUnknownRequest(t *testing.T) {
	repo, db, ctx := runDB(t)
	in := fixtureInput()
	snapshot := fixtureSnapshot()
	row, _, err := repo.Create(ctx, in, snapshot, completeClaimPolicy(t), snapshot.AcceptedAt)
	if err != nil {
		t.Fatal(err)
	}
	token := uuid.NewString()
	claimed, err := repo.Claim(ctx, row.ID, token, time.Now())
	if err != nil || claimed.ExecutionStatus != 1 {
		t.Fatalf("claim=%+v err=%v", claimed, err)
	}
	if _, err := repo.Claim(ctx, row.ID, uuid.NewString(), time.Now()); !errors.Is(err, ErrRunLeaseLost) {
		t.Fatalf("duplicate claim=%v", err)
	}
	if err := repo.Renew(ctx, row.ID, token, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE ai_run SET lease_until=now()-interval '1 second' WHERE id=?`, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.Renew(ctx, row.ID, token, time.Now()); !errors.Is(err, ErrRunLeaseLost) {
		t.Fatalf("expired renewal=%v", err)
	}
	if _, err := repo.Claim(ctx, row.ID, uuid.NewString(), time.Now()); !errors.Is(err, ErrRunLeaseLost) {
		t.Fatalf("expired run was resubmitted=%v", err)
	}
}
