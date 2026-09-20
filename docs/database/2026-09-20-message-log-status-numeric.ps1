<#[.SYNOPSIS
    Converts Mail/SMS log status columns from strings to numeric enum values.
#>
[CmdletBinding()]
param([switch]$OldAPIStopped)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not $OldAPIStopped) { throw '拒绝执行：请先停止 API/Worker，并显式传入 -OldAPIStopped。' }

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$envPath = Join-Path $repoRoot 'server\.env'
$sqlPath = Join-Path $PSScriptRoot '2026-09-20-message-log-status-numeric.sql'

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

$before = Query $dsn "SELECT table_name || ':' || data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('message_mail_log','message_sms_log') AND column_name='status' ORDER BY table_name;"
if ($before -notmatch 'message_mail_log:character varying|message_mail_log:smallint' -or $before -notmatch 'message_sms_log:character varying|message_sms_log:smallint') {
    throw "拒绝执行：目标 status 列不存在或类型不受支持。"
}

$backupDir = Join-Path $env:LOCALAPPDATA ('Admin\backups\message-log-status-numeric-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
$dump = Join-Path $backupDir 'public-before.dump'
& pg_dump --format=custom --schema=public --no-owner --no-privileges --dbname=$dsn --file=$dump
if ($LASTEXITCODE -ne 0) { throw "pg_dump 备份失败（exit $LASTEXITCODE）。" }
& pg_restore --list $dump | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'pg_restore --list 无法读取备份。' }

& psql -X -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath
if ($LASTEXITCODE -ne 0) { throw 'migration SQL 失败；事务已回滚。' }

$types = Query $dsn "SELECT table_name || ':' || data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('message_mail_log','message_sms_log') AND column_name='status' ORDER BY table_name;"
$distribution = Query $dsn "SELECT 'mail' || ':' || status::text || ':' || count(*) FROM message_mail_log GROUP BY status UNION ALL SELECT 'sms' || ':' || status::text || ':' || count(*) FROM message_sms_log GROUP BY status ORDER BY 1;"
$checks = Query $dsn "SELECT conrelid::regclass || ':' || conname FROM pg_constraint WHERE conrelid IN ('message_mail_log'::regclass,'message_sms_log'::regclass) AND conname IN ('ck_message_mail_log_scene','message_mail_log_status_check','ck_message_sms_log_status') ORDER BY 1;"
$invalid = Query $dsn "SELECT (SELECT count(*) FROM message_mail_log WHERE status NOT IN (1,2,3)) + (SELECT count(*) FROM message_sms_log WHERE status NOT IN (1,2,3));"
if ($types -ne "message_mail_log:smallint`nmessage_sms_log:smallint" -or $invalid -ne '0' -or $checks -ne "message_mail_log:ck_message_mail_log_scene`nmessage_mail_log:message_mail_log_status_check`nmessage_sms_log:ck_message_sms_log_status") {
    throw "验证失败：types='$types' invalid=$invalid checks='$checks' distribution='$distribution'。"
}

& psql -X -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath | Out-Null
if ($LASTEXITCODE -ne 0) { throw '重复执行 migration SQL 失败。' }
$repeatTypes = Query $dsn "SELECT table_name || ':' || data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('message_mail_log','message_sms_log') AND column_name='status' ORDER BY table_name;"
$repeatInvalid = Query $dsn "SELECT (SELECT count(*) FROM message_mail_log WHERE status NOT IN (1,2,3)) + (SELECT count(*) FROM message_sms_log WHERE status NOT IN (1,2,3));"
if ($repeatTypes -ne $types -or $repeatInvalid -ne '0') { throw '幂等验证失败。' }

Write-Host "完成：Mail/SMS 状态数值化、备份、只读验证和重复执行通过。分布：$distribution"
