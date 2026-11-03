package run

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type fixtureCredentials struct{ config pr.FrozenConfig }

func (f fixtureCredentials) Load(context.Context, RuntimeSnapshot) (pr.FrozenConfig, error) {
	return f.config, nil
}

func TestTextEngineUsesDurableIntentAndNeverResendsFinishedRun(t *testing.T) {
	repo, db, ctx := runDB(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var count int64
		if e := db.WithContext(ctx).Raw(`SELECT count(*) FROM ai_provider_request WHERE status=1`).Scan(&count).Error; e != nil || count != 1 {
			t.Errorf("POST preceded intent count=%d err=%v", count, e)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"response-fixture","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":3,"input_tokens_details":{"cached_tokens":0},"output_tokens":2,"total_tokens":5}}`)
	}))
	defer server.Close()
	snap := fixtureSnapshot()
	snap.Config = json.RawMessage(`{"upstreamModelId":"fixture","limits":{"maxInputTokens":1000,"maxOutputTokens":10},"parameters":{},"generationParams":{},"rates":[{"id":1,"operation":"text.generate","usageKey":"input","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":1},{"id":2,"operation":"text.generate","usageKey":"output","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":1}]}`)
	p := testGatePolicy(t)
	policy, _ := json.Marshal(p)
	snap.Policy = policy
	row, _, e := repo.Create(ctx, fixtureInput(), snap, policy, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	engine := NewTextEngine(repo, fixtureCredentials{pr.FrozenConfig{BaseURL: server.URL, APIKey: "fixture", Model: "fixture", Timeout: time.Second}}, nil, nil, pr.NewResponsesClient(server.Client()), pr.NewChatCompletionsClient(server.Client()))
	engine.SetStreamPressure(streamPressureFunc(func(context.Context) (int64, error) { return 0, nil }))
	if e = engine.Execute(ctx, row); e != nil {
		t.Fatal(e)
	}
	if e = engine.Execute(ctx, row); e != nil {
		t.Fatal(e)
	}
	out, e := repo.Find(ctx, row.ID)
	if e != nil || out.ExecutionStatus != 3 || out.UsageStatus != 1 || calls.Load() != 1 {
		t.Fatalf("out=%+v calls=%d err=%v", out, calls.Load(), e)
	}
}
