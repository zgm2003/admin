package ai

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestAICoreRunnerMaintainsBackupCommitAuditAndReplay(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell runner tests require pwsh")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(root, "docs", "database", "2026-10-28-ai-core-seed.ps1")
	if _, err := os.Stat(runner); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mode, state      string
		stopped, ok, committed bool
		applies                int
	}{
		{"stopped assertion", "ok", "", false, false, false, 0},
		{"backup fails", "backup-fail", "not_started", true, false, false, 0},
		{"archive unreadable", "archive-fail", "not_started", true, false, false, 0},
		{"SQL rolls back", "sql-fail", "rolled_back", true, false, false, 1},
		{"connection lost", "unknown", "unknown", true, false, false, 1},
		{"verify fails", "verify-fail", "committed", true, false, true, 1},
		{"replay changes facts", "replay-fail", "committed", true, false, true, 2},
		{"fresh install", "ok", "committed", true, true, true, 2},
		{"existing published generations", "existing", "committed", true, true, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			envFile := filepath.Join(dir, "fixture.env")
			if err := os.WriteFile(envFile, []byte("POSTGRES_DSN=host=HOST user=USER password=fixture-secret-TOKEN dbname=TARGET\n"), 0600); err != nil {
				t.Fatal(err)
			}
			harness := filepath.Join(dir, "harness.ps1")
			if err := os.WriteFile(harness, []byte(aiCoreRunnerHarness), 0600); err != nil {
				t.Fatal(err)
			}
			audit := filepath.Join(dir, "audit")
			calls := filepath.Join(dir, "calls.txt")
			cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-File", harness)
			cmd.Env = append(os.Environ(), "AI_CORE_TEST_RUNNER="+runner, "AI_CORE_TEST_ENV="+envFile, "AI_CORE_TEST_AUDIT="+audit, "AI_CORE_TEST_CALLS="+calls, "AI_CORE_TEST_MODE="+tc.mode, "AI_CORE_TEST_STOPPED="+map[bool]string{true: "1", false: "0"}[tc.stopped])
			output, runErr := cmd.CombinedOutput()
			if (runErr == nil) != tc.ok {
				t.Fatalf("success=%v want=%v: %s", runErr == nil, tc.ok, output)
			}
			if strings.Contains(string(output), "fixture-secret-TOKEN") {
				t.Fatal("runner leaked connection secret")
			}
			raw, _ := os.ReadFile(calls)
			if got := strings.Count(string(raw), "apply"); got != tc.applies {
				t.Fatalf("applies=%d want=%d: %s", got, tc.applies, output)
			}
			if !tc.stopped {
				return
			}
			raw, err = os.ReadFile(filepath.Join(audit, "audit.json"))
			if err != nil {
				t.Fatalf("audit missing: %v: %s", err, output)
			}
			if strings.Contains(string(raw), "fixture-secret-TOKEN") {
				t.Fatal("audit leaked connection secret")
			}
			var facts struct {
				Committed   bool              `json:"committed"`
				CommitState string            `json:"commitState"`
				CompletedAt string            `json:"completedAt"`
				FailedAt    string            `json:"failedAt"`
				SQLSHA256   map[string]string `json:"sqlSHA256"`
			}
			if err := json.Unmarshal(raw, &facts); err != nil {
				t.Fatal(err)
			}
			if facts.Committed != tc.committed || facts.CommitState != tc.state {
				t.Fatalf("wrong commit audit: %s", raw)
			}
			if len(facts.SQLSHA256) != 2 {
				t.Fatalf("SQL hashes missing: %s", raw)
			}
			if tc.ok && facts.CompletedAt == "" || !tc.ok && facts.FailedAt == "" {
				t.Fatalf("missing outcome audit: %s", raw)
			}
			if tc.committed && !tc.ok && !strings.Contains(string(output), "PostgreSQL 已提交") {
				t.Fatalf("committed failure misreported: %s", output)
			}
		})
	}
}

