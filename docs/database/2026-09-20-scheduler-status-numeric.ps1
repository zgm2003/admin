<#
.SYNOPSIS
    Converts Scheduler Job/Run status strings to numeric enum values.
#>
[CmdletBinding()]
param([switch]$OldAPIStopped)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not $OldAPIStopped) { throw '拒绝执行：请先停止 API/Worker，并显式传入 -OldAPIStopped。' }

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$envPath = Join-Path $repoRoot 'server\.env'
$sqlPath = Join-Path $PSScriptRoot '2026-09-20-scheduler-status-numeric.sql'

function Read-Env([string]$Path) {
    $result = @{}
    if (-not (Test-Path -LiteralPath $Path)) { return $result }
    foreach ($raw in Get-Content -LiteralPath $Path -Encoding utf8) {
        $line = $raw.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        $at = $line.IndexOf('=')
        if ($at -lt 1) { continue }
        $result[$line.Substring(0, $at).Trim()] = $line.Substring($at + 1).Trim().Trim('"').Trim("'")
    }
    return $result
}
function Setting([hashtable]$Values, [string]$Key) {
    $value = [Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    if ($Values.ContainsKey($Key)) { return [string]$Values[$Key] }
    return ''
}
function Query([string]$Dsn, [string]$Sql) {
    $output = @(& psql -X -A -t -v ON_ERROR_STOP=1 -d $Dsn -c $Sql)
    if ($LASTEXITCODE -ne 0) { throw "psql 查询失败（exit $LASTEXITCODE）。" }
    return (($output | Where-Object { $_ -ne '' }) -join "`n").Trim()
}

$values = Read-Env $envPath
$dsn = Setting $values 'POSTGRES_DSN'
if ([string]::IsNullOrWhiteSpace($dsn)) { throw '缺少 POSTGRES_DSN。' }
$dsn = $dsn -replace '(?i)(^|\s+)TimeZone=\S+', '$1'
if (-not (Test-Path -LiteralPath $sqlPath)) { throw "找不到 migration SQL：$sqlPath" }

$before = Query $dsn "SELECT table_name || ':' || data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('system_scheduler_job','system_scheduler_run') AND column_name='status' ORDER BY table_name;"
if ($before -notmatch 'system_scheduler_job:(character varying|smallint)' -or $before -notmatch 'system_scheduler_run:(character varying|smallint)') {
    throw "拒绝执行：Scheduler status 列不存在或类型不受支持：$before"
}

$backupDir = Join-Path $env:LOCALAPPDATA ('Admin\backups\scheduler-status-numeric-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
$dump = Join-Path $backupDir 'public-before.dump'
& pg_dump --format=custom --schema=public --no-owner --no-privileges --dbname=$dsn --file=$dump
if ($LASTEXITCODE -ne 0) { throw "pg_dump 备份失败（exit $LASTEXITCODE）。" }
& pg_restore --list $dump | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'pg_restore --list 无法读取备份。' }

& psql -X -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath
if ($LASTEXITCODE -ne 0) { throw 'migration SQL 失败；事务已回滚。' }

$facts = Query $dsn "SELECT (SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='system_scheduler_job' AND column_name='status') || ':' || (SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='system_scheduler_run' AND column_name='status') || ':' || (SELECT count(*) FROM system_scheduler_job WHERE status NOT BETWEEN 1 AND 6) || ':' || (SELECT count(*) FROM system_scheduler_run WHERE status NOT BETWEEN 1 AND 3);"
if ($facts -ne 'smallint:smallint:0:0') { throw "验证失败：$facts" }

& psql -X -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath | Out-Null
if ($LASTEXITCODE -ne 0) { throw '重复执行 migration SQL 失败。' }
$repeatFacts = Query $dsn "SELECT (SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='system_scheduler_job' AND column_name='status') || ':' || (SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='system_scheduler_run' AND column_name='status') || ':' || (SELECT count(*) FROM system_scheduler_job WHERE status NOT BETWEEN 1 AND 6) || ':' || (SELECT count(*) FROM system_scheduler_run WHERE status NOT BETWEEN 1 AND 3);"
if ($repeatFacts -ne $facts) { throw "幂等验证失败：$repeatFacts" }

Write-Host "完成：Scheduler Job/Run 状态数值化、备份、只读验证和重复执行通过。事实：$facts"
