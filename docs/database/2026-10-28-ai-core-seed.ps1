[CmdletBinding()]
param(
    [switch]$ServicesStopped,
    [string]$EnvFile = (Join-Path $PSScriptRoot '../../server/.env'),
    [string]$AuditDirectory = (Join-Path $env:LOCALAPPDATA ('Admin/maintenance/ai-core/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '-' + [guid]::NewGuid().ToString('N')))
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '停止 API/Worker 及其他数据库写入入口后使用 -ServicesStopped 执行' }
foreach ($name in @('psql','pg_dump','pg_restore')) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) { throw "缺少 PostgreSQL 工具：$name" }
}
$coreFile = Join-Path $PSScriptRoot '2026-10-28-ai-core.sql'
$seedFile = Join-Path $PSScriptRoot '2026-10-28-ai-core-seed.sql'
foreach ($file in @($coreFile,$seedFile)) { if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw '迁移文件缺失' } }
$envPath = (Resolve-Path -LiteralPath $EnvFile).Path
$lines = @(Get-Content -LiteralPath $envPath | Where-Object { $_ -match '^POSTGRES_DSN=' })
if ($lines.Count -ne 1) { throw '环境文件必须恰好包含一条 POSTGRES_DSN' }
$dsn = $lines[0].Substring($lines[0].IndexOf('=') + 1).Trim().Trim('"').Trim("'")
if ([string]::IsNullOrWhiteSpace($dsn)) { throw 'POSTGRES_DSN 为空' }
# libpq does not accept the Go-only TimeZone option. Keep credentials in memory.
$dsn = $dsn -replace '(?i)\s+TimeZone=(?:''[^'']*''|"[^"]*"|\S+)', ''
$savedEncoding = $env:PGCLIENTENCODING
$savedTimeout = $env:PGCONNECT_TIMEOUT
$savedOptions = $env:PGOPTIONS
$env:PGCLIENTENCODING = 'UTF8'
$env:PGCONNECT_TIMEOUT = '10'
$env:PGOPTIONS = '-c search_path=public -c statement_timeout=60000'
$audit = $null
$auditPath = $null
$committed = $false
$policyKeys = @(
    'ai.run.user_inflight_limit','ai.run.user_rate_per_second','ai.run.user_rate_burst',
    'ai.run.global_rate_per_second','ai.run.global_rate_burst','ai.run.queued_backpressure_threshold',
    'ai.run.active_backpressure_threshold','ai.run.max_queued_seconds','ai.run.text_timeout_seconds',
    'ai.run.image_timeout_seconds','ai.run.audio_timeout_seconds','ai.run.video_timeout_seconds',
    'ai.run.max_model_rounds','ai.run.max_tool_calls','ai.run.max_bound_tools',
    'ai.run.max_input_attachments','ai.run.max_output_tokens','ai.run.max_text_input_bytes','ai.run.max_images',
    'ai.media.image_max_bytes','ai.media.audio_max_bytes','ai.media.video_max_bytes',
    'ai.media.download_timeout_seconds','ai.media.transfer_timeout_seconds',
    'ai.media.spool_max_bytes_per_worker','ai.run.event_retention_days'
)
$policySQL = 'ARRAY[' + (($policyKeys | ForEach-Object { "'$_'" }) -join ',') + ']::text[]'

function Read-JsonQuery([string]$Query) {
    $output = @(& psql -X --dbname=$dsn --no-password --quiet --tuples-only --no-align --set=ON_ERROR_STOP=1 --command=$Query 2>&1)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读核验失败，未输出连接参数' }
    try { return (($output -join [Environment]::NewLine) | ConvertFrom-Json -ErrorAction Stop) }
    catch { throw 'PostgreSQL 只读核验返回了非法 JSON' }
}