func TestAICoreRunnerReadOnlyFactsAgainstIsolatedPostgreSQL(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell runner validation requires pwsh")
	}
	if _, err := exec.LookPath("psql"); err != nil {
		t.Skip("runner read-only validation requires psql")
	}
	db, ctx, sql := aiCoreSeedFixture(t)
	if err := db.WithContext(ctx).Exec(`CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY); CREATE TABLE storage_file(id bigint PRIMARY KEY); CREATE TABLE permission_menu(id bigint PRIMARY KEY);`).Error; err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := db.WithContext(ctx).Raw(`SELECT current_schema()`).Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	harness := filepath.Join(dir, "read-facts.ps1")
	if err := os.WriteFile(harness, []byte(aiCoreReadOnlyHarness), 0600); err != nil {
		t.Fatal(err)
	}
	beforeFile := filepath.Join(dir, "before.json")
	for step := 0; step < 2; step++ {
		if step == 1 {
			if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec(aiCoreSQL(t)).Error; err != nil {
					return err
				}
				return tx.Exec(sql).Error
			}); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-File", harness)
		cmd.Env = append(os.Environ(), "AI_CORE_SOURCE="+filepath.Join(root, "docs", "database", "2026-10-28-ai-core-seed.ps1"), "AI_CORE_SCHEMA="+schema, "AI_CORE_BEFORE="+beforeFile, "AI_CORE_STEP="+map[int]string{0: "before", 1: "after"}[step])
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("real read-only runner facts failed (%s): %v (context: %v): %s", map[int]string{0: "before", 1: "after"}[step], err, ctx.Err(), output)
		}
	}
}

// Load only the trusted runner's declarations through its PowerShell AST.
// The top-level runner (backup/apply/maintenance-window checks) is never run.
const aiCoreReadOnlyHarness = `
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$tokens=$null;$errors=$null
$ast=[System.Management.Automation.Language.Parser]::ParseFile($env:AI_CORE_SOURCE,[ref]$tokens,[ref]$errors)
if($errors.Count -ne 0){throw 'runner syntax is invalid'}
foreach($name in @('Read-JsonQuery','Read-Facts','Json','Assert-ExistingRows','Assert-SeedFacts')) {
 $nodes=@($ast.FindAll({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name},$true))
 if($nodes.Count -ne 1){throw 'runner declaration missing'}
 Invoke-Expression $nodes[0].Extent.Text
}
foreach($name in @('$policyKeys','$policySQL')) {
 $nodes=@($ast.FindAll({param($node) $node -is [System.Management.Automation.Language.AssignmentStatementAst] -and $node.Left.Extent.Text -eq $name},$true))
 if($nodes.Count -ne 1){throw 'runner policy declaration missing'}
 Invoke-Expression $nodes[0].Extent.Text
}
if($env:AI_CORE_SCHEMA -notmatch '^test_ai_core_seed_[0-9]+$'){throw 'test schema required'}
$dsn=$env:POSTGRES_DSN -replace '(?i)\s+TimeZone=(?:''[^'']*''|"[^"]*"|\S+)',''
$env:PGOPTIONS='-c search_path='+$env:AI_CORE_SCHEMA+' -c default_transaction_read_only=on -c statement_timeout=10000'
$env:PGCLIENTENCODING='UTF8'
$env:PGCONNECT_TIMEOUT='10'
try {
 $facts=Read-Facts
 if($env:AI_CORE_STEP -eq 'before') {
  if($facts.coreTableCount -ne 0){throw 'test fixture already contains AI tables'}
  Json $facts | Set-Content -LiteralPath $env:AI_CORE_BEFORE -Encoding utf8
 } else {
  $before=Get-Content -LiteralPath $env:AI_CORE_BEFORE -Raw | ConvertFrom-Json
  Assert-SeedFacts $before $facts
 }
} catch {
 # Do not surface libpq arguments or the source line containing credentials.
 Write-Output $_.Exception.Message
 exit 1
}
`

