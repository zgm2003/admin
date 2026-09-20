<#
.SYNOPSIS
    通用调度器 forward migration 唯一入口。

.DESCRIPTION
    默认模式执行 PostgreSQL 迁移并进行两轮旧通知 Asynq task 定向清理。
    -CleanupOnly 跳过 SQL，只重试 Redis 清理。
    所有模式都要求 API/Worker 已停止并显式传入 -OldAPIStopped。
#>
[CmdletBinding()]
param(
    [switch]$OldAPIStopped,
    [switch]$CleanupOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir = Join-Path $repoRoot 'server'
$sqlPath = Join-Path $PSScriptRoot '2026-09-19-system-scheduler.sql'
$envFile = Join-Path $serverDir '.env'
$sqlCommitted = $false

function Read-EnvFile {
    param([string]$Path)
    $values = @{}
    if (-not (Test-Path -LiteralPath $Path)) { return $values }
    foreach ($rawLine in Get-Content -LiteralPath $Path -Encoding utf8) {
        $line = $rawLine.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        $index = $line.IndexOf('=')
        if ($index -lt 1) { continue }
        $values[$line.Substring(0, $index).Trim()] = $line.Substring($index + 1).Trim().Trim('"').Trim("'")
    }
    return $values
}

function Get-EffectiveSetting {
    param([hashtable]$FileValues, [string]$Key)
    $value = [Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    if ($FileValues.ContainsKey($Key)) { return [string]$FileValues[$Key] }
    return ''
}

function Assert-NoServiceProcess {
    $processes = Get-CimInstance Win32_Process | Where-Object {
        $_.CommandLine -match '(cmd[\\/]api|cmd[\\/]worker|\\bapi\.exe\\b|\\bworker\.exe\\b)'
    }
    if ($processes) {
        $names = ($processes | ForEach-Object { "$($_.Name):$($_.ProcessId)" }) -join ', '
        throw "检测到 API/Worker 进程：$names；请先停止服务。"
    }
}

function Invoke-Cleanup {
    param([string]$Phase)
    Push-Location $serverDir
    try {
        & go run ./cmd/system-scheduler-migration -mode cleanup
        if ($LASTEXITCODE -ne 0) { throw "$Phase Redis cleanup 失败（exit $LASTEXITCODE）。" }
    } finally { Pop-Location }
}

try {
    if (-not $OldAPIStopped) {
        throw '拒绝执行：请先停止 API/Worker，并显式传入 -OldAPIStopped。'
    }
    Assert-NoServiceProcess
    $fileValues = Read-EnvFile -Path $envFile
    $postgresDsn = Get-EffectiveSetting -FileValues $fileValues -Key 'POSTGRES_DSN'
    $redisUrl = Get-EffectiveSetting -FileValues $fileValues -Key 'REDIS_URL'
    if ([string]::IsNullOrWhiteSpace($postgresDsn)) { throw '缺少 POSTGRES_DSN（server/.env 或进程环境）。' }
    if ([string]::IsNullOrWhiteSpace($redisUrl) -or $redisUrl -notmatch '^rediss?://') { throw 'REDIS_URL 必须以 redis:// 或 rediss:// 开头。' }
    $env:POSTGRES_DSN = $postgresDsn
    $env:REDIS_URL = $redisUrl

    if (-not $CleanupOnly) {
        if (-not (Test-Path -LiteralPath $sqlPath)) { throw "找不到 migration SQL：$sqlPath" }
        $backupDir = Join-Path $env:LOCALAPPDATA ("Admin\backups\system-scheduler-" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
        New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
        $dumpPath = Join-Path $backupDir 'public-before.dump'
        & pg_dump --format=custom --schema=public --no-owner --no-privileges --dbname=$postgresDsn --file=$dumpPath
        if ($LASTEXITCODE -ne 0) { throw "pg_dump 备份失败（exit $LASTEXITCODE）。" }
        & pg_restore --list $dumpPath | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'pg_restore --list 无法读取备份归档。' }

        & psql -X -d $postgresDsn -v ON_ERROR_STOP=1 -f $sqlPath
        if ($LASTEXITCODE -ne 0) { throw "migration SQL 失败（exit $LASTEXITCODE）：事务已回滚，Redis cleanup 未执行。" }
        $sqlCommitted = $true
        Write-Host 'migration SQL 已提交。'
    }

    Invoke-Cleanup -Phase '首次'
    Invoke-Cleanup -Phase '复验'

    $tableCount = (& psql -X -At -d $postgresDsn -c "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relkind='r' AND c.relname IN ('system_scheduler_schedule','system_scheduler_job','system_scheduler_run');").Trim()
    if ($tableCount -ne '3') { throw "调度器目标表数量错误：$tableCount。" }
    $oldTableCount = (& psql -X -At -d $postgresDsn -c "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relname='message_notification_dispatch_outbox';").Trim()
    if ($oldTableCount -ne '0') { throw "旧 notification dispatch outbox 仍存在：$oldTableCount。" }
    $scheduleCount = (& psql -X -At -d $postgresDsn -c "SELECT count(*) FROM system_scheduler_schedule WHERE deleted_at IS NULL AND builtin_key IN ('realtime.retention.cleanup','message.notification.retention.cleanup','system.scheduler.history.cleanup');").Trim()
    if ($scheduleCount -ne '3') { throw "内置调度计划数量错误：$scheduleCount。" }
    $settingCount = (& psql -X -At -d $postgresDsn -c "SELECT count(*) FROM system_setting WHERE deleted_at IS NULL AND setting_key='system.scheduler.history_retention_days' AND value_type=2 AND is_enabled=1 AND is_builtin=1 AND value ~ '^[0-9]+$' AND value::BIGINT BETWEEN 7 AND 3650;").Trim()
    if ($settingCount -ne '1') { throw "调度器历史保留设置不合法：$settingCount。" }
    Write-Host '完成：调度器 SQL、两轮 Redis cleanup 和只读验证全部通过。'
    exit 0
} catch {
    [Console]::Error.WriteLine("迁移未完成：{0}", $_.Exception.Message)
    if ($sqlCommitted) { [Console]::Error.WriteLine('PostgreSQL 已提交；可安全重复执行本 runner。') }
    exit 1
}
