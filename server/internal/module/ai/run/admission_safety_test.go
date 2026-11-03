package run

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	pr "admin/server/internal/module/ai/providerRequest"
	"github.com/google/uuid"
)

func completeClaimPolicy(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(testGatePolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAdmissionBlocksUnknownProviderUsageBeforeRunFinalization(t *testing.T) {
	repo, db, ctx := runDB(t)
	if err := db.WithContext(ctx).Exec(`UPDATE payment_wallet SET balance_units=10000 WHERE user_id=1`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	policy := completeClaimPolicy(t)
	row, _, err := repo.Create(ctx, fixtureInput(), fixtureSnapshot(), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	token := uuid.NewString()
	if _, err = repo.Claim(ctx, row.ID, token, now); err != nil {
		t.Fatal(err)
	}
	request, err := repo.StartRequest(ctx, row.ID, token, 1, json.RawMessage(`{}`), now)
	if err != nil {
		t.Fatal(err)
	}
	// A normal sending request remains covered by its estimate; it is not an
	// unknown completed consumption fact.
	other := fixtureInput()
	other.PlatformID = 2
	other.EstimatedUnits = 1
	if _, _, err = repo.Create(ctx, other, fixtureSnapshot(), policy, now); err != nil {
		t.Fatalf("sending request incorrectly blocked admission: %v", err)
	}
	if err = db.WithContext(ctx).Exec(`UPDATE ai_run SET reservation_status=2 WHERE request_key=?`, other.RequestKey).Error; err != nil {
		t.Fatal(err)
	}
	if err = repo.FinishRequest(ctx, row.ID, token, request.ID, pr.TextResponse{Usage: pr.TokenUsage{Known: false}}, nil, now); err != nil {
		t.Fatal(err)
	}
	other.RequestKey = uuid.NewString()
	if _, _, err = repo.Create(ctx, other, fixtureSnapshot(), policy, now); !errors.Is(err, ErrUnknownInflight) {
		t.Fatalf("unknown provider usage admitted before FinishText: %v", err)
	}
	got, err := repo.Find(ctx, row.ID)
	if err != nil || got.ExecutionStatus != 1 || got.UsageStatus != 0 {
		t.Fatalf("test must precede Run finalization: %+v %v", got, err)
	}
}

func TestAdmissionBlocksUnknownToolBeforeRunFinalization(t *testing.T) {
	repo, db, ctx := runDB(t)
	if err := db.WithContext(ctx).Exec(`UPDATE payment_wallet SET balance_units=10000 WHERE user_id=1`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`WITH inserted AS (
		INSERT INTO ai_tool(id,code,name,type,current_version,status,created_by,updated_by) VALUES(1,'fixture','Fixture',2,1,1,1,1) RETURNING id
	) INSERT INTO ai_tool_version(id,tool_id,version,type,description,input_schema,output_schema,builtin_key,has_side_effect,idempotency_mode,created_by)
	SELECT 1,id,1,2,'fixture','{}','{}','currentTime',0,0,1 FROM inserted`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	policy := completeClaimPolicy(t)
	row, _, err := repo.Create(ctx, fixtureInput(), fixtureSnapshot(), policy, now)
	if err != nil {
		t.Fatal(err)
	}
	token := uuid.NewString()
	if _, err = repo.Claim(ctx, row.ID, token, now); err != nil {
		t.Fatal(err)
	}
	request, err := repo.StartRequest(ctx, row.ID, token, 1, json.RawMessage(`{}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.FinishRequest(ctx, row.ID, token, request.ID, pr.TextResponse{Usage: pr.TokenUsage{Known: true}}, nil, now); err != nil {
		t.Fatal(err)
	}
	call, err := repo.StartTool(ctx, row.ID, token, request.ID, ToolSnapshot{ToolID: 1, VersionID: 1, Name: "currentTime"}, pr.FunctionCall{CallID: "call-fixture", Name: "currentTime", Arguments: "{}"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.FinishTool(ctx, row.ID, token, call.ID, json.RawMessage(`{}`), true, now); err != nil {
		t.Fatal(err)
	}
	other := fixtureInput()
	other.PlatformID, other.EstimatedUnits = 2, 1
	if _, _, err = repo.Create(ctx, other, fixtureSnapshot(), policy, now); !errors.Is(err, ErrUnknownInflight) {
		t.Fatalf("unknown tool admitted before FinishText: %v", err)
	}
}
