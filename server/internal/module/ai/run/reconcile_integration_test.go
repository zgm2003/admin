package run

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"admin/server/internal/module/ai/usage/charge"
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

func TestReconcileRepositoryDryRunApplyAndReplay(t *testing.T) {
	repo, db, ctx := runDB(t)
	snap := fixtureSnapshot()
	snap.Config = json.RawMessage(`{"rates":[{"id":1,"operation":"text.generate","usageKey":"input","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":1},{"id":2,"operation":"text.generate","usageKey":"output","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":1}]}`)
	policy, _ := json.Marshal(testGatePolicy(t))
	snap.Policy = policy
	row, _, err := repo.Create(ctx, fixtureInput(), snap, policy, snap.AcceptedAt)
	if err != nil {
		t.Fatal(err)
	}
	row, err = repo.Find(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := pr.Fact{RunID: row.ID, Sequence: 1, RequestKey: uuid.NewString(), RequestFingerprint: strings.Repeat("a", 64), Status: 4, UsageStatus: 2, RequestSummary: json.RawMessage(`{}`), ResponseItems: json.RawMessage(`[]`), Usage: json.RawMessage(`{"known":false}`), CreatedAt: now, UpdatedAt: now}
	if err = db.WithContext(ctx).Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return repo.finishTextTx(ctx, tx, row, 4, "USAGE_UNKNOWN", "", now) }); err != nil {
		t.Fatal(err)
	}
	row, err = repo.Find(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	evidence := ReconcileEvidence{SchemaVersion: 1, RunID: row.ID, Operation: row.Operation, ProviderModelID: row.ProviderModelID, PriceVersionID: row.PriceVersionID, SnapshotDigest: frozenReconcileDigest(row.ConfigSnapshot, row.PolicySnapshot), Requests: []ReconcileRequestEvidence{{RequestID: request.ID, RequestKey: request.RequestKey, RequestFingerprint: request.RequestFingerprint, UpstreamRequestID: "supplier-proof-id", SourceReference: "supplier report line 3", ObservedAt: time.Now().UTC(), Status: 2, Usage: json.RawMessage(`{"known":true,"inputTokens":2,"cachedInputTokens":0,"outputTokens":3}`)}}}
	raw, _ := json.Marshal(evidence)
	service := NewReconcileService(NewReconcileRepository(db, repo.scheduler, repo.events))
	dry, err := service.Execute(ctx, row.ID, raw, "operator", false)
	if err != nil || !dry.DryRun || !dry.Complete {
		t.Fatalf("%+v %v", dry, err)
	}
	var unchanged pr.Fact
	if err = db.WithContext(ctx).First(&unchanged, request.ID).Error; err != nil || unchanged.UsageStatus != 2 {
		t.Fatalf("dry run wrote facts %+v %v", unchanged, err)
	}
	applied, err := service.Execute(ctx, row.ID, raw, "operator", true)
	if err != nil || !applied.SettlementQueued || applied.DryRun {
		t.Fatalf("%+v %v", applied, err)
	}
	after, err := repo.Find(ctx, row.ID)
	if err != nil || after.ExecutionStatus != 4 || after.ReservationStatus != 1 || after.UsageStatus != 1 || after.BillingStatus != 1 {
		t.Fatalf("%+v %v", after, err)
	}
	var ch charge.Model
	if err = db.WithContext(ctx).Where("run_id=?", row.ID).First(&ch).Error; err != nil || ch.Status != 1 || ch.ActualUnits == nil {
		t.Fatalf("%+v %v", ch, err)
	}
	again, err := service.Execute(ctx, row.ID, raw, "operator", true)
	if err != nil || !again.AlreadyApplied || again.SettlementQueued {
		t.Fatalf("replay=%+v %v", again, err)
	}
	var jobs, items int64
	db.WithContext(ctx).Table("system_scheduler_job").Where("task_type='ai.usage.settle'").Count(&jobs)
	db.WithContext(ctx).Table("ai_usage_charge_item").Where("charge_id=?", ch.ID).Count(&items)
	if jobs != 1 || items != 2 {
		t.Fatalf("jobs=%d items=%d", jobs, items)
	}
	if err = db.WithContext(ctx).First(&unchanged, request.ID).Error; err != nil {
		t.Fatal(err)
	}
	var audit struct {
		Reconciliation ReconcileAudit `json:"reconciliation"`
	}
	if json.Unmarshal(unchanged.RequestSummary, &audit) != nil || audit.Reconciliation.Operator != "operator" || audit.Reconciliation.EvidenceDigest == "" {
		t.Fatalf("missing durable audit %s", unchanged.RequestSummary)
	}
}
