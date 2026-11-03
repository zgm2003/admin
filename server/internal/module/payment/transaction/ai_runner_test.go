package transaction

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAILedgerRunnerBacksUpAuditsAndReplays(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		stopped    bool
		wantCalls  int
		wantCommit bool
		wantOK     bool
		wantState  string
	}{
		{"requires stopped services", "ok", false, 0, false, false, ""},
		{"backup failure", "backup-fail", true, 0, false, false, "not_started"},
		{"transaction failure", "sql-fail", true, 1, false, false, "rolled_back"},
		{"unknown commit", "unknown-commit", true, 1, false, false, "unknown"},
		{"postcommit mismatch", "after-mismatch", true, 1, true, false, "committed"},
		{"replay mismatch", "replay-mismatch", true, 2, true, false, "committed"},
		{"complete", "ok", true, 2, true, true, "committed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shell, err := exec.LookPath("pwsh")
			if err != nil {
				t.Skip("PowerShell runner validation requires pwsh")
			}
			root, err := filepath.Abs("../../../../..")
			if err != nil {
				t.Fatal(err)
			}
			runner := filepath.Join(root, "docs", "database", "2026-10-28-ai-payment.ps1")
			if _, err := os.Stat(runner); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			envFile := filepath.Join(dir, "fixture.env")
			if err := os.WriteFile(envFile, []byte("POSTGRES_DSN=host=HOST user=USER password=fixture-secret-TOKEN dbname=TARGET\n"), 0600); err != nil {
				t.Fatal(err)
			}
			harness := filepath.Join(dir, "harness.ps1")
			if err := os.WriteFile(harness, []byte(aiRunnerHarness), 0600); err != nil {
				t.Fatal(err)
			}
			audit := filepath.Join(dir, "audit")
			calls := filepath.Join(dir, "calls.txt")
			cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-File", harness)
			cmd.Env = append(os.Environ(), "AI_TEST_RUNNER="+runner, "AI_TEST_MODE="+tc.mode, "AI_TEST_ENV="+envFile, "AI_TEST_AUDIT="+audit, "AI_TEST_CALLS="+calls, "AI_TEST_STOPPED="+map[bool]string{true: "1", false: "0"}[tc.stopped])
			output, runErr := cmd.CombinedOutput()
			if (runErr == nil) != tc.wantOK {
				t.Fatalf("success=%v, want %v: %s", runErr == nil, tc.wantOK, output)
			}
			if strings.Contains(string(output), "fixture-secret-TOKEN") {
				t.Fatal("runner leaked DSN")
			}
			raw, _ := os.ReadFile(calls)
			if got := strings.Count(string(raw), "apply"); got != tc.wantCalls {
				t.Fatalf("migration executions=%d want %d: %s", got, tc.wantCalls, output)
			}
			metadata, err := os.ReadFile(filepath.Join(audit, "audit.json"))
			if !tc.stopped {
				return
			}
			if err != nil {
				t.Fatalf("missing audit after SQL attempt: %v; %s", err, output)
			}
			if strings.Contains(string(metadata), "fixture-secret-TOKEN") {
				t.Fatal("audit contains secret")
			}
			var facts struct {
				Committed   bool   `json:"committed"`
				CommitState string `json:"commitState"`
				FailedAt    string `json:"failedAt"`
				CompletedAt string `json:"completedAt"`
			}
			if err := json.Unmarshal(metadata, &facts); err != nil {
				t.Fatal(err)
			}
			if facts.Committed != tc.wantCommit {
				t.Fatalf("commit state=%v want %v: %s", facts.Committed, tc.wantCommit, metadata)
			}
			if facts.CommitState != tc.wantState {
				t.Fatalf("commitState=%q want %q: %s", facts.CommitState, tc.wantState, metadata)
			}
			if !tc.wantOK && facts.FailedAt == "" {
				t.Fatal("failure lacks persistent audit")
			}
			if tc.wantOK && facts.CompletedAt == "" {
				t.Fatal("completed audit lacks completion time")
			}
			if tc.wantCommit && !tc.wantOK && !strings.Contains(string(output), "PostgreSQL 已提交") {
				t.Fatalf("postcommit failure not distinguished from rollback: %s", output)
			}
		})
	}
}

const aiRunnerHarness = `
$ErrorActionPreference = 'Stop'
$global:factReads = 0
function global:psql {
    $all = $args -join ' '
    $global:LASTEXITCODE = 0
    if ($all.Contains('pg_stat_activity')) { return '0' }
    if ($all.Contains('--file=')) {
        Add-Content -LiteralPath $env:AI_TEST_CALLS -Value 'apply'
        if ($env:AI_TEST_MODE -eq 'sql-fail') { $global:LASTEXITCODE = 3 }
        if ($env:AI_TEST_MODE -eq 'unknown-commit') { $global:LASTEXITCODE = 2 }
        return
    }
    $global:factReads++
    $digest = 'stable-ledger'
    if (($env:AI_TEST_MODE -eq 'after-mismatch' -and $global:factReads -eq 2) -or
        ($env:AI_TEST_MODE -eq 'replay-mismatch' -and $global:factReads -eq 3)) { $digest = 'changed' }
    return (@{legacyDigest=$digest;walletDigest='stable-wallet';ledgerCount=2;walletCount=1;guards=2;extended=($global:factReads -gt 1);invalidExtended=0;cacheDigest='stable-cache'} | ConvertTo-Json -Compress)
}
function global:pg_dump {
    $global:LASTEXITCODE = 0
    if ($env:AI_TEST_MODE -eq 'backup-fail') { $global:LASTEXITCODE = 1; return }
    $file = ($args | Where-Object { $_ -like '--file=*' } | Select-Object -First 1).Substring(7)
    Set-Content -LiteralPath $file -Value 'fixture archive'
}
function global:pg_restore { $global:LASTEXITCODE = 0; return 'fixture archive list' }
function global:icacls.exe { $global:LASTEXITCODE = 0 }
try {
    $parameters = @{EnvFile=$env:AI_TEST_ENV; AuditDirectory=$env:AI_TEST_AUDIT}
    if ($env:AI_TEST_STOPPED -eq '1') { $parameters.ServicesStopped = $true }
    & $env:AI_TEST_RUNNER @parameters
    exit 0
} catch {
    Write-Output $_.Exception.Message
    exit 1
}
`
