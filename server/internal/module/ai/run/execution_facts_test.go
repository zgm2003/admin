package run

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"encoding/json"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestExecutionFactsSendingCannotBeReplayedAndTerminalChargeIsAtomic(t *testing.T) {
	repo, db, ctx := runDB(t)
	// The immutable source rates are intentionally simple so fractional units
	// from separate requests must be rounded only at Run completion.
	snap := fixtureSnapshot()
	snap.Config = json.RawMessage(`{"rates":[{"id":1,"operation":"text.generate","usageKey":"input","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":1},{"id":2,"operation":"text.generate","usageKey":"output","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":1}]}`)
	row, _, e := repo.Create(ctx, fixtureInput(), snap, completeClaimPolicy(t), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	token := uuid.NewString()
	if _, e = repo.Claim(ctx, row.ID, token, time.Now()); e != nil {
		t.Fatal(e)
	}
	req, e := repo.StartRequest(ctx, row.ID, token, 1, []byte(`{"model":"fixture"}`), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = repo.StartRequest(ctx, row.ID, token, 1, []byte(`{"model":"fixture"}`), time.Now()); e == nil {
		t.Fatal("sending intent was replayed")
	}
	response := pr.TextResponse{ID: "fixture", Status: "completed", OutputItems: []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","content":[]}`)}, Usage: pr.TokenUsage{Known: true, InputTokens: 1, OutputTokens: 1}}
	if e = repo.FinishRequest(ctx, row.ID, token, req.ID, response, nil, time.Now()); e != nil {
		t.Fatal(e)
	}
	if e = repo.FinishText(ctx, row.ID, token, 3, "", "ok", time.Now()); e != nil {
		t.Fatal(e)
	}
	var count, total int64
	if e = db.WithContext(ctx).Raw(`SELECT count(*),coalesce(sum(actual_units),0) FROM ai_usage_charge WHERE run_id=? AND status=1 AND usage_complete=1`, row.ID).Row().Scan(&count, &total); e != nil {
		t.Fatal(e)
	}
	if count != 1 || total != 1 {
		t.Fatalf("charge count=%d total=%d", count, total)
	}
	if e = repo.FinishText(ctx, row.ID, token, 3, "", "ok", time.Now()); e != nil {
		t.Fatal(e)
	}
	if e = db.WithContext(ctx).Raw(`SELECT count(*) FROM system_scheduler_job WHERE task_type='ai.usage.settle'`).Scan(&count).Error; e != nil || count != 1 {
		t.Fatalf("settle jobs=%d err=%v", count, e)
	}
}