function Read-Facts {
    return Read-JsonQuery @"
BEGIN READ ONLY;
SELECT jsonb_build_object(
 'coreTableCount',(SELECT count(*) FROM pg_tables WHERE schemaname=current_schema() AND tablename LIKE 'ai\_%' ESCAPE '\'),
 'coreSchemaStamp',(SELECT obj_description(c.oid,'pg_class') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relname='ai_provider' AND c.relkind='r'),
 'policyRows',(SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY setting_key,id),'[]'::jsonb) FROM system_setting s WHERE setting_key=ANY($policySQL)),
 'otherSettingsDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(s)::text),'' ORDER BY id),'')) FROM system_setting s WHERE setting_key<>ALL($policySQL)),
 'runtimeState',(SELECT to_jsonb(s) FROM system_config_cache_state s WHERE namespace='ai.runtime' AND scope_key='global'),
 'settingState',(SELECT to_jsonb(s) FROM system_config_cache_state s WHERE namespace='system.setting' AND scope_key='global'),
 'outboxRows',(SELECT coalesce(jsonb_agg(to_jsonb(o) ORDER BY id),'[]'::jsonb) FROM system_config_cache_outbox o WHERE namespace IN('ai.runtime','system.setting') AND scope_key='global'),
 'otherCacheDigest',(SELECT md5(coalesce(string_agg(md5(fact),'' ORDER BY fact),'')) FROM (
    SELECT 'state:'||to_jsonb(s)::text fact FROM system_config_cache_state s WHERE NOT(namespace IN('ai.runtime','system.setting') AND scope_key='global')
    UNION ALL SELECT 'outbox:'||to_jsonb(o)::text FROM system_config_cache_outbox o WHERE NOT(namespace IN('ai.runtime','system.setting') AND scope_key='global')
 ) other_cache),
 'schedules',(SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY builtin_key,id),'[]'::jsonb) FROM system_scheduler_schedule s WHERE builtin_key IN('ai.run.recover','ai.run.event.cleanup') AND deleted_at IS NULL),
 'otherSchedulesDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(s)::text),'' ORDER BY id),'')) FROM system_scheduler_schedule s WHERE builtin_key IS NULL OR builtin_key NOT IN('ai.run.recover','ai.run.event.cleanup') OR deleted_at IS NOT NULL),
 'menuDigest',(SELECT md5(coalesce(string_agg(md5(fact),'' ORDER BY fact),'')) FROM (
    SELECT 'menu:'||to_jsonb(m)::text fact FROM permission_menu m
    UNION ALL SELECT 'platform:'||to_jsonb(p)::text FROM permission_auth_platform p
 ) menus)
);
COMMIT;
"@
}

function Json([object]$Value) { return ConvertTo-Json -InputObject $Value -Depth 20 -Compress }
function Save-Audit { $audit | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $auditPath -Encoding utf8 }

function Assert-ExistingRows([object[]]$BeforeRows, [object[]]$AfterRows, [string]$Key) {
    foreach ($row in $BeforeRows) {
        $matches = @($AfterRows | Where-Object { $_.$Key -eq $row.$Key })
        if ($matches.Count -ne 1 -or (Json $matches[0]) -ne (Json $row)) { throw '迁移改写了已有配置、调度或 outbox 事实' }
    }
}

