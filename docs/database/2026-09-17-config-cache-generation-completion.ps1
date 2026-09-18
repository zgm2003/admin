<#
.SYNOPSIS
    Dictionary/Mail/SMS 配置缓存代际完成迁移的唯一入口。

.DESCRIPTION
    默认模式（-OldAPIStopped）：
      1. 备份 public schema 为 custom-format 归档，打印大小、SHA256，并用 pg_restore --list 验证；
      2. 记录目标 scope/outbox、策略事实、revision 列、权限版本、Session/access 版本；
      3. 通过 psql -X -v ON_ERROR_STOP=1 执行同名 SQL；
      4. 执行固定 forward Redis cleanup 两次；
      5. 只读验证最终结构、代际事实与全部受保护事实未被误改。

    -OldAPIStopped -CleanupOnly：跳过备份与 SQL，只重试固定 forward cleanup。
    -OldAPIStopped -RollbackCleanupOnly：不连接 PostgreSQL，只删除三个新 scope 的 state/snapshot。

    三种模式都必须显式传入 -OldAPIStopped。命令不打印 POSTGRES_DSN、REDIS_URL 或缓存内容。
#>
[CmdletBinding()]
param(
    [switch]$OldAPIStopped,
    [switch]$CleanupOnly,
    [switch]$RollbackCleanupOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir = Join-Path $repoRoot 'server'
$sqlPath = Join-Path $PSScriptRoot '2026-09-17-config-cache-generation-completion.sql'
$envFile = Join-Path $serverDir '.env'
$cleanupFailureNotice = 'PostgreSQL 已提交、Redis cleanup 未完成；可安全重复执行本 runner'

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
    $value = [System.Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    if ($FileValues.ContainsKey($Key)) { return [string]$FileValues[$Key] }
    return ''
}

$sqlCommitted = $false

try {
    if (-not $OldAPIStopped) {
        throw '拒绝执行：请先停止旧 API/Worker，并显式传入 -OldAPIStopped（cleanup 模式同样必须确认）。'
    }
    if ($CleanupOnly -and $RollbackCleanupOnly) {
        throw '拒绝执行：-CleanupOnly 与 -RollbackCleanupOnly 互斥。'
    }

    $fileValues = Read-EnvFile -Path $envFile
    $redisUrl = Get-EffectiveSetting -FileValues $fileValues -Key 'REDIS_URL'
    if ([string]::IsNullOrWhiteSpace($redisUrl)) { throw '缺少 REDIS_URL（server/.env 或进程环境）。' }
    if ($redisUrl -notmatch '^rediss?://') { throw 'REDIS_URL 必须以 redis:// 或 rediss:// 开头。' }
    $env:REDIS_URL = $redisUrl

    function Invoke-Cleanup {
        param([string]$Mode, [string]$Phase)
        & go run ./cmd/config-cache-generation-completion-migration -mode $Mode
        if ($LASTEXITCODE -ne 0) {
            throw "$Phase cleanup（mode=$Mode）失败（exit $LASTEXITCODE）。$cleanupFailureNotice"
        }
    }

    if ($RollbackCleanupOnly) {
        Write-Host 'RollbackCleanupOnly：不连接 PostgreSQL，只清除三个新 scope 的 state/snapshot。'
        Push-Location $serverDir
        try {
            Invoke-Cleanup -Mode 'rollback' -Phase '回滚'
            Invoke-Cleanup -Mode 'rollback' -Phase '回滚复验'
        } finally {
            Pop-Location
        }
        Write-Host '完成：rollback cleanup 已执行两次，所有固定 pattern remaining=0。'
        exit 0
    }

    $postgresDsn = Get-EffectiveSetting -FileValues $fileValues -Key 'POSTGRES_DSN'
    if ([string]::IsNullOrWhiteSpace($postgresDsn)) { throw '缺少 POSTGRES_DSN（server/.env 或进程环境）。' }
    $psqlDsn = $postgresDsn -replace '\s+TimeZone=\S+', ''
    $env:POSTGRES_DSN = $postgresDsn
    $env:PGCLIENTENCODING = 'UTF8'

    function Get-PsqlValue {
        param([string]$Query)
        $output = @(& psql -X -A -t -d $psqlDsn -v ON_ERROR_STOP=1 -c $Query)
        if ($LASTEXITCODE -ne 0) { throw "psql 查询失败（exit $LASTEXITCODE）。" }
        $lines = @($output | Where-Object { $_ -ne '' })
        if ($lines.Count -eq 0) { return '' }
        return $lines[-1].Trim()
    }

    function Assert-PsqlValue {
        param([string]$Label, [string]$Query, [string]$Expected)
        $value = Get-PsqlValue -Query $Query
        if ($value -ne $Expected) { throw "验证失败：$Label = $value（期望 $Expected）。" }
        Write-Host "验证通过：$Label = $value"
    }

    $policyFingerprintQuery = "SELECT md5(concat(
        COALESCE((SELECT string_agg(concat_ws('|',platform_id,policy_key,mode,dimension,limit_count,window_seconds,created_at,updated_at),';' ORDER BY platform_id,policy_key) FROM message_mail_rate_limit_policy),''), '#',
        COALESCE((SELECT string_agg(concat_ws('|',platform_id,policy_key,mode,dimension,limit_count,window_seconds,created_at,updated_at),';' ORDER BY platform_id,policy_key) FROM message_sms_rate_limit_policy),'')))"
    $securityFingerprintQuery = "SELECT md5(concat(
        COALESCE((SELECT string_agg(id || ':' || menu_version || ':' || policy_version,',' ORDER BY id) FROM permission_auth_platform),''), '#',
        COALESCE((SELECT string_agg(user_id || ':' || version,',' ORDER BY user_id) FROM permission_access_version),''), '#',
        COALESCE((SELECT string_agg(id || ':' || version,',' ORDER BY id) FROM user_session),'')))"
    $generationFactsQuery = "SELECT COALESCE(string_agg(namespace || '/' || scope_key || ':' || generation,',' ORDER BY namespace,scope_key),'') FROM system_config_cache_generation WHERE scope_key='global' AND namespace IN ('system.dictionary','message.mail','message.sms')"
    $outboxFactsQuery = "SELECT COALESCE(string_agg(namespace || '/' || scope_key || ':' || generation || ':' || CASE WHEN published_at IS NULL THEN 'pending' ELSE 'published' END,',' ORDER BY id),'') FROM system_config_cache_outbox WHERE scope_key='global' AND namespace IN ('system.dictionary','message.mail','message.sms')"

    $before = [ordered]@{
        generationFacts = Get-PsqlValue -Query $generationFactsQuery
        outboxFacts     = Get-PsqlValue -Query $outboxFactsQuery
        completeScopes  = Get-PsqlValue -Query "SELECT CASE WHEN
            (SELECT count(*) FROM system_config_cache_generation WHERE scope_key='global' AND namespace IN ('system.dictionary','message.mail','message.sms'))=3
            AND NOT EXISTS (SELECT 1 FROM system_config_cache_generation AS generation
                WHERE generation.scope_key='global' AND generation.namespace IN ('system.dictionary','message.mail','message.sms')
                  AND NOT EXISTS (SELECT 1 FROM system_config_cache_outbox AS outbox
                    WHERE outbox.namespace=generation.namespace AND outbox.scope_key=generation.scope_key AND outbox.generation=generation.generation))
            THEN 'yes' ELSE 'no' END"
        mailRevision   = Get-PsqlValue -Query "SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_mail_rate_limit_policy' AND column_name='revision'"
        smsRevision    = Get-PsqlValue -Query "SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_rate_limit_policy' AND column_name='revision'"
        policyFacts    = Get-PsqlValue -Query $policyFingerprintQuery
        securityFacts  = Get-PsqlValue -Query $securityFingerprintQuery
    }
    Write-Host ("迁移前摘要：scopes={0}；outbox={1}；complete={2}；Mail revision={3}；SMS revision={4}；policy SHA={5}；security SHA={6}" -f `
        $before.generationFacts, $before.outboxFacts, $before.completeScopes, $before.mailRevision, $before.smsRevision, $before.policyFacts, $before.securityFacts)

    if (-not $CleanupOnly) {
        $backupDir = Join-Path $env:LOCALAPPDATA ("Admin\backups\config-cache-generation-completion-" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
        New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
        $dumpPath = Join-Path $backupDir 'public-before.dump'
        Write-Host "备份 public schema 到 $dumpPath"
        & pg_dump --format=custom --schema=public --no-owner --no-privileges --dbname=$psqlDsn --file=$dumpPath
        if ($LASTEXITCODE -ne 0) { throw "pg_dump 备份失败（exit $LASTEXITCODE）。" }
        $dumpHash = (Get-FileHash -LiteralPath $dumpPath -Algorithm SHA256).Hash
        $dumpSize = (Get-Item -LiteralPath $dumpPath).Length
        $entryCount = @(& pg_restore --list $dumpPath).Count
        if ($LASTEXITCODE -ne 0 -or $entryCount -lt 1) { throw 'pg_restore --list 无法读取备份归档。' }
        Write-Host "备份完成：size=$dumpSize SHA256=$dumpHash 归档条目=$entryCount"

        Write-Host "执行 forward SQL：$sqlPath"
        & psql -X -d $psqlDsn -v ON_ERROR_STOP=1 -f $sqlPath
        if ($LASTEXITCODE -ne 0) {
            throw "migration SQL 失败（exit $LASTEXITCODE）：事务已整体回滚，Redis cleanup 未运行；修正原因后重跑本 runner。"
        }
        $sqlCommitted = $true
        Write-Host 'migration SQL 已提交。'
    } else {
        Write-Host 'CleanupOnly：跳过备份与 SQL，只执行固定 forward Redis cleanup。'
    }

    Push-Location $serverDir
    try {
        Invoke-Cleanup -Mode 'forward' -Phase '首次'
        Invoke-Cleanup -Mode 'forward' -Phase '复验'
        Write-Host 'forward Redis cleanup 已执行两次，所有固定 pattern remaining=0。'
    } finally {
        Pop-Location
    }

    if (-not $CleanupOnly) {
        Assert-PsqlValue '三个固定 scope 与当前代 outbox' "SELECT CASE WHEN
            (SELECT count(*) FROM system_config_cache_generation WHERE scope_key='global' AND namespace IN ('system.dictionary','message.mail','message.sms'))=3
            AND NOT EXISTS (SELECT 1 FROM system_config_cache_generation AS generation
                WHERE generation.scope_key='global' AND generation.namespace IN ('system.dictionary','message.mail','message.sms')
                  AND (generation.generation < 1 OR NOT EXISTS (SELECT 1 FROM system_config_cache_outbox AS outbox
                    WHERE outbox.namespace=generation.namespace AND outbox.scope_key=generation.scope_key AND outbox.generation=generation.generation)))
            THEN 'ok' ELSE 'fail' END" 'ok'
        Assert-PsqlValue 'revision 列与索引已删除' "SELECT CASE WHEN
            NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('message_mail_rate_limit_policy','message_sms_rate_limit_policy') AND column_name='revision')
            AND to_regclass('ix_message_mail_rate_limit_policy_platform_revision') IS NULL
            AND to_regclass('ix_message_sms_rate_limit_policy_platform_revision') IS NULL
            THEN 'ok' ELSE 'fail' END" 'ok'
        Assert-PsqlValue 'Mail/SMS policy 事实与时间戳不变' $policyFingerprintQuery "$($before.policyFacts)"
        Assert-PsqlValue 'menu/policy/session/access 版本不变' $securityFingerprintQuery "$($before.securityFacts)"

        if ($before.completeScopes -eq 'yes') {
            Assert-PsqlValue '幂等复跑 generation 不变' $generationFactsQuery "$($before.generationFacts)"
            Assert-PsqlValue '幂等复跑 outbox 不变' $outboxFactsQuery "$($before.outboxFacts)"
        }
        Write-Host '完成：SQL、Redis cleanup 与只读验证全部通过。'
    } else {
        Write-Host '完成：CleanupOnly 已执行，未修改 PostgreSQL。'
    }
    exit 0
} catch {
    [Console]::Error.WriteLine("迁移未完成：{0}", $_.Exception.Message)
    if ($sqlCommitted) {
        [Console]::Error.WriteLine($cleanupFailureNotice)
    }
    exit 1
}
