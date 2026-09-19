<#
.SYNOPSIS
    实时通道与站内通知 forward migration 的唯一入口。

.DESCRIPTION
    默认模式备份 public schema、执行同名 SQL、执行两轮固定 Redis cleanup，并验证 PostgreSQL/Redis 事实。
    -CleanupOnly 只重试 forward cleanup；-RollbackCleanupOnly 只执行固定 rollback cleanup。
    所有模式都要求显式传入 -OldAPIStopped，不启动 API/Worker，不删除备份。
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
$sqlPath = Join-Path $PSScriptRoot '2026-09-18-realtime-notification.sql'
$envFile = Join-Path $serverDir '.env'
$cleanupFailureNotice = 'PostgreSQL 已提交、Redis cleanup 未完成；可安全重复执行本 runner'
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
    $value = [System.Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    if ($FileValues.ContainsKey($Key)) { return [string]$FileValues[$Key] }
    return ''
}

try {
    if (-not $OldAPIStopped) {
        throw '拒绝执行：请先停止旧 API/Worker，并显式传入 -OldAPIStopped。'
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
        & go run ./cmd/realtime-notification-migration -mode $Mode
        if ($LASTEXITCODE -ne 0) {
            throw "$Phase cleanup（mode=$Mode）失败（exit $LASTEXITCODE）。$cleanupFailureNotice"
        }
    }

    if ($RollbackCleanupOnly) {
        Push-Location $serverDir
        try {
            Invoke-Cleanup -Mode 'rollback' -Phase '首次回滚'
            Invoke-Cleanup -Mode 'rollback' -Phase '回滚复验'
        } finally { Pop-Location }
        Write-Host '完成：rollback cleanup 已执行两次，所有固定 pattern remaining=0。'
        exit 0
    }

    if ($CleanupOnly) {
        Push-Location $serverDir
        try {
            Invoke-Cleanup -Mode 'forward' -Phase '首次'
            Invoke-Cleanup -Mode 'forward' -Phase '复验'
        } finally { Pop-Location }
        Write-Host '完成：CleanupOnly 已执行两次，未修改 PostgreSQL。'
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

    $tableCountQuery = "SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ('message_notification_task','message_notification_task_target','message_notification','message_notification_recipient','message_notification_broadcast_state','message_notification_mailbox_state','message_notification_dispatch_outbox','realtime_event','realtime_event_outbox','realtime_retention_state')"
    $expectedSchemaFingerprint = '2ebc4dc2c8f4746f21d676304fa120e6'
    $schemaFingerprintQuery = @"
WITH targets(table_name) AS (VALUES
  ('message_notification_task'),('message_notification_task_target'),('message_notification'),
  ('message_notification_recipient'),('message_notification_broadcast_state'),
  ('message_notification_mailbox_state'),('message_notification_dispatch_outbox'),
  ('realtime_event'),('realtime_event_outbox'),('realtime_retention_state')
), columns AS (
  SELECT string_agg(format('%s.%s:%s:%s:%s:%s:%s:%s',column_row.table_name,column_row.ordinal_position,column_row.column_name,column_row.udt_name,column_row.is_nullable,column_row.is_identity,COALESCE(column_row.character_maximum_length::text,''),COALESCE(column_row.column_default,'')),E'\n' ORDER BY column_row.table_name,column_row.ordinal_position) AS value
  FROM information_schema.columns column_row JOIN targets ON targets.table_name=column_row.table_name
  WHERE column_row.table_schema=current_schema()
), constraints AS (
  SELECT string_agg(format('%s:%s:%s:%s',relation.relname,constraint_row.conname,constraint_row.contype,pg_get_constraintdef(constraint_row.oid,true)),E'\n' ORDER BY relation.relname,constraint_row.conname) AS value
  FROM pg_constraint constraint_row
  JOIN pg_class relation ON relation.oid=constraint_row.conrelid
  JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
  JOIN targets ON targets.table_name=relation.relname
  WHERE namespace.nspname=current_schema()
), indexes AS (
  SELECT string_agg(format('%s:%s:%s',index_row.tablename,index_row.indexname,replace(index_row.indexdef,format('%I.',current_schema()),'')),E'\n' ORDER BY index_row.tablename,index_row.indexname) AS value
  FROM pg_indexes index_row JOIN targets ON targets.table_name=index_row.tablename
  WHERE index_row.schemaname=current_schema()
)
SELECT md5(concat_ws(E'\n--constraints--\n',COALESCE(columns.value,''),COALESCE(constraints.value,''),COALESCE(indexes.value,'')))
FROM columns,constraints,indexes
"@
    $settingFactsQuery = "SELECT md5(COALESCE(string_agg(concat_ws('|',setting_key,value,value_type,is_enabled,is_builtin,description,created_at,updated_at),';' ORDER BY setting_key),'')) FROM system_setting WHERE deleted_at IS NULL AND setting_key IN ('message.notification.retention_days','realtime.event.retention_days')"
    $generationFactsQuery = "SELECT md5(concat(COALESCE((SELECT namespace || '|' || scope_key || '|' || generation || '|' || created_at || '|' || updated_at FROM system_config_cache_generation WHERE namespace='system.setting' AND scope_key='global'),''),'#',COALESCE((SELECT string_agg(concat_ws('|',id,generation,attempts,available_at,locked_until,lock_token,last_error,published_at,created_at,updated_at),';' ORDER BY id) FROM system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global'),'')))"
    $menuFactsQuery = "SELECT md5(COALESCE(string_agg(concat_ws('|',id,platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,is_enabled,is_hidden,created_at,updated_at),';' ORDER BY id),'')) FROM permission_menu WHERE deleted_at IS NULL AND (code LIKE 'message:notification:%' OR code LIKE 'message:notificationTask:%')"
    $roleGrantFactsQuery = "SELECT md5(COALESCE(string_agg(concat_ws('|',rm.id,rm.role_id,rm.menu_id,rm.created_at,rm.updated_at),';' ORDER BY rm.id),'')) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.deleted_at IS NULL AND m.deleted_at IS NULL AND m.platform_id=(SELECT id FROM permission_auth_platform WHERE code='admin' AND deleted_at IS NULL) AND m.code IN ('message:notification:view','message:notification:list','message:notification:read','message:notification:delete')"
    $accessFactsQuery = "SELECT md5(COALESCE(string_agg(user_id || '|' || version || '|' || created_at || '|' || updated_at,';' ORDER BY user_id),'')) FROM permission_access_version"
    $adminVersionQuery = "SELECT menu_version FROM permission_auth_platform WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL"
    $canvasFingerprintQuery = "SELECT md5(concat(p.menu_version,'#',COALESCE(string_agg(concat_ws('|',m.id,m.parent_id,m.menu_type,m.code,m.path,m.component_path,m.is_hidden,m.is_enabled,m.created_at,m.updated_at),';' ORDER BY m.id),''))) FROM permission_auth_platform p LEFT JOIN permission_menu m ON m.platform_id=p.id AND m.deleted_at IS NULL WHERE p.code='canvas' AND p.deleted_at IS NULL GROUP BY p.id,p.menu_version"
    $completeQuery = "SELECT CASE WHEN ($tableCountQuery)=10 AND ($schemaFingerprintQuery)='$expectedSchemaFingerprint' AND (SELECT count(*) FROM permission_menu WHERE platform_id=(SELECT id FROM permission_auth_platform WHERE code='admin' AND deleted_at IS NULL) AND deleted_at IS NULL AND (code LIKE 'message:notification:%' OR code LIKE 'message:notificationTask:%'))=13 AND (SELECT count(*) FROM system_setting WHERE deleted_at IS NULL AND setting_key IN ('message.notification.retention_days','realtime.event.retention_days') AND value_type=2 AND is_enabled=1 AND is_builtin=1)=2 THEN 'yes' ELSE 'no' END"

    $before = [ordered]@{
        complete       = Get-PsqlValue -Query $completeQuery
        structure      = Get-PsqlValue -Query $schemaFingerprintQuery
        setting        = Get-PsqlValue -Query $settingFactsQuery
        generation     = Get-PsqlValue -Query $generationFactsQuery
        menu            = Get-PsqlValue -Query $menuFactsQuery
        grants          = Get-PsqlValue -Query $roleGrantFactsQuery
        access          = Get-PsqlValue -Query $accessFactsQuery
        adminVersion    = Get-PsqlValue -Query $adminVersionQuery
        canvas          = Get-PsqlValue -Query $canvasFingerprintQuery
    }
    Write-Host ("迁移前指纹：complete={0} structure={1} setting={2} generation={3} menu={4} grants={5} access={6} adminMenuVersion={7} canvas={8}" -f $before.complete,$before.structure,$before.setting,$before.generation,$before.menu,$before.grants,$before.access,$before.adminVersion,$before.canvas)

    $backupDir = Join-Path $env:LOCALAPPDATA ("Admin\backups\realtime-notification-" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
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

    & psql -X -d $psqlDsn -v ON_ERROR_STOP=1 -f $sqlPath
    if ($LASTEXITCODE -ne 0) { throw "migration SQL 失败（exit $LASTEXITCODE）：事务已整体回滚，Redis cleanup 未运行。" }
    $sqlCommitted = $true
    Write-Host 'migration SQL 已提交。'

    Push-Location $serverDir
    try {
        Invoke-Cleanup -Mode 'forward' -Phase '首次'
        Invoke-Cleanup -Mode 'forward' -Phase '复验'
    } finally { Pop-Location }

    Assert-PsqlValue '十张目标表' $tableCountQuery '10'
    Assert-PsqlValue '目标 schema 结构指纹' $schemaFingerprintQuery $expectedSchemaFingerprint
    Assert-PsqlValue '两个必需 setting 合法' "SELECT CASE WHEN (SELECT count(*) FROM system_setting WHERE deleted_at IS NULL AND setting_key='message.notification.retention_days' AND value_type=2 AND is_enabled=1 AND is_builtin=1 AND value ~ '^[0-9]+$' AND value::BIGINT BETWEEN 30 AND 3650)=1 AND (SELECT count(*) FROM system_setting WHERE deleted_at IS NULL AND setting_key='realtime.event.retention_days' AND value_type=2 AND is_enabled=1 AND is_builtin=1 AND value ~ '^[0-9]+$' AND value::BIGINT BETWEEN 1 AND 30)=1 THEN 'ok' ELSE 'fail' END" 'ok'
    Assert-PsqlValue 'Admin 通知菜单节点' "SELECT count(*) FROM permission_menu WHERE platform_id=(SELECT id FROM permission_auth_platform WHERE code='admin' AND deleted_at IS NULL) AND deleted_at IS NULL AND (code LIKE 'message:notification:%' OR code LIKE 'message:notificationTask:%')" '13'
    Assert-PsqlValue '非 Admin 未创建通知节点' "SELECT count(*) FROM permission_menu WHERE platform_id<>(SELECT id FROM permission_auth_platform WHERE code='admin' AND deleted_at IS NULL) AND deleted_at IS NULL AND (code LIKE 'message:notification:%' OR code LIKE 'message:notificationTask:%')" '0'
    Assert-PsqlValue '有效角色个人通知授权完整' "SELECT CASE WHEN (SELECT count(*) FROM permission_role WHERE is_enabled=1 AND deleted_at IS NULL)*4=(SELECT count(*) FROM permission_role_menu rm JOIN permission_role r ON r.id=rm.role_id JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.deleted_at IS NULL AND r.is_enabled=1 AND r.deleted_at IS NULL AND m.deleted_at IS NULL AND m.code IN ('message:notification:view','message:notification:list','message:notification:read','message:notification:delete')) THEN 'ok' ELSE 'fail' END" 'ok'
    Assert-PsqlValue '角色历史游标索引' "SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema() AND indexname='ix_permission_user_role_role_history_cursor'" '1'
    Assert-PsqlValue 'Canvas 菜单和版本保持' $canvasFingerprintQuery "$($before.canvas)"

    $after = [ordered]@{
        structure   = Get-PsqlValue -Query $schemaFingerprintQuery
        setting     = Get-PsqlValue -Query $settingFactsQuery
        generation  = Get-PsqlValue -Query $generationFactsQuery
        menu         = Get-PsqlValue -Query $menuFactsQuery
        grants       = Get-PsqlValue -Query $roleGrantFactsQuery
        access       = Get-PsqlValue -Query $accessFactsQuery
        adminVersion = Get-PsqlValue -Query $adminVersionQuery
        canvas       = Get-PsqlValue -Query $canvasFingerprintQuery
    }
    Write-Host ("迁移后指纹：structure={0} setting={1} generation={2} menu={3} grants={4} access={5} adminMenuVersion={6} canvas={7}" -f $after.structure,$after.setting,$after.generation,$after.menu,$after.grants,$after.access,$after.adminVersion,$after.canvas)

    if ($before.complete -eq 'yes') {
        foreach ($name in @('structure','setting','generation','menu','grants','access','adminVersion','canvas')) {
            if ($before[$name] -ne $after[$name]) { throw "幂等验证失败：$name 指纹发生变化。" }
        }
        Write-Host '幂等验证通过：完整事实与版本指纹均未变化。'
    }
    Write-Host '完成：SQL、两轮 Redis cleanup 和只读验证全部通过。'
    exit 0
} catch {
    [Console]::Error.WriteLine("迁移未完成：{0}", $_.Exception.Message)
    if ($sqlCommitted) { [Console]::Error.WriteLine($cleanupFailureNotice) }
    exit 1
}
