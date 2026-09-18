<#
.SYNOPSIS
    COS 配置版本化 + 上传规则互斥启用 + v2 对象 key 迁移的唯一入口。

.DESCRIPTION
    默认模式（-OldAPIStopped）：
      1. 自动 pg_dump 备份 public schema（custom 格式）、打印 SHA256 并用 pg_restore --list 验证可读；
      2. 记录迁移前摘要（行数、非空头像、品牌默认头像是否非空、menu_version、generation/outbox）；
      3. psql -v ON_ERROR_STOP=1 执行同名 SQL；
      4. 在 server 目录执行固定 pattern 的 forward Redis cleanup 两次（第二次验证 remaining=0）；
      5. 只读验证结构、回填、旧引用归零、每配置 generation/outbox 与 menu_version 不变。

    -OldAPIStopped -CleanupOnly：跳过备份与 SQL，仅重试 forward cleanup 与计数。
    -OldAPIStopped -RollbackCleanupOnly：仅执行 rollback cleanup（清除新二进制写入的
    storage.cosconfig 缓存与 v2 route）；该模式不连接、不修改 PostgreSQL。

    三种模式都必须显式传入 -OldAPIStopped。SQL 幂等：重复执行不新增物理版本、不推进 generation。
    不打印 POSTGRES_DSN / REDIS_URL 值。
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
$sqlPath = Join-Path $PSScriptRoot '2026-09-17-storage-object-versioning.sql'
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
        throw '拒绝执行：请先确认旧 API/Worker 已停止，并显式传入 -OldAPIStopped（CleanupOnly / RollbackCleanupOnly 同样必须确认）。'
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
        & go run ./cmd/storage-object-versioning-migration -mode $Mode
        if ($LASTEXITCODE -ne 0) {
            throw "$Phase cleanup（mode=$Mode）失败（exit $LASTEXITCODE）。$cleanupFailureNotice"
        }
    }

    if ($RollbackCleanupOnly) {
        Write-Host 'RollbackCleanupOnly：不连接 PostgreSQL，仅清除新二进制缓存。'
        Push-Location $serverDir
        try {
            Invoke-Cleanup -Mode 'rollback' -Phase '回滚'
            Invoke-Cleanup -Mode 'rollback' -Phase '回滚复验'
        } finally {
            Pop-Location
        }
        Write-Host '完成：rollback cleanup 已执行，remaining=0。'
        exit 0
    }

    $postgresDsn = Get-EffectiveSetting -FileValues $fileValues -Key 'POSTGRES_DSN'
    if ([string]::IsNullOrWhiteSpace($postgresDsn)) { throw '缺少 POSTGRES_DSN（server/.env 或进程环境）。' }
    # psql/libpq 不接受 pgx 专有的 TimeZone 连接参数；沿用项目既有 psql 执行方式剥离。
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

    $before = [ordered]@{
        menuVersions         = Get-PsqlValue -Query "SELECT COALESCE(string_agg(id || ':' || menu_version, ',' ORDER BY id), '') FROM permission_auth_platform"
        brandNonEmpty        = Get-PsqlValue -Query "SELECT CASE WHEN value <> '' THEN '1' ELSE '0' END FROM system_setting WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL"
        systemGeneration     = Get-PsqlValue -Query "SELECT generation FROM system_config_cache_generation WHERE namespace = 'system.setting' AND scope_key = 'global'"
        storageGeneration    = Get-PsqlValue -Query "SELECT COALESCE(string_agg(scope_key || ':' || generation, ',' ORDER BY scope_key), '') FROM system_config_cache_generation WHERE namespace = 'storage.cosconfig'"
        outboxRows           = Get-PsqlValue -Query "SELECT count(*) FROM system_config_cache_outbox WHERE namespace IN ('storage.cosconfig', 'system.setting')"
        nonEmptyAvatars      = Get-PsqlValue -Query "SELECT count(*) FROM user_profile WHERE avatar <> ''"
        ruleCodeRows         = Get-PsqlValue -Query "SELECT count(*) FROM storage_upload_rule_code"
    }
    Write-Host ("迁移前摘要：menu_version={0}；品牌默认头像非空={1}；system.setting generation={2}；storage generation={3}；outbox={4}；非空头像={5}；编码行={6}" -f `
        $before.menuVersions, $before.brandNonEmpty, $before.systemGeneration, $before.storageGeneration, $before.outboxRows, $before.nonEmptyAvatars, $before.ruleCodeRows)

    if (-not $CleanupOnly) {
        $backupDir = Join-Path $env:LOCALAPPDATA ("Admin\backups\storage-object-versioning-" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
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
        Write-Host 'CleanupOnly：跳过备份与 migration SQL，仅执行 forward Redis cleanup 与计数。'
    }

    Push-Location $serverDir
    try {
        Invoke-Cleanup -Mode 'forward' -Phase '首次'
        Invoke-Cleanup -Mode 'forward' -Phase '复验'
        Write-Host 'forward Redis cleanup 复验完成：legacy pattern remaining=0。'
    } finally {
        Pop-Location
    }

    if (-not $CleanupOnly) {
        Assert-PsqlValue '结构与索引' "SELECT CASE WHEN
            to_regclass('storage_cos_config_version') IS NOT NULL
            AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'storage_cos_config' AND column_name = 'current_version')
            AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'storage_upload_rule' AND column_name = 'revision')
            AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'storage_cos_config' AND column_name IN ('bucket','region','endpoint','bucket_domain'))
            AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'storage_upload_rule_code' AND column_name = 'updated_at')
            AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'storage_upload_rule_code' AND column_name = 'platform_id')
            AND EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'ux_storage_upload_rule_platform_enabled')
            AND EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'ux_storage_upload_rule_code_rule_code')
            AND to_regclass('storage_object_legacy_route') IS NULL
        THEN 'ok' ELSE 'fail' END" 'ok'

        Assert-PsqlValue '物理版本回填' "SELECT CASE WHEN
            (SELECT count(*) FROM storage_cos_config AS config WHERE NOT EXISTS (
                SELECT 1 FROM storage_cos_config_version AS version
                WHERE version.cos_config_id = config.id AND version.version = config.current_version)) = 0
        THEN 'ok' ELSE 'fail' END" 'ok'

        Assert-PsqlValue '旧对象引用已清空' "SELECT CASE WHEN
            (SELECT count(*) FROM user_profile WHERE avatar <> '') = 0
            AND (SELECT count(*) FROM system_setting WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL AND value <> '') = 0
        THEN 'ok' ELSE 'fail' END" 'ok'

        Assert-PsqlValue '每配置 generation 与同代 outbox' "SELECT CASE WHEN
            (SELECT count(*) FROM storage_cos_config AS config WHERE NOT EXISTS (
                SELECT 1 FROM system_config_cache_generation AS generation
                WHERE generation.namespace = 'storage.cosconfig' AND generation.scope_key = config.id::text)) = 0
            AND (SELECT count(*) FROM system_config_cache_generation AS generation
                 WHERE generation.namespace = 'storage.cosconfig' AND NOT EXISTS (
                     SELECT 1 FROM system_config_cache_outbox AS outbox
                     WHERE outbox.namespace = generation.namespace AND outbox.scope_key = generation.scope_key
                       AND outbox.generation = generation.generation)) = 0
        THEN 'ok' ELSE 'fail' END" 'ok'

        $expectedSystemGeneration = [int64]$before.systemGeneration
        if ($before.brandNonEmpty -eq '1') { $expectedSystemGeneration = $expectedSystemGeneration + 1 }
        Assert-PsqlValue 'system.setting generation' "SELECT generation FROM system_config_cache_generation WHERE namespace = 'system.setting' AND scope_key = 'global'" "$expectedSystemGeneration"
        Assert-PsqlValue 'system.setting 当前代 outbox=1' "SELECT count(*) FROM system_config_cache_outbox WHERE namespace = 'system.setting' AND scope_key = 'global' AND generation = $expectedSystemGeneration" '1'
        Assert-PsqlValue 'menu_version 全部不变' "SELECT COALESCE(string_agg(id || ':' || menu_version, ',' ORDER BY id), '') FROM permission_auth_platform" "$($before.menuVersions)"

        if ($before.storageGeneration -ne '') {
            Assert-PsqlValue '重跑后 storage generation 不变' "SELECT COALESCE(string_agg(scope_key || ':' || generation, ',' ORDER BY scope_key), '') FROM system_config_cache_generation WHERE namespace = 'storage.cosconfig'" "$($before.storageGeneration)"
            Assert-PsqlValue '重跑后 outbox 行数不变' "SELECT count(*) FROM system_config_cache_outbox WHERE namespace IN ('storage.cosconfig', 'system.setting')" "$($before.outboxRows)"
            Assert-PsqlValue '重跑后编码行数不变' "SELECT count(*) FROM storage_upload_rule_code" "$($before.ruleCodeRows)"
        }

        $phase = '首次迁移'
        if ($before.storageGeneration -ne '') { $phase = '幂等复跑' }
        Write-Host "完成：SQL、forward Redis cleanup 与只读验证全部通过（阶段=$phase）。"
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
