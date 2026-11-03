package run

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestQueryWindowAndOwnerValidation(t *testing.T) {
	now := time.Now().UTC()
	q := RunQuery{}
	if err := q.normalize(now, false); err != nil {
		t.Fatal(err)
	}
	if q.Limit != 20 || q.EndAt.Sub(q.StartAt) != 24*time.Hour {
		t.Fatalf("%+v", q)
	}
	q.StartAt = now.Add(-32 * 24 * time.Hour)
	if q.normalize(now, false) == nil {
		t.Fatal("unbounded history accepted")
	}
	q = RunQuery{StartAt: now.Add(-25 * time.Hour), EndAt: now}
	if q.normalize(now, true) == nil {
		t.Fatal("summary over 24h accepted")
	}
	if _, e := NewQueryService(nil).Detail(context.Background(), 1, QueryOwner{UserID: 1}, false); e == nil {
		t.Fatal("invalid owner accepted")
	}
}
func TestQueryProjectionNeverExportsSecrets(t *testing.T) {
	m := Model{ID: 1, ConfigSnapshot: json.RawMessage(`{"token":"secret","prompt":"private"}`)}
	b, _ := json.Marshal(AdminResponse(m))
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "private") {
		t.Fatal(string(b))
	}
}