function Assert-SeedFacts([object]$Before, [object]$After) {
    if ($After.coreTableCount -ne 21 -or $After.coreSchemaStamp -notmatch '^admin-ai-core-v1:[a-z0-9]+$') { throw 'AI Core 结构不完整' }
    if ($Before.coreTableCount -eq 21 -and $After.coreSchemaStamp -ne $Before.coreSchemaStamp) { throw '既有 AI Core 结构指纹被改写' }
    foreach ($field in @('otherSettingsDigest','otherCacheDigest','otherSchedulesDigest','menuDigest')) {
        if ($After.$field -ne $Before.$field) { throw "迁移改写了无关事实：$field" }
    }
    $policies = @($After.policyRows)
    if ($policies.Count -ne $policyKeys.Count) { throw 'AI 策略数量不完整或重复' }
    foreach ($key in $policyKeys) {
        $rows = @($policies | Where-Object { $_.setting_key -eq $key })
        if ($rows.Count -ne 1) { throw 'AI 策略缺失或重复' }
        $row = $rows[0]
        if ($row.value_type -ne 2 -or $row.status -ne 1 -or $row.is_builtin -ne 1 -or $null -ne $row.deleted_at -or
            $row.value -notmatch '^[0-9]{1,19}$' -or [bigint]$row.value -lt 1 -or [bigint]$row.value -gt [bigint][long]::MaxValue) { throw 'AI 策略值或元数据不合法' }
    }
    Assert-ExistingRows @($Before.policyRows) $policies 'setting_key'
    $changed = [int](@($Before.policyRows).Count -lt $policies.Count)
    $settingGeneration = [bigint]$Before.settingState.generation + $changed
    $runtimeGeneration = [bigint]1
    if ($null -ne $Before.runtimeState) { $runtimeGeneration = [bigint]$Before.runtimeState.generation + $changed }
    if ($null -eq $After.runtimeState -or $null -eq $After.settingState -or
        [bigint]$After.settingState.generation -ne $settingGeneration -or [bigint]$After.runtimeState.generation -ne $runtimeGeneration) { throw '缓存 generation 推进不符合实际 seed 变更' }
    foreach ($field in @('settingState','runtimeState')) {
        if ($null -ne $Before.$field) {
            foreach ($property in @('namespace','scope_key','created_at')) {
                if ($Before.$field.$property -ne $After.$field.$property) { throw '缓存 scope 身份被改写' }
            }
            if ($changed -eq 0 -and (Json $Before.$field) -ne (Json $After.$field)) { throw '重复执行改变了缓存 state' }
        }
    }
    if ($After.runtimeState.namespace -ne 'ai.runtime' -or $After.runtimeState.scope_key -ne 'global') { throw 'AI runtime scope 不合法' }
    Assert-ExistingRows @($Before.outboxRows) @($After.outboxRows) 'id'
    $newRows = @($After.outboxRows | Where-Object { $id=$_.id; @($Before.outboxRows | Where-Object { $_.id -eq $id }).Count -eq 0 })
    $wantNew = $changed
    if ($null -eq $Before.runtimeState -or $changed -eq 1) { $wantNew++ }
    if ($newRows.Count -ne $wantNew) { throw 'outbox 事件数量不符合实际 seed 变更' }
    foreach ($row in $newRows) {
        $wantGeneration = $runtimeGeneration
        if ($row.namespace -eq 'system.setting') { $wantGeneration = $settingGeneration }
        elseif ($row.namespace -ne 'ai.runtime') { throw 'outbox namespace 不合法' }
        if ($row.scope_key -ne 'global' -or [bigint]$row.generation -ne $wantGeneration -or $null -ne $row.published_at -or $row.attempts -ne 0) { throw '新 outbox 事实不合法' }
    }
    if (@($After.outboxRows | Where-Object { $_.namespace -eq 'ai.runtime' -and $_.scope_key -eq 'global' -and [bigint]$_.generation -eq $runtimeGeneration }).Count -ne 1) { throw 'AI runtime 当前代际缺少唯一 outbox' }
    $schedules = @($After.schedules)
    if ($schedules.Count -ne 2) { throw 'AI 周期调度数量不完整' }
    foreach ($key in @('ai.run.recover','ai.run.event.cleanup')) {
        $rows = @($schedules | Where-Object { $_.builtin_key -eq $key })
        if ($rows.Count -ne 1) { throw 'AI builtin 调度缺失或重复' }
        $cron = '* * * * *'
        if ($key -eq 'ai.run.event.cleanup') { $cron = '0 3 * * *' }
        $row = $rows[0]
        if ($row.task_type -ne $key -or $row.cron_expression -ne $cron -or $row.timezone -ne 'Asia/Shanghai' -or
            (Json $row.params) -ne '{}' -or $row.status -ne 0 -or $null -ne $row.next_run_at -or $null -ne $row.deleted_at) { throw 'AI 调度定义不符合待装配停用状态' }
    }
    Assert-ExistingRows @($Before.schedules) $schedules 'builtin_key'
}

