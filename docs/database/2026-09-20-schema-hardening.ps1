<#[.SYNOPSIS
    Schema hygiene migration for duplicate session FK, empty device ids and access-version identity default.
#>
[CmdletBinding()]
param([switch]$OldAPIStopped)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not $OldAPIStopped) { throw '拒绝执行：请先停止 API/Worker，并显式传入 -OldAPIStopped。' }

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir = Join-Path $repoRoot 'server'
$envPath = Join-Path $serverDir '.env'
$sqlPath = Join-Path $PSScriptRoot '2026-09-20-schema-hardening.sql'

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

$before = Query $dsn "SELECT count(*) FROM user_session WHERE btrim(device_id) = '';"
if ($before -notin @('0', '6')) {
    throw "拒绝执行：预期空 device_id 数量为 6（首次）或 0（已迁移），实际为 $before。请先人工复核数据。"
}

$backupDir = Join-Path $env:LOCALAPPDATA ('Admin\backups\schema-hardening-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
$dump = Join-Path $backupDir 'public-before.dump'
& pg_dump --format=custom --schema=public --no-owner --no-privileges --dbname=$dsn --file=$dump
if ($LASTEXITCODE -ne 0) { throw "pg_dump 备份失败（exit $LASTEXITCODE）。" }
& pg_restore --list $dump | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'pg_restore --list 无法读取备份。' }

& psql -X -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath
if ($LASTEXITCODE -ne 0) { throw 'migration SQL 失败；事务已回滚。' }

$duplicateFk = Query $dsn "SELECT count(*) FROM pg_constraint WHERE conrelid='user_session'::regclass AND contype='f' AND conname='fk_auth_session_user';"
$emptyDevices = Query $dsn "SELECT count(*) FROM user_session WHERE btrim(device_id) = '';"
$default = Query $dsn "SELECT COALESCE(column_default,'') FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='permission_access_version' AND column_name='user_id';"
$sequence = Query $dsn "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relname='rbac_access_version_user_id_seq';"
$check = Query $dsn "SELECT count(*) FROM pg_constraint WHERE conrelid='user_session'::regclass AND conname='ck_user_session_device_id_nonempty';"
if ($duplicateFk -ne '0' -or $emptyDevices -ne '0' -or $default -ne '' -or $sequence -ne '0' -or $check -ne '1') {
    throw "验证失败：duplicateFk=$duplicateFk emptyDevices=$emptyDevices default='$default' sequence=$sequence check=$check。"
}

& psql -X -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath | Out-Null
if ($LASTEXITCODE -ne 0) { throw '重复执行 migration SQL 失败。' }
Write-Host '完成：结构清理、备份、重复执行和只读验证通过。'
