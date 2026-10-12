<#
.SYNOPSIS
    Rename the configuration cache generation state table and align its RBAC menu contract.

    The runner executes only the forward PostgreSQL migration. It does not touch Redis.
    The second SQL invocation verifies that a repeat is a no-op for generations, outbox rows,
    menu rows, and platform menu_version values.
#>
[CmdletBinding()]
param(
    [switch]$OldAPIStopped
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not $OldAPIStopped) {
    throw '拒绝执行：旧 API/Worker 仍可能读取旧表名；确认已摘除写流量后传入 -OldAPIStopped。'
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir = Join-Path $repoRoot 'server'
$sqlPath = Join-Path $PSScriptRoot '2026-10-11-system-config-cache-status.sql'
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

$fileValues = Read-EnvFile -Path $envFile
$postgresDsn = Get-EffectiveSetting -FileValues $fileValues -Key 'POSTGRES_DSN'
if ([string]::IsNullOrWhiteSpace($postgresDsn)) { throw '缺少 POSTGRES_DSN（server/.env 或进程环境）。' }
$psqlDsn = $postgresDsn -replace '\s+TimeZone=\S+', ''
$env:PGCLIENTENCODING = 'UTF8'

function Get-PsqlValue {
    param([string]$Query)
    $output = @(& psql -X -A -t -d $psqlDsn -v ON_ERROR_STOP=1 -c $Query)
    if ($LASTEXITCODE -ne 0) { throw "psql 只读验证失败（exit $LASTEXITCODE）。" }
    $lines = @($output | Where-Object { $_ -ne '' })
    if ($lines.Count -eq 0) { return '' }
    return $lines[-1].Trim()
}

function Invoke-Migration {
    & psql -X -d $psqlDsn -v ON_ERROR_STOP=1 -f $sqlPath
    if ($LASTEXITCODE -ne 0) {
        throw "forward migration 失败（exit $LASTEXITCODE）；事务应已回滚。"
    }
}

function Get-CacheTableName {
    $value = Get-PsqlValue -Query "SELECT CASE WHEN to_regclass('public.system_config_cache_state') IS NOT NULL AND to_regclass('public.system_config_cache_generation') IS NULL THEN 'system_config_cache_state' WHEN to_regclass('public.system_config_cache_generation') IS NOT NULL AND to_regclass('public.system_config_cache_state') IS NULL THEN 'system_config_cache_generation' ELSE 'invalid' END"
    if ($value -eq 'invalid') { throw "缓存状态表形状非法：$value。" }
    return $value
}

function Get-GenerationFacts {
    param([string]$TableName)
    $query = "SELECT md5(COALESCE((SELECT string_agg(namespace || '/' || scope_key || ':' || generation, ',' ORDER BY namespace,scope_key) FROM public.$TableName),'' ) || '|' || COALESCE((SELECT string_agg(id || ':' || namespace || '/' || scope_key || ':' || generation || ':' || attempts || ':' || COALESCE(published_at::text,''), ',' ORDER BY id) FROM public.system_config_cache_outbox),''))"
    return Get-PsqlValue -Query $query
}

function Get-MenuFacts {
    $query = "SELECT md5(COALESCE((SELECT string_agg(platform_id || ':' || id || ':' || COALESCE(parent_id::text,'') || ':' || menu_type || ':' || code || ':' || COALESCE(i18n_key,'') || ':' || COALESCE(path,'') || ':' || COALESCE(component_path,''), ',' ORDER BY platform_id,id) FROM public.permission_menu WHERE deleted_at IS NULL AND (code IN ('system:cacheGeneration:view','system:cacheGeneration:list','system:configCacheStatus:view','system:configCacheStatus:list') OR path IN ('/system/cacheGeneration','/system/configCacheStatus') OR component_path IN ('system/cacheGeneration','system/configCacheStatus'))),''))"
    return Get-PsqlValue -Query $query
}

function Get-MenuVersions {
    return Get-PsqlValue -Query "SELECT COALESCE(string_agg(id || ':' || menu_version, ',' ORDER BY id),'') FROM public.permission_auth_platform WHERE deleted_at IS NULL AND is_enabled=1"
}

function Assert-TargetSchema {
    $state = Get-PsqlValue -Query "SELECT CASE WHEN to_regclass('public.system_config_cache_state') IS NOT NULL AND to_regclass('public.system_config_cache_generation') IS NULL AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.system_config_cache_state'::regclass AND conname='pk_system_config_cache_state') AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.system_config_cache_state'::regclass AND conname='ck_system_config_cache_state_namespace') AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.system_config_cache_state'::regclass AND conname='ck_system_config_cache_state_scope_key') AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.system_config_cache_state'::regclass AND conname='ck_system_config_cache_state_value') AND EXISTS (SELECT 1 FROM pg_constraint c JOIN pg_class r ON r.oid=c.confrelid WHERE c.conrelid='public.system_config_cache_outbox'::regclass AND c.conname='fk_system_config_cache_outbox_state' AND r.oid='public.system_config_cache_state'::regclass) THEN 'ok' ELSE 'fail' END"
    if ($state -ne 'ok') { throw "缓存状态表/约束/FK 验证失败：$state。" }
}

function Assert-MenuContract {
    $state = Get-PsqlValue -Query "SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM public.permission_menu WHERE deleted_at IS NULL AND code LIKE 'system:cacheGeneration:%') AND NOT EXISTS (SELECT 1 FROM public.permission_menu WHERE deleted_at IS NULL AND path='/system/cacheGeneration') AND NOT EXISTS (SELECT 1 FROM public.permission_menu WHERE deleted_at IS NULL AND component_path='system/cacheGeneration') AND NOT EXISTS (SELECT 1 FROM public.permission_menu WHERE deleted_at IS NULL AND code='system:configCacheStatus:view' AND (menu_type <> 'page' OR i18n_key <> 'navigation.systemConfigCacheStatus' OR path <> '/system/configCacheStatus' OR component_path <> 'system/configCacheStatus')) AND NOT EXISTS (SELECT 1 FROM public.permission_menu WHERE deleted_at IS NULL AND code='system:configCacheStatus:list' AND (menu_type <> 'action' OR i18n_key IS NOT NULL OR path IS NOT NULL OR component_path IS NOT NULL)) AND NOT EXISTS (SELECT 1 FROM public.permission_menu AS action WHERE action.deleted_at IS NULL AND action.code='system:configCacheStatus:list' AND NOT EXISTS (SELECT 1 FROM public.permission_menu AS page WHERE page.id=action.parent_id AND page.deleted_at IS NULL AND page.code='system:configCacheStatus:view')) THEN 'ok' ELSE 'fail' END"
    if ($state -ne 'ok') { throw "菜单契约验证失败：$state。" }
}

try {
    $beforeTable = Get-CacheTableName
    $beforeGeneration = Get-GenerationFacts -TableName $beforeTable
    $beforeMenu = Get-MenuFacts
    $beforeVersions = Get-MenuVersions
    Write-Host "迁移前缓存表=$beforeTable；执行 forward SQL：$sqlPath"
    Invoke-Migration
    $firstTable = Get-CacheTableName
    $firstGeneration = Get-GenerationFacts -TableName $firstTable
    $firstMenu = Get-MenuFacts
    $firstVersions = Get-MenuVersions
    Assert-TargetSchema
    Assert-MenuContract

    # Second execution is the idempotence probe. No generation/outbox/menu fact may move.
    Write-Host '执行第二次 forward SQL 进行幂等验证。'
    Invoke-Migration
    $secondTable = Get-CacheTableName
    $secondGeneration = Get-GenerationFacts -TableName $secondTable
    $secondMenu = Get-MenuFacts
    $secondVersions = Get-MenuVersions
    Assert-TargetSchema
    Assert-MenuContract
    if ($secondTable -ne 'system_config_cache_state' -or $firstTable -ne 'system_config_cache_state') { throw '重复执行后的缓存表名不正确。' }
    if ($firstGeneration -ne $secondGeneration) { throw "generation/outbox 幂等校验失败：$firstGeneration -> $secondGeneration。" }
    if ($firstMenu -ne $secondMenu) { throw "菜单事实幂等校验失败：$firstMenu -> $secondMenu。" }
    if ($firstVersions -ne $secondVersions) { throw "menu_version 幂等校验失败：$firstVersions -> $secondVersions。" }
    Write-Host "完成：缓存状态表、菜单契约和幂等复跑验证通过（初始缓存表=$beforeTable；初始 facts=$beforeGeneration/$beforeMenu/$beforeVersions）。"
    exit 0
} catch {
    [Console]::Error.WriteLine("迁移未完成：{0}", $_.Exception.Message)
    exit 1
}
