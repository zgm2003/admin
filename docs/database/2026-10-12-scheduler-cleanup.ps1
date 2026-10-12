param(
    [switch]$CheckOnly,
    [switch]$OldServicesStopped
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$sqlPath = Join-Path $PSScriptRoot '2026-10-12-scheduler-cleanup.sql'
$envPath = Join-Path $repoRoot 'server/.env'
$audit = [ordered]@{ migration = 'scheduler-cleanup'; postgresCommitted = $false; verified = $false }
$auditPath = $null
$originalOptions = $env:PGOPTIONS
$originalAppName = $env:PGAPPNAME

function Invoke-MaintenanceQuery([string]$Query) {
    $output = @(& psql -X -q -A -t -d $script:maintenanceDsn -v ON_ERROR_STOP=1 -c $Query 2> $script:errorPath)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 查询失败，连接信息不输出；检查本地 maintenance-errors.log。' }
    return (($output -join "`n") | ConvertFrom-Json)
}

function Read-SchedulerState {
    return Invoke-MaintenanceQuery @'
BEGIN READ ONLY;
SET LOCAL statement_timeout='30s';
SELECT json_build_object(
 'database',current_database(),
 'schema',current_schema(),
 'plans',(SELECT count(*) FROM public.system_scheduler_schedule),
 'jobs',(SELECT count(*) FROM public.system_scheduler_job),
 'runs',(SELECT count(*) FROM public.system_scheduler_run),
 'legacyColumns',(SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND
 ((table_name='system_scheduler_job' AND column_name IN ('queue','timeout_seconds','worker_id')) OR
 (table_name='system_scheduler_run' AND column_name='result_summary'))),
 'redundantIndexes',(SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname IN
 ('ux_system_scheduler_job_task_attempt','ix_system_scheduler_run_job_list','ix_system_scheduler_run_cleanup')),
 'queuedIndex',(SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='ix_system_scheduler_job_queued_lease'))
);
COMMIT;
'@
}

try {
    foreach ($toolName in @('psql','pg_dump','pg_restore')) { Get-Command $toolName -ErrorAction Stop | Out-Null }
    $fileValues = @{}
    if (Test-Path -LiteralPath $envPath) {
        foreach ($line in Get-Content -LiteralPath $envPath -Encoding UTF8) {
            if ($line -match '^\s*(?:export\s+)?([A-Z][A-Z0-9_]*)\s*=\s*(.*?)\s*$') {
                $fileValues[$Matches[1]] = $Matches[2].Trim().Trim('"').Trim("'")
            }
        }
    }
    $postgresDsn = [Environment]::GetEnvironmentVariable('POSTGRES_DSN')
    if ([string]::IsNullOrWhiteSpace($postgresDsn)) { $postgresDsn = $fileValues['POSTGRES_DSN'] }
    if ([string]::IsNullOrWhiteSpace($postgresDsn)) { throw '缺少 POSTGRES_DSN 配置。' }
    $script:maintenanceDsn = $postgresDsn -replace '(?i)\s+TimeZone=\S+', ''
    $env:PGOPTIONS = '-c search_path=public,pg_catalog'
    $env:PGAPPNAME = 'admin-scheduler-cleanup-maintenance'
    $runDirectory = Join-Path $env:LOCALAPPDATA ('Admin/maintenance/scheduler-cleanup/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
    New-Item -ItemType Directory -Path $runDirectory -Force | Out-Null
    $auditPath = Join-Path $runDirectory 'audit.json'
    $script:errorPath = Join-Path $runDirectory 'maintenance-errors.log'
    $audit.startedAt = (Get-Date).ToString('o')
    $audit.sqlSHA256 = (Get-FileHash -LiteralPath $sqlPath -Algorithm SHA256).Hash
    $audit.before = Read-SchedulerState
    if ($CheckOnly) {
        $audit.checkOnly = $true
        $audit.before | ConvertTo-Json
        return
    }
    if (-not $OldServicesStopped) { throw '迁移需要先停止本项目 API/Worker，再显式传入 -OldServicesStopped。' }
    $sessions = Invoke-MaintenanceQuery @'
SELECT json_build_object('otherConnections',count(*),
 'blockingConnections',count(*) FILTER (WHERE NOT (
 application_name='Navicat' AND state='idle' AND xact_start IS NULL
 AND NOT EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_stat_activity.pid))),
 'idleNavicatConnections',count(*) FILTER (WHERE
 application_name='Navicat' AND state='idle' AND xact_start IS NULL
 AND NOT EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_stat_activity.pid))) FROM pg_stat_activity
WHERE datname=current_database() AND backend_type='client backend' AND pid<>pg_backend_pid();
'@
    $audit.clientSessions = $sessions
    if ($sessions.blockingConnections -ne 0) { throw '当前数据库还有服务连接、活动查询或事务，迁移未执行；停止 API/Worker 并结束活动查询/事务后重试。' }
    $backupPath = Join-Path $runDirectory 'public-before.dump'
    & pg_dump --format=custom --schema=public --no-owner --no-privileges --dbname=$script:maintenanceDsn --file=$backupPath 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw '迁移前备份失败，迁移未执行。' }
    & pg_restore --list $backupPath > (Join-Path $runDirectory 'backup-list.txt') 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw '备份归档检查失败，迁移未执行。' }
    & pg_restore --no-owner --no-acl --file=NUL $backupPath 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw '备份完整读取检查失败，迁移未执行。' }
    $audit.backup = $backupPath
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backupPath -Algorithm SHA256).Hash
    & psql -X -q -d $script:maintenanceDsn --single-transaction -v ON_ERROR_STOP=1 -f $sqlPath 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw '迁移事务失败，PostgreSQL 已回滚；查看本地维护日志。' }
    $audit.postgresCommitted = $true
    $audit.after = Read-SchedulerState
    foreach ($counter in @('plans','jobs','runs')) {
        if ($audit.before.$counter -ne $audit.after.$counter) { throw 'PostgreSQL 已提交，但记录数验证失败。' }
    }
    if ($audit.after.legacyColumns -ne 0 -or $audit.after.redundantIndexes -ne 0 -or -not $audit.after.queuedIndex) {
        throw 'PostgreSQL 已提交，但目标结构验证失败。'
    }
    $audit.verified = $true
    $snapshotTemp = Join-Path $runDirectory 'current.sql'
    & pg_dump --schema-only --schema=public --no-owner --no-privileges --dbname=$script:maintenanceDsn --file=$snapshotTemp 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 已提交且验证通过，但 current.sql 导出失败。' }
    Copy-Item -LiteralPath $snapshotTemp -Destination (Join-Path $PSScriptRoot 'current.sql') -Force
    $audit.snapshotUpdated = $true
    $audit.after | ConvertTo-Json
    Write-Output "迁移完成；备份与审计：$runDirectory"
}
catch {
    $audit.failed = $true
    $audit.error = $_.Exception.Message
    Write-Error $_.Exception.Message -ErrorAction Continue
    exit 1
}
finally {
    $env:PGOPTIONS = $originalOptions
    $env:PGAPPNAME = $originalAppName
    $audit.finishedAt = (Get-Date).ToString('o')
    if ($null -ne $auditPath) { $audit | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $auditPath -Encoding UTF8 }
}
