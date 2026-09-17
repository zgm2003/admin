<#
.SYNOPSIS
    统一配置缓存代际协议第一阶段迁移的唯一入口：forward SQL + 定向 Redis cleanup + 只读验证。

.DESCRIPTION
    默认模式（-OldAPIStopped）：执行
    docs/database/2026-09-16-system-config-cache-generation.sql，随后在 server 目录运行
    go run ./cmd/system-setting-cache-migration 清理 legacy 键 system:setting:v1:*，
    最后做只读 SQL 验证与 legacy key=0 复验。

    -OldAPIStopped -CleanupOnly：跳过 SQL，仅执行固定 legacy key cleanup 与 key-count
    验证，供旧应用回滚前使用。

    两种模式都必须显式传入 -OldAPIStopped，确认旧 API/Worker 写流量已摘除。
    SQL 幂等：重复执行不推进 generation/menu_version；cleanup 幂等：重复执行只重试缺失步骤。
    不打印 POSTGRES_DSN / REDIS_URL 值。
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
$sqlPath = Join-Path $PSScriptRoot '2026-09-16-system-config-cache-generation.sql'
$envFile = Join-Path $serverDir '.env'

function Read-EnvFile {
    param([string]$Path)
    $values = @{}
    if (-not (Test-Path -LiteralPath $Path)) { return $values }
    foreach ($rawLine in Get-Content -LiteralPath $Path) {
        $line = $rawLine.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        $index = $line.IndexOf('=')
        if ($index -lt 1) { continue }
        $key = $line.Substring(0, $index).Trim()
        $values[$key] = $line.Substring($index + 1).Trim().Trim('"').Trim("'")
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

try {
    if (-not $OldAPIStopped) {
        throw '拒绝执行：请先确认旧 API/Worker 写流量已摘除，并显式传入 -OldAPIStopped（-CleanupOnly 同样必须确认）。'
    }

    $fileValues = Read-EnvFile -Path $envFile
    $postgresDsn = Get-EffectiveSetting -FileValues $fileValues -Key 'POSTGRES_DSN'
    $redisUrl = Get-EffectiveSetting -FileValues $fileValues -Key 'REDIS_URL'
    if ([string]::IsNullOrWhiteSpace($postgresDsn)) { throw '缺少 POSTGRES_DSN（server/.env 或进程环境）。' }
    if ([string]::IsNullOrWhiteSpace($redisUrl)) { throw '缺少 REDIS_URL（server/.env 或进程环境）。' }
    if ($redisUrl -notmatch '^rediss?://') { throw 'REDIS_URL 必须以 redis:// 或 rediss:// 开头。' }

    # psql/libpq 不接受 pgx 专有的 TimeZone 连接参数；沿用项目既有 psql 执行方式剥离。
    $psqlDsn = $postgresDsn -replace '\s+TimeZone=\S+', ''
    $env:POSTGRES_DSN = $postgresDsn
    $env:REDIS_URL = $redisUrl
    $env:PGCLIENTENCODING = 'UTF8'

    function Get-PsqlValue {
        param([string]$Query)
        $output = @(& psql -X -A -t -d $psqlDsn -v ON_ERROR_STOP=1 -c $Query)
        if ($LASTEXITCODE -ne 0) { throw "psql 只读验证失败（exit $LASTEXITCODE）。" }
        $lines = @($output | Where-Object { $_ -ne '' })
        if ($lines.Count -eq 0) { return '' }
        return $lines[-1].Trim()
    }

    function Invoke-RedisCleanup {
        param([string]$Phase)
        & go run ./cmd/system-setting-cache-migration
        if ($LASTEXITCODE -ne 0) {
            throw "$Phase Redis cleanup 失败（exit $LASTEXITCODE）。PostgreSQL 已提交，Redis cleanup 未完成；可安全重复执行本 runner。"
        }
    }

    $beforeVersion = ''
    $beforeCacheState = ''
    if (-not $CleanupOnly) {
        $beforeVersion = Get-PsqlValue -Query "SELECT menu_version FROM public.permission_auth_platform WHERE code='admin' AND deleted_at IS NULL AND is_enabled=1"
        $cacheTables = Get-PsqlValue -Query "SELECT CASE WHEN to_regclass('public.system_config_cache_generation') IS NOT NULL AND to_regclass('public.system_config_cache_outbox') IS NOT NULL THEN 'yes' ELSE 'no' END"
        if ($cacheTables -eq 'yes') {
            $beforeCacheState = Get-PsqlValue -Query "SELECT concat(g.generation, '|', (SELECT count(*) FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global'), '|', (SELECT count(*) FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global' AND published_at IS NULL), '|', (SELECT count(*) FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global' AND published_at IS NOT NULL), '|', coalesce((SELECT max(generation) FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global'), 0)) FROM public.system_config_cache_generation AS g WHERE g.namespace='system.setting' AND g.scope_key='global'"
        }
        Write-Host "migration 前 Admin menu_version=$beforeVersion"
        Write-Host "执行 forward SQL：$sqlPath"
        & psql -X -d $psqlDsn -v ON_ERROR_STOP=1 -f $sqlPath
        if ($LASTEXITCODE -ne 0) {
            throw "migration SQL 失败（exit $LASTEXITCODE）：事务已整体回滚，Redis cleanup 未运行；修正原因后重跑本 runner。"
        }
        Write-Host 'migration SQL 已提交。'
    } else {
        Write-Host '-CleanupOnly：跳过 migration SQL，仅执行 legacy Redis cleanup 与验证。'
    }

    Push-Location $serverDir
    try {
        Invoke-RedisCleanup -Phase '首次'
        # 复跑一次作为 key-count 复验：legacy key 必须为 0，且不产生其他副作用。
        Invoke-RedisCleanup -Phase '复验'
        Write-Host 'legacy Redis key 复验完成：system:setting:v1:* remaining=0。'
    } finally {
        Pop-Location
    }

    if (-not $CleanupOnly) {
        $cacheState = Get-PsqlValue -Query "SELECT CASE WHEN (to_regclass('public.system_config_cache_generation') IS NOT NULL) AND (to_regclass('public.system_config_cache_outbox') IS NOT NULL) AND ((SELECT generation FROM public.system_config_cache_generation WHERE namespace='system.setting' AND scope_key='global') >= 1) AND EXISTS (SELECT 1 FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global' AND generation=(SELECT generation FROM public.system_config_cache_generation WHERE namespace='system.setting' AND scope_key='global')) THEN 'ok' ELSE 'fail' END"
        if ($cacheState -ne 'ok') { throw "验证失败：generation/outbox 当前状态为 $cacheState。" }
        $afterCacheState = Get-PsqlValue -Query "SELECT concat(g.generation, '|', (SELECT count(*) FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global'), '|', (SELECT count(*) FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global' AND published_at IS NULL), '|', (SELECT count(*) FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global' AND published_at IS NOT NULL), '|', coalesce((SELECT max(generation) FROM public.system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global'), 0)) FROM public.system_config_cache_generation AS g WHERE g.namespace='system.setting' AND g.scope_key='global'"
        if ($beforeCacheState -ne '' -and $afterCacheState -ne $beforeCacheState) {
            throw "generation/outbox 幂等校验失败：$beforeCacheState -> $afterCacheState。"
        }

        $menuState = Get-PsqlValue -Query "SELECT CASE WHEN ((SELECT count(*) FROM public.permission_menu WHERE platform_id=(SELECT id FROM public.permission_auth_platform WHERE code='admin' AND deleted_at IS NULL AND is_enabled=1) AND code='system:cacheGeneration:view' AND menu_type='page' AND i18n_key='navigation.systemCacheGeneration' AND path='/system/cacheGeneration' AND component_path='system/cacheGeneration' AND icon='lucide:database-zap' AND sort_order=91 AND is_enabled=1 AND is_hidden=0 AND deleted_at IS NULL) = 1) AND ((SELECT count(*) FROM public.permission_menu AS action JOIN public.permission_menu AS page ON page.id=action.parent_id WHERE action.code='system:cacheGeneration:list' AND action.menu_type='action' AND action.is_hidden=1 AND action.is_enabled=1 AND action.i18n_key IS NULL AND action.path IS NULL AND action.component_path IS NULL AND action.icon IS NULL AND action.deleted_at IS NULL AND page.code='system:cacheGeneration:view') = 1) THEN 'ok' ELSE 'fail' END"
        if ($menuState -ne 'ok') { throw "验证失败：page/action 形状为 $menuState。" }

        $afterVersion = Get-PsqlValue -Query "SELECT menu_version FROM public.permission_auth_platform WHERE code='admin' AND deleted_at IS NULL AND is_enabled=1"
        Write-Host "migration 后 Admin menu_version=$afterVersion（本次最多 +1，重复执行保持不变）"
        if ($beforeVersion -ne '' -and $afterVersion -ne $beforeVersion) {
            $delta = [int64]$afterVersion - [int64]$beforeVersion
            if ($delta -ne 1) { throw "menu_version 变化异常：$beforeVersion -> $afterVersion。" }
        }
    }

    Write-Host '完成：SQL、legacy Redis cleanup 与验证全部通过。'
    exit 0
} catch {
    [Console]::Error.WriteLine("迁移未完成：{0}", $_.Exception.Message)
    exit 1
}
