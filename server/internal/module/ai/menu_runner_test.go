package ai

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIMenuRunnerAuditsBackupTransactionReplayAndPublication(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell runner requires pwsh")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(root, "docs/database/2026-10-28-ai-menu.ps1")
	if _, err := os.Stat(runner); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mode, state      string
		stopped, ok, committed bool
		applies, publishes     int
	}{
		{"stopped assertion", "ok", "", false, false, false, 0, 0},
		{"database clients still connected", "clients", "not_started", true, false, false, 0, 0},
		{"partial menu", "partial", "not_started", true, false, false, 0, 0},
		{"active menu mutation", "invalidating", "not_started", true, false, false, 0, 0},
		{"backup failure", "backup-fail", "not_started", true, false, false, 0, 0},
		{"unreadable backup", "archive-fail", "not_started", true, false, false, 0, 0},
		{"SQL rollback", "sql-fail", "rolled_back", true, false, false, 1, 0},
		{"unknown commit", "unknown", "unknown", true, false, false, 1, 0},
		{"unrelated changed", "verify-fail", "committed", true, false, true, 1, 0},
		{"replay changed", "replay-fail", "committed", true, false, true, 2, 0},
		{"Redis failure after SQL", "redis-fail", "committed", true, false, true, 2, 1},
		{"fresh install", "ok", "committed", true, true, true, 2, 1},
		{"existing install", "existing", "committed", true, true, true, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			envFile := filepath.Join(dir, "fixture.env")
			if err := os.WriteFile(envFile, []byte("POSTGRES_DSN=host=HOST user=USER password=fixture-secret-TOKEN dbname=TARGET\nREDIS_URL=redis://:fixture-secret-TOKEN@HOST:6379\n"), 0600); err != nil {
				t.Fatal(err)
			}
			harness := filepath.Join(dir, "harness.ps1")
			if err := os.WriteFile(harness, []byte(aiMenuRunnerHarness), 0600); err != nil {
				t.Fatal(err)
			}
			audit, calls := filepath.Join(dir, "audit"), filepath.Join(dir, "calls.txt")
			cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-File", harness)
			cmd.Env = append(os.Environ(), "AI_MENU_RUNNER="+runner, "AI_MENU_ENV="+envFile, "AI_MENU_AUDIT="+audit, "AI_MENU_CALLS="+calls, "AI_MENU_MODE="+tc.mode, "AI_MENU_STOPPED="+map[bool]string{true: "1", false: "0"}[tc.stopped])
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("success=%v want=%v: %s", err == nil, tc.ok, output)
			}
			if strings.Contains(string(output), "fixture-secret-TOKEN") {
				t.Fatal("connection secret leaked")
			}
			raw, _ := os.ReadFile(calls)
			if strings.Count(string(raw), "apply") != tc.applies || strings.Count(string(raw), "publish") != tc.publishes {
				t.Fatalf("side effects=%s: %s", raw, output)
			}
			if !tc.stopped {
				return
			}
			raw, err = os.ReadFile(filepath.Join(audit, "audit.json"))
			if err != nil {
				t.Fatalf("audit missing: %v, %s", err, output)
			}
			if strings.Contains(string(raw), "fixture-secret-TOKEN") {
				t.Fatal("audit leaked connection secret")
			}
			var outcome struct {
				Committed      bool
				CommitState    string
				CompletedAt    string
				FailedAt       string
				SQLSHA256      string
				RedisPublished bool
			}
			if err := json.Unmarshal(raw, &outcome); err != nil {
				t.Fatal(err)
			}
			if outcome.Committed != tc.committed || outcome.CommitState != tc.state || outcome.SQLSHA256 == "" {
				t.Fatalf("bad audit: %s", raw)
			}
			if tc.ok && (outcome.CompletedAt == "" || !outcome.RedisPublished) || !tc.ok && outcome.FailedAt == "" {
				t.Fatalf("outcome missing: %s", raw)
			}
			if tc.committed && !tc.ok && !strings.Contains(string(output), "PostgreSQL 已提交") {
				t.Fatalf("committed failure misreported: %s", output)
			}
		})
	}
}

const aiMenuRunnerHarness = `
$ErrorActionPreference='Stop'
$global:reads=0
function global:psql {
 $global:LASTEXITCODE=0
 $all=$args -join ' '
 if($all.Contains('pg_stat_activity')) { if($env:AI_MENU_MODE -eq 'clients') { return '1' }; return '0' }
 if(-not $all.Contains('--single-transaction') -or -not $all.Contains('2026-10-28-ai-menu.sql')) { throw 'menu SQL must use its single transaction' }
 Add-Content -LiteralPath $env:AI_MENU_CALLS -Value 'apply'
 if($env:AI_MENU_MODE -eq 'sql-fail') { $global:LASTEXITCODE=3 }
 if($env:AI_MENU_MODE -eq 'unknown') { $global:LASTEXITCODE=2 }
}
function global:go {
 $global:LASTEXITCODE=0
 if($args[0] -ne 'run' -or $args[1] -ne './cmd/ai-menu-state') { throw 'unexpected Go command' }
 $apply=$args -contains '--apply'
 if($apply) {
  if(-not ($args -contains '--services-stopped')) { throw 'missing stopped assertion' }
  Add-Content -LiteralPath $env:AI_MENU_CALLS -Value 'publish'
  if($env:AI_MENU_MODE -eq 'redis-fail') { $global:LASTEXITCODE=1;return }
 } else { $global:reads++ }
 $installed=$env:AI_MENU_MODE -eq 'existing' -or $global:reads -gt 1
 $nodes=0;$version=24;$dirs=0;$pages=0;$actions=0;$digest='empty'
 if($installed) {$nodes=45;$dirs=1;$pages=6;$actions=38;$version=25;$digest='seeded'}
 if($env:AI_MENU_MODE -eq 'partial') {$nodes=1}
 $other='stable'
 if(($env:AI_MENU_MODE -eq 'verify-fail' -and $global:reads -eq 2) -or ($env:AI_MENU_MODE -eq 'replay-fail' -and $global:reads -eq 3)) {$other='changed'}
 @{dryRun=(-not $apply);facts=@{platformId=1;version=$version;nodes=$nodes;directories=$dirs;pages=$pages;actions=$actions;menuDigest=$digest;otherMenuDigest=$other;otherPlatformDigest='stable';roleMenuDigest='stable'};redisFound=$true;redisState=$(if($env:AI_MENU_MODE -eq 'invalidating'){'invalidating'}else{'ready'});redisVersion=$(if($apply){$version}else{24})} | ConvertTo-Json -Depth 8 -Compress
}
function global:pg_dump {
 $global:LASTEXITCODE=0
 if($env:AI_MENU_MODE -eq 'backup-fail') {$global:LASTEXITCODE=1;return}
 $file=($args|Where-Object {$_ -like '--file=*'}|Select-Object -First 1).Substring(7)
 Set-Content -LiteralPath $file -Value 'fixture archive'
}
function global:pg_restore {$global:LASTEXITCODE=0;if($env:AI_MENU_MODE -eq 'archive-fail'){$global:LASTEXITCODE=1};'archive list'}
function global:icacls.exe {$global:LASTEXITCODE=0}
try {
 $p=@{EnvFile=$env:AI_MENU_ENV;AuditDirectory=$env:AI_MENU_AUDIT}
 if($env:AI_MENU_STOPPED -eq '1'){$p.ServicesStopped=$true}
 & $env:AI_MENU_RUNNER @p
 exit 0
} catch {Write-Output $_.Exception.Message;exit 1}
`
