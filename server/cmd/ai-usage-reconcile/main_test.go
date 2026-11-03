package main

import (
	"admin/server/internal/module/ai/run"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIReconciliationDefaultsToDryRunAndRejectsAmountOrUser(t *testing.T) {
	v, e := parseArgs([]string{"--run-id", "1", "--evidence", "proof.json"}, &bytes.Buffer{})
	if e != nil || v.Apply || v.RunID != 1 {
		t.Fatalf("%+v %v", v, e)
	}
	for _, args := range [][]string{{"--run-id", "1", "--evidence", "proof.json", "--apply"}, {"--run-id", "1", "--evidence", "proof.json", "--amount", "1"}, {"--run-id", "1", "--evidence", "proof.json", "--user-id", "2"}} {
		if _, e := parseArgs(args, &bytes.Buffer{}); e == nil {
			t.Fatalf("unsafe arguments accepted: %v", args)
		}
	}
}

type reconcileProbe struct {
	called bool
	apply  bool
}

func (p *reconcileProbe) Execute(context.Context, int64, []byte, string, bool) (run.ReconcileReport, error) {
	p.called = true
	return run.ReconcileReport{DryRun: true}, nil
}
func TestCLIRejectsOversizedEvidenceBeforeService(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.json")
	if e := os.WriteFile(path, make([]byte, run.MaxReconcileEvidenceBytes+1), 0600); e != nil {
		t.Fatal(e)
	}
	p := &reconcileProbe{}
	if e := execute(context.Background(), p, settings{RunID: 1, Evidence: path, Operator: "dry-run"}, &bytes.Buffer{}); e == nil || p.called {
		t.Fatal("oversized evidence reached service")
	}
}

func TestCLIRejectsEmptyDirectoryAndNetworkEvidence(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{empty, dir, "//HOST/evidence.json", string([]byte{92, 92}) + "HOST/evidence.json"} {
		if _, err := readEvidence(path); err == nil {
			t.Fatalf("invalid evidence file accepted: %q", path)
		}
	}
}
