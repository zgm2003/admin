package run

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"admin/server/internal/module/ai/usage/charge"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func reconcileFixture(t *testing.T) (ReconcileState, ReconcileEvidence) {
	t.Helper()
	now := time.Now().UTC()
	snap := fixtureSnapshot()
	snap.Config = json.RawMessage(`{"rates":[{"id":1,"operation":"text.generate","usageKey":"input","selector":{},"unit":"token","unitQuantity":"1","billingUnitPriceUnits":1},{"id":2,"operation":"text.generate","usageKey":"output","selector":{},"unit":"token","unitQuantity":"1","billingUnitPriceUnits":1}]}`)
	config, _ := json.Marshal(snap)
	p := testGatePolicy(t)
	policy, _ := json.Marshal(p)
	digest := frozenReconcileDigest(config, policy)
	u := json.RawMessage(`{"known":false}`)
	state := ReconcileState{Run: Model{ID: 1, UserID: 7, PlatformID: 8, AgentID: snap.AgentID, BindingID: snap.BindingID, ProviderModelID: snap.ProviderModelID, ProviderVersionID: snap.ProviderVersionID, OfficialModelID: snap.OfficialModelID, PriceVersionID: snap.PriceVersionID, MultiplierPPM: snap.MultiplierPPM, Operation: "text.generate", ExecutionStatus: 4, UsageStatus: 2, BillingStatus: 4, ReservationStatus: 1, ConfigSnapshot: config, PolicySnapshot: policy}, Charge: charge.Model{ID: 2, RunID: 1, UserID: 7, PlatformID: 8, Status: 4, Currency: "CNY", PriceVersionID: snap.PriceVersionID, MultiplierPPM: snap.MultiplierPPM, UsageComplete: 0, SnapshotDigest: digest}, Requests: []pr.Fact{{ID: 3, RunID: 1, RequestKey: "request-key", RequestFingerprint: strings.Repeat("a", 64), Status: 4, UsageStatus: 2, Usage: u, RequestSummary: json.RawMessage(`{}`)}}}
	evidence := ReconcileEvidence{SchemaVersion: 1, RunID: 1, Operation: "text.generate", ProviderModelID: snap.ProviderModelID, PriceVersionID: snap.PriceVersionID, SnapshotDigest: digest, Requests: []ReconcileRequestEvidence{{RequestID: 3, RequestKey: "request-key", RequestFingerprint: strings.Repeat("a", 64), UpstreamRequestID: "upstream-trace", SourceReference: "supplier export line 5", ObservedAt: now, Status: 2, Usage: json.RawMessage(`{"known":true,"inputTokens":2,"cachedInputTokens":0,"outputTokens":3}`)}}}
	return state, evidence
}
func TestReconciliationUsesFrozenRatesAndNeverReopensExecution(t *testing.T) {
	state, e := reconcileFixture(t)
	p, err := planReconciliation(state, e, "operator", fmt.Sprintf("%x", sha256.Sum256([]byte("evidence"))), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !p.Complete || p.ActualUnits == nil || *p.ActualUnits != 5 || len(p.Items) != 2 || len(p.Requests) != 1 {
		t.Fatalf("%+v", p)
	}
	if state.Run.ExecutionStatus != 4 || state.Run.ReservationStatus != 1 {
		t.Fatal("terminal or reservation changed")
	}
	if p.Requests[0].Audit.Operator != "operator" || p.Requests[0].Audit.SourceReference == "" {
		t.Fatal("missing audit")
	}
}
func TestReconciliationRejectsWrongIdentityAndIncompleteUsage(t *testing.T) {
	state, e := reconcileFixture(t)
	for _, mutate := range []func(*ReconcileEvidence){func(v *ReconcileEvidence) { v.RunID = 9 }, func(v *ReconcileEvidence) { v.PriceVersionID = 99 }, func(v *ReconcileEvidence) { v.Requests[0].RequestKey = "wrong" }, func(v *ReconcileEvidence) { v.Requests[0].Usage = json.RawMessage(`{"known":true,"inputTokens":0}`) }, func(v *ReconcileEvidence) { v.SnapshotDigest = "wrong" }} {
		b, _ := json.Marshal(e)
		var copy ReconcileEvidence
		json.Unmarshal(b, &copy)
		mutate(&copy)
		if _, err := planReconciliation(state, copy, "operator", "digest", time.Now().UTC()); err == nil {
			t.Fatal("unproven usage accepted")
		}
	}
}
func TestReconciliationBlocksUnknownToolSideEffects(t *testing.T) {
	state, e := reconcileFixture(t)
	state.UnknownTools = true
	p, err := planReconciliation(state, e, "operator", "digest", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if p.Complete || p.ActualUnits != nil || len(p.Items) > 0 {
		t.Fatal("unknown tool was silently resolved")
	}
}
func TestEvidenceDecodeRejectsAmountsAndTargetUser(t *testing.T) {
	for _, s := range []string{`{"schemaVersion":1,"userId":"7"}`, `{"schemaVersion":1,"amountUnits":10}`, `{} {}`, `null`} {
		if _, err := DecodeReconcileEvidence([]byte(s)); err == nil {
			t.Fatalf("malformed evidence accepted: %s", s)
		}
	}
}

type reconcileRepositoryProbe struct {
	state   ReconcileState
	applied bool
}

func (p *reconcileRepositoryProbe) Review(_ context.Context, _ int64, apply bool, fn func(ReconcileState) (ReconcilePlan, error)) (ReconcilePlan, error) {
	p.applied = apply
	return fn(p.state)
}
func TestReconcileServiceDryRunDoesNotApply(t *testing.T) {
	state, e := reconcileFixture(t)
	raw, _ := json.Marshal(e)
	probe := &reconcileRepositoryProbe{state: state}
	out, err := NewReconcileService(probe).Execute(context.Background(), 1, raw, "dry-run", false)
	if err != nil || probe.applied || !out.DryRun || out.SettlementQueued || out.ActualUnits == nil || *out.ActualUnits != 5 {
		t.Fatalf("%+v err=%v applied=%v", out, err, probe.applied)
	}
}
func TestReconciliationEvidenceReplayDoesNotRecreateChargeItems(t *testing.T) {
	state, e := reconcileFixture(t)
	p, err := planReconciliation(state, e, "operator", "digest", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	update := p.Requests[0]
	state.Requests[0].Status = update.Status
	state.Requests[0].UsageStatus = 1
	state.Requests[0].Usage = update.Usage
	state.Requests[0].RequestSummary, _ = json.Marshal(map[string]any{"reconciliation": update.Audit})
	state.Charge.Status = 2
	state.Charge.UsageComplete = 1
	state.Charge.ActualUnits = p.ActualUnits
	state.Run.ReservationStatus = 2
	state.ExistingItems = true
	again, err := planReconciliation(state, e, "operator", "digest", time.Now().UTC())
	if err != nil || !again.Report.AlreadyApplied || len(again.Requests) != 0 || len(again.Items) != 0 {
		t.Fatalf("%+v err=%v", again, err)
	}
	if _, err = planReconciliation(state, e, "operator", "changed-file", time.Now().UTC()); err == nil {
		t.Fatal("known usage overwritten by another evidence file")
	}
}
func TestPartialReconciliationKeepsUnknownReservation(t *testing.T) {
	state, e := reconcileFixture(t)
	state.Requests = append(state.Requests, pr.Fact{ID: 4, RunID: 1, Status: 4, UsageStatus: 2, Usage: json.RawMessage(`{"known":false}`)})
	p, err := planReconciliation(state, e, "operator", "digest", time.Now().UTC())
	if err != nil || p.Complete || p.ActualUnits != nil || len(p.Items) != 0 || len(p.Requests) != 1 || state.Run.ReservationStatus != 1 {
		t.Fatalf("%+v err=%v", p, err)
	}
}
func TestEvidenceRejectsDuplicateKeysAndCredentialReferences(t *testing.T) {
	if _, err := DecodeReconcileEvidence([]byte(`{"schemaVersion":1,"schemaVersion":1}`)); err == nil {
		t.Fatal("duplicate evidence key accepted")
	}
	for _, reference := range []string{"https://user:password@example.test/report", "https://example.test/report?token=secret"} {
		if validReconcileReference(reference) {
			t.Fatal("credentials accepted in persistent audit")
		}
	}
}
