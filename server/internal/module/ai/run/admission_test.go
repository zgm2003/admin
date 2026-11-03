package run

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAdmissionAllowsFirstVariableChargeWithPositiveBalance(t *testing.T) {
	decision, err := Admit(AdmissionInput{BalanceUnits: 1, EstimatedUnits: 100, VariableUsage: true, InflightLimit: 2, Now: time.Now()})
	if err != nil || !decision.Allowed {
		t.Fatalf("expected first variable request to be admitted: %+v %v", decision, err)
	}
}

func TestAdmissionRejectsUnknownInflightAndRequiresCoverageForAdditionalVariableRun(t *testing.T) {
	if d, err := Admit(AdmissionInput{BalanceUnits: 100, EstimatedUnits: 1, UnknownInflight: true, InflightLimit: 2, Now: time.Now()}); err == nil || d.Allowed {
		t.Fatalf("unknown inflight must fail closed: %+v %v", d, err)
	}
	if d, err := Admit(AdmissionInput{BalanceUnits: 100, EstimatedUnits: 80, VariableUsage: true, InflightEstimatedUnits: 30, InflightCount: 1, InflightLimit: 2, Now: time.Now()}); err == nil || d.Allowed {
		t.Fatalf("insufficient coverage must be rejected: %+v %v", d, err)
	}
}

func TestRuntimeSnapshotCanonicalDigestAndStrictObject(t *testing.T) {
	s := RuntimeSnapshot{AgentID: 1, BindingID: 2, ProviderModelID: 3, ProviderVersionID: 4, OfficialModelID: 6, PriceVersionID: 5, MultiplierPPM: 1000000, Prompt: "p", Config: json.RawMessage(`{"b":2,"a":1}`), Policy: json.RawMessage(`{}`), AcceptedAt: time.Unix(10, 0).UTC()}
	digest, err := s.Digest()
	if err != nil || len(digest) != 64 {
		t.Fatalf("digest=%q err=%v", digest, err)
	}
	if _, err := NewRuntimeSnapshot(RuntimeSnapshot{Config: json.RawMessage(`[]`), Policy: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("array config must be rejected")
	}
}

func TestRuntimeSnapshotOwnsItsToolsAndRejectsTrailingJunk(t *testing.T) {
	s := fixtureSnapshot()
	s.Tools = []ToolSnapshot{{ToolID: 2, VersionID: 2, Name: "later", Config: json.RawMessage(`{}`)}, {ToolID: 1, VersionID: 1, Name: "first", Config: json.RawMessage(`{}`)}}
	frozen, err := NewRuntimeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	if s.Tools[0].Name != "later" {
		t.Fatal("normalization mutated the source binding slice")
	}
	s.Tools[0].Config[0] = '['
	if string(frozen.Tools[1].Config) != "{}" {
		t.Fatal("snapshot shares tool configuration bytes")
	}
	s = fixtureSnapshot()
	s.Config = json.RawMessage(`{} junk`)
	if _, err := NewRuntimeSnapshot(s); err == nil {
		t.Fatal("trailing JSON junk accepted")
	}
}
