package run

import (
	"encoding/json"
	"testing"
	"time"
)

func TestQueryRepositoryOwnerIsolationAndBoundedChildren(t *testing.T) {
	r, db, ctx := runDB(t)
	in := fixtureInput()
	snap := fixtureSnapshot()
	m, _, e := r.Create(ctx, in, snap, json.RawMessage(`{"userInflightLimit":2}`), snap.AcceptedAt)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.WithContext(ctx).Exec(`CREATE TABLE payment_transaction(id bigint PRIMARY KEY,ai_usage_charge_id bigint)`).Error; e != nil {
		t.Fatal(e)
	}
	q := RunQuery{}
	if e = q.normalize(time.Now().UTC(), false); e != nil {
		t.Fatal(e)
	}
	service := NewQueryService(NewQueryRepository(db))
	if _, e = service.Detail(ctx, m.ID, QueryOwner{2, 1}, false); e == nil {
		t.Fatal("foreign owner detail visible")
	}
	if _, e = service.Detail(ctx, m.ID, QueryOwner{1, 2}, false); e == nil {
		t.Fatal("foreign platform detail visible")
	}
	found, e := service.Detail(ctx, m.ID, QueryOwner{2, 2}, true)
	if e != nil || found.UserID != 1 {
		t.Fatalf("management=%+v err=%v", found, e)
	}
	rows, _, e := service.List(ctx, q, QueryOwner{2, 1}, false)
	if e != nil || len(rows) != 0 {
		t.Fatalf("foreign owner list=%+v err=%v", rows, e)
	}
	rows, _, e = service.List(ctx, q, QueryOwner{2, 2}, true)
	if e != nil || len(rows) != 1 {
		t.Fatalf("admin list=%+v err=%v", rows, e)
	}
	events, e := service.Events(ctx, m.ID, 0, 1, QueryOwner{1, 1})
	if e != nil || len(events.List) != 1 || events.NextSequence != 1 {
		t.Fatalf("events=%+v err=%v", events, e)
	}
	events, e = service.Events(ctx, m.ID, 42, 1, QueryOwner{1, 1})
	if e != nil || events.NextSequence != 42 || len(events.List) != 0 {
		t.Fatalf("empty events=%+v err=%v", events, e)
	}
	if e = db.WithContext(ctx).Exec(`DELETE FROM ai_run_event WHERE run_id=?`, m.ID).Error; e != nil {
		t.Fatal(e)
	}
	events, e = service.Events(ctx, m.ID, 0, 1, QueryOwner{1, 1})
	if e != nil || events.ResetReason != "EVENTS_EXPIRED" || events.NextSequence != m.NextEventSequence || len(events.List) != 0 {
		t.Fatalf("expired events=%+v err=%v", events, e)
	}
	for _, kind := range []string{"request", "tool-call", "usage-charge", "usage-charge-item", "media"} {
		if _, e = service.Children(ctx, m.ID, 0, 20, kind, QueryOwner{1, 1}, false); e != nil {
			t.Fatalf("%s: %v", kind, e)
		}
	}
	summary, e := service.repository.Summary(ctx, q)
	if e != nil || summary.RunCount != 1 || summary.EstimatedUnits != "80" || summary.SettledUnits != "0" {
		t.Fatalf("summary=%+v err=%v", summary, e)
	}
	input, result, e := service.Content(ctx, found)
	if e != nil || input.Text != "hello" || string(result) != "null" {
		t.Fatalf("input=%+v result=%s err=%v", input, result, e)
	}
}
