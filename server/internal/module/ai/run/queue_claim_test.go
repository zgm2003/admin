package run

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	pr "admin/server/internal/module/ai/providerRequest"
	"github.com/google/uuid"
)

func TestClaimFinalizesExpiredCanceledAndInvalidPolicyWithoutConsumption(t *testing.T) {
	for _, tc := range []struct {
		name    string
		policy  json.RawMessage
		cancel  bool
		expired bool
		code    string
		status  int16
	}{
		{name: "queue deadline", expired: true, code: "QUEUE_TIMEOUT", status: 4},
		{name: "canceled", cancel: true, code: "CANCELED", status: 5},
		{name: "missing policy", policy: json.RawMessage(`{}`), code: "INVALID_POLICY", status: 4},
		{name: "partial policy", policy: json.RawMessage(`{"userInflightLimit":2}`), code: "INVALID_POLICY", status: 4},
		{name: "wrong policy type", policy: json.RawMessage(`{"maxQueuedSeconds":"60"}`), code: "INVALID_POLICY", status: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, db, ctx := runDB(t)
			now := time.Now().UTC().Truncate(time.Microsecond)
			snap := fixtureSnapshot()
			snap.AcceptedAt = now.Add(-time.Minute)
			if !tc.expired {
				snap.AcceptedAt = now
			}
			in := fixtureInput()
			in.DeadlineAt = now.Add(time.Hour)
			row, _, err := repo.Create(ctx, in, snap, completeClaimPolicy(t), snap.AcceptedAt)
			if err != nil {
				t.Fatal(err)
			}
			if tc.policy != nil {
				// Simulate an older malformed frozen Run by inserting a distinct
				// fixture. The snapshot immutability trigger stays enabled.
				row.ID = 0
				row.RequestKey = uuid.NewString()
				row.PolicySnapshot = tc.policy
				if err = db.WithContext(ctx).Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.cancel {
				if err = db.WithContext(ctx).Exec(`UPDATE ai_run SET cancel_requested_at=? WHERE id=?`, now, row.ID).Error; err != nil {
					t.Fatal(err)
				}
			}
			token := uuid.NewString()
			claimed, err := repo.Claim(ctx, row.ID, token, now)
			if err != nil || claimed.ExecutionStatus != tc.status || claimed.ErrorCode == nil || *claimed.ErrorCode != tc.code || claimed.StartedAt != nil {
				t.Fatalf("claim=%+v err=%v", claimed, err)
			}
			got, err := repo.Find(ctx, row.ID)
			if err != nil || got.ExecutionStatus != tc.status || got.UsageStatus != 1 {
				t.Fatalf("terminal did not commit: %+v %v", got, err)
			}
			if _, err = repo.StartRequest(ctx, row.ID, token, 1, json.RawMessage(`{}`), now); !errors.Is(err, ErrRunLeaseLost) {
				t.Fatalf("terminal request started: %v", err)
			}
			if _, err = repo.Claim(ctx, row.ID, uuid.NewString(), now.Add(time.Second)); !errors.Is(err, ErrRunLeaseLost) {
				t.Fatalf("terminal reclaimed: %v", err)
			}
			var charges, requests, started, balance int64
			if err = db.WithContext(ctx).Table("ai_usage_charge").Where("run_id=? AND usage_complete=1 AND actual_units=0", row.ID).Count(&charges).Error; err != nil {
				t.Fatal(err)
			}
			if err = db.WithContext(ctx).Table("ai_provider_request").Where("run_id=?", row.ID).Count(&requests).Error; err != nil {
				t.Fatal(err)
			}
			if err = db.WithContext(ctx).Table("ai_run_event").Where("run_id=? AND event_type='started'", row.ID).Count(&started).Error; err != nil {
				t.Fatal(err)
			}
			if err = db.WithContext(ctx).Raw(`SELECT balance_units FROM payment_wallet WHERE user_id=1`).Scan(&balance).Error; err != nil {
				t.Fatal(err)
			}
			if charges != 1 || requests != 0 || started != 0 || balance != 100 {
				t.Fatalf("charges=%d requests=%d started=%d balance=%d", charges, requests, started, balance)
			}
		})
	}
}

type queuedClaimCredentials struct{ calls int }

func (c *queuedClaimCredentials) Load(context.Context, RuntimeSnapshot) (pr.FrozenConfig, error) {
	c.calls++
	return pr.FrozenConfig{}, errors.New("unexpected credential load after terminal claim")
}

func TestTextEngineStopsWhenClaimClosesExpiredQueue(t *testing.T) {
	repo, db, ctx := runDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	snap := fixtureSnapshot()
	snap.AcceptedAt = now.Add(-time.Minute)
	in := fixtureInput()
	in.DeadlineAt = now.Add(time.Hour)
	row, _, err := repo.Create(ctx, in, snap, completeClaimPolicy(t), snap.AcceptedAt)
	if err != nil {
		t.Fatal(err)
	}
	credentials := &queuedClaimCredentials{}
	engine := NewTextEngine(repo, credentials, nil, nil, nil, nil)
	engine.now = func() time.Time { return now }
	if err = engine.Execute(ctx, row); err != nil {
		t.Fatal(err)
	}
	if credentials.calls != 0 {
		t.Fatalf("generation entered after terminal claim: credentials=%d", credentials.calls)
	}
	var requests int64
	if err = db.WithContext(ctx).Table("ai_provider_request").Where("run_id=?", row.ID).Count(&requests).Error; err != nil || requests != 0 {
		t.Fatalf("requests=%d err=%v", requests, err)
	}
	got, err := repo.Find(ctx, row.ID)
	if err != nil || got.ExecutionStatus != 4 || got.ErrorCode == nil || *got.ErrorCode != "QUEUE_TIMEOUT" {
		t.Fatalf("run=%+v err=%v", got, err)
	}
}