const aiCoreRunnerHarness = `
$ErrorActionPreference='Stop'
$global:reads=0
function global:psql {
 $global:LASTEXITCODE=0
 $all=$args -join ' '
 if ($all.Contains('pg_stat_activity')) {return '0'}
 if ($all.Contains('--file=')) {
  $files=@($args | Where-Object {$_ -like '--file=*'})
  if (-not $all.Contains('--single-transaction') -or $files.Count -ne 2 -or
      -not $files[0].EndsWith('2026-10-28-ai-core.sql') -or -not $files[1].EndsWith('2026-10-28-ai-core-seed.sql')) {throw 'core and seed must execute in one ordered transaction'}
  Add-Content -LiteralPath $env:AI_CORE_TEST_CALLS -Value 'apply'
  if ($env:AI_CORE_TEST_MODE -eq 'sql-fail') {$global:LASTEXITCODE=3}
  if ($env:AI_CORE_TEST_MODE -eq 'unknown') {$global:LASTEXITCODE=2}
  return
 }
 $global:reads++
 $existing=$env:AI_CORE_TEST_MODE -eq 'existing'
 $installed=$existing -or $global:reads -gt 1
 $rows=@()
 $state=$null
 $outbox=@()
 $schedules=@()
 $settingGeneration=7
 if ($installed) {
  $keys=@('ai.run.user_inflight_limit','ai.run.user_rate_per_second','ai.run.user_rate_burst','ai.run.global_rate_per_second','ai.run.global_rate_burst','ai.run.queued_backpressure_threshold','ai.run.active_backpressure_threshold','ai.run.max_queued_seconds','ai.run.text_timeout_seconds','ai.run.image_timeout_seconds','ai.run.audio_timeout_seconds','ai.run.video_timeout_seconds','ai.run.max_model_rounds','ai.run.max_tool_calls','ai.run.max_bound_tools','ai.run.max_input_attachments','ai.run.max_output_tokens','ai.run.max_text_input_bytes','ai.run.max_images','ai.media.image_max_bytes','ai.media.audio_max_bytes','ai.media.video_max_bytes','ai.media.download_timeout_seconds','ai.media.transfer_timeout_seconds','ai.media.spool_max_bytes_per_worker','ai.run.event_retention_days')
  $id=0
  $rows=@($keys | ForEach-Object {$id++; @{id=$id;setting_key=$_;value='9';value_type=2;status=1;is_builtin=1;deleted_at=$null;created_at='2026-10-28';updated_at='2026-10-28';description='policy'}})
  $generation=1
  if($existing){$generation=5}
  $state=@{namespace='ai.runtime';scope_key='global';generation=$generation;created_at='2026-10-28';updated_at='2026-10-28'}
  $published=$null
  if($existing){$published='2026-10-28'}
  $outbox=@(@{id=1;namespace='ai.runtime';scope_key='global';generation=$generation;attempts=0;published_at=$published})
  if(-not $existing) {$settingGeneration=8; $outbox+=@{id=2;namespace='system.setting';scope_key='global';generation=8;attempts=0;published_at=$null}}
  $schedules=@(@{builtin_key='ai.run.recover';task_type='ai.run.recover';cron_expression='* * * * *';timezone='Asia/Shanghai';params=@{};status=0;next_run_at=$null;deleted_at=$null},@{builtin_key='ai.run.event.cleanup';task_type='ai.run.event.cleanup';cron_expression='0 3 * * *';timezone='Asia/Shanghai';params=@{};status=0;next_run_at=$null;deleted_at=$null})
 }
 $menu='stable'
 if(($env:AI_CORE_TEST_MODE -eq 'verify-fail' -and $global:reads -eq 2) -or ($env:AI_CORE_TEST_MODE -eq 'replay-fail' -and $global:reads -eq 3)) {$menu='changed'}
 return (@{coreTableCount=$(if($installed){21}else{0});coreSchemaStamp=$(if($installed){'admin-ai-core-v1:digest'}else{$null});policyRows=$rows;runtimeState=$state;settingState=@{namespace='system.setting';scope_key='global';generation=$settingGeneration;created_at='2026-10-28';updated_at='2026-10-28'};outboxRows=$outbox;schedules=$schedules;otherSettingsDigest='stable';otherCacheDigest='stable';otherSchedulesDigest='stable';menuDigest=$menu} | ConvertTo-Json -Depth 12 -Compress)
}
function global:pg_dump {
 $global:LASTEXITCODE=0
 if($env:AI_CORE_TEST_MODE -eq 'backup-fail') {$global:LASTEXITCODE=1;return}
 $file=($args|Where-Object {$_ -like '--file=*'}|Select-Object -First 1).Substring(7)
 Set-Content -LiteralPath $file -Value 'fixture archive'
}
function global:pg_restore {$global:LASTEXITCODE=0;if($env:AI_CORE_TEST_MODE -eq 'archive-fail') {$global:LASTEXITCODE=1};return 'fixture archive list'}
function global:icacls.exe {$global:LASTEXITCODE=0}
try {
 $p=@{EnvFile=$env:AI_CORE_TEST_ENV;AuditDirectory=$env:AI_CORE_TEST_AUDIT}
 if($env:AI_CORE_TEST_STOPPED -eq '1') {$p.ServicesStopped=$true}
 & $env:AI_CORE_TEST_RUNNER @p
 exit 0
} catch {Write-Output $_.Exception.Message;exit 1}
`