try {
    $clients = Read-JsonQuery "BEGIN READ ONLY; SELECT to_json(count(*)) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND backend_type='client backend'; COMMIT;"
    if ($clients -ne 0) { throw '数据库仍有客户端连接，关闭服务和数据库工具后重试；脚本不会终止连接' }
    $before = Read-Facts
    if ($before.coreTableCount -notin @(0,21) -or $null -eq $before.settingState) { throw 'AI Core 或系统缓存前置事实不完整' }
    $directory = [IO.Path]::GetFullPath($AuditDirectory)
    if (Test-Path -LiteralPath $directory) { throw '审计目录已存在，禁止覆盖已有备份与审计' }
    New-Item -ItemType Directory -Path $directory -ErrorAction Stop | Out-Null
    $auditPath = Join-Path $directory 'audit.json'
    $audit = [ordered]@{
        migrations = @('2026-10-28-ai-core.sql','2026-10-28-ai-core-seed.sql')
        startedAt = (Get-Date).ToUniversalTime().ToString('o')
        sqlSHA256 = [ordered]@{core=(Get-FileHash -LiteralPath $coreFile -Algorithm SHA256).Hash;seed=(Get-FileHash -LiteralPath $seedFile -Algorithm SHA256).Hash}
        before = $before
        committed = $false
        commitState = 'not_started'
        stage = 'permissions'
    }
    Save-Audit
    if ($env:OS -eq 'Windows_NT') {
        $owner = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        & icacls.exe $directory /inheritance:r /grant:r ($owner + ':(OI)(CI)F') 'SYSTEM:(OI)(CI)F' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw '限制备份目录访问权限失败，未执行迁移' }
    }
    $audit.stage = 'backup'
    Save-Audit
    $backup = Join-Path $directory 'before.dump'
    $null = & pg_dump --dbname=$dsn --no-password --format=custom --file=$backup 2>&1
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $backup)) { throw '备份失败，未执行迁移' }
    $null = & pg_restore --list $backup 2>&1
    if ($LASTEXITCODE -ne 0) { throw '备份归档不可读，未执行迁移' }
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash
    $audit.stage = 'apply'
    $audit.commitState = 'unknown'
    Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$coreFile --file=$seedFile 2>&1
    $migrationExitCode = $LASTEXITCODE
    if ($migrationExitCode -ne 0) {
        if ($migrationExitCode -eq 3) { $audit.commitState='rolled_back'; Save-Audit; throw 'Core/seed 事务失败并回滚，备份和审计已保留' }
        throw 'PostgreSQL 提交状态未知（连接/客户端错误）；保持服务停止，先只读核验，勿按已回滚处理'
    }
    $committed = $true
    $audit.committed = $true
    $audit.commitState = 'committed'
    $audit.stage = 'verify'
    Save-Audit
    $after = Read-Facts
    $audit['after'] = $after
    Save-Audit
    Assert-SeedFacts $before $after
    $audit.stage = 'replay'
    Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$coreFile --file=$seedFile 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'Core/seed 幂等复跑失败' }
    $audit.stage = 'verify_replay'
    $replay = Read-Facts
    $audit['replay'] = $replay
    Save-Audit
    Assert-SeedFacts $after $replay
    if ((Json $after) -ne (Json $replay)) { throw 'Core/seed 幂等复跑改变了事实' }
    $audit.stage = 'completed'
    $audit['completedAt'] = (Get-Date).ToUniversalTime().ToString('o')
    $audit['cacheChanges'] = 'AI runtime 初始化与 system.setting 变更均写 generation/outbox；无 Redis cleanup，正常 relay 发布'
    $audit['activation'] = 'AI 周期任务保持停用、next_run_at 为空，须在真实 Worker 装配完成后显式启用'
    Save-Audit
    Write-Output "AI Core/seed 迁移与幂等复跑通过，备份/审计：$directory"
    Write-Output 'AI Worker 尚未装配时不要启用 AI 周期任务；钱包扩展和菜单迁移仍需独立交付。'
} catch {
    if ($audit -and $auditPath) {
        $audit['committed'] = $committed
        $audit['failedAt'] = (Get-Date).ToUniversalTime().ToString('o')
        try { Save-Audit } catch { Write-Warning '失败审计写入失败，请保留备份目录' }
    }
    if ($audit -and $audit.commitState -eq 'unknown') { Write-Warning 'PostgreSQL 提交状态未知；保持服务停止，先只读核验' }
    elseif ($committed) { Write-Warning 'PostgreSQL 已提交，后续校验未完成；保持服务停止，保留备份和审计后核验复跑' }
    throw
} finally {
    $env:PGCLIENTENCODING = $savedEncoding
    $env:PGCONNECT_TIMEOUT = $savedTimeout
    $env:PGOPTIONS = $savedOptions
    $dsn = $null
}
