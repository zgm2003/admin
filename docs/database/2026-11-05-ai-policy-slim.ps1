[CmdletBinding()]
param(
    [switch]$ServicesStopped,
    [string]$EnvFile = (Join-Path $PSScriptRoot '../../server/.env'),
    [string]$AuditDirectory = (Join-Path $env:LOCALAPPDATA ('Admin/maintenance/ai-policy-slim/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '-' + [guid]::NewGuid().ToString('N')))
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '停止 API/Worker 及其他数据库写入入口后使用 -ServicesStopped 执行' }
foreach ($name in @('psql', 'pg_dump', 'pg_restore')) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) { throw "缺少 PostgreSQL 工具：$name" }
}

$sqlFile = Join-Path $PSScriptRoot '2026-11-05-ai-policy-slim.sql'
if (-not (Test-Path -LiteralPath $sqlFile -PathType Leaf)) { throw '迁移文件缺失' }
$envPath = (Resolve-Path -LiteralPath $EnvFile).Path
$lines = @(Get-Content -LiteralPath $envPath | Where-Object { $_ -match '^POSTGRES_DSN=' })
if ($lines.Count -ne 1) { throw '环境文件必须恰好包含一条 POSTGRES_DSN' }
$dsn = $lines[0].Substring($lines[0].IndexOf('=') + 1).Trim().Trim('"').Trim("'")
if ([string]::IsNullOrWhiteSpace($dsn)) { throw 'POSTGRES_DSN 为空' }
$dsn = $dsn -replace '(?i)\s+TimeZone=(?:''[^'']*''|"[^"]*"|\S+)', ''
$savedEncoding, $savedTimeout, $savedOptions = $env:PGCLIENTENCODING, $env:PGCONNECT_TIMEOUT, $env:PGOPTIONS
$env:PGCLIENTENCODING = 'UTF8'
$env:PGCONNECT_TIMEOUT = '10'
$env:PGOPTIONS = '-c search_path=public -c statement_timeout=60000'
$audit = $null
$auditPath = $null
$committed = $false

$retainedKeys = @(
    'ai.run.user_inflight_limit',
    'ai.run.user_rate_per_second',
    'ai.run.user_rate_burst',
    'ai.run.max_queued_seconds',
    'ai.run.event_retention_days'
)
$removedKeys = @(
    'ai.run.global_rate_per_second',
    'ai.run.global_rate_burst',
    'ai.run.queued_backpressure_threshold',
    'ai.run.active_backpressure_threshold',
    'ai.run.text_timeout_seconds',
    'ai.run.image_timeout_seconds',
    'ai.run.audio_timeout_seconds',
    'ai.run.video_timeout_seconds',
    'ai.run.max_model_rounds',
    'ai.run.max_tool_calls',
    'ai.run.max_bound_tools',
    'ai.run.max_input_attachments',
    'ai.run.max_output_tokens',
    'ai.run.max_text_input_bytes',
    'ai.run.max_images',
    'ai.media.image_max_bytes',
    'ai.media.audio_max_bytes',
    'ai.media.video_max_bytes',
    'ai.media.download_timeout_seconds',
    'ai.media.transfer_timeout_seconds',
    'ai.media.spool_max_bytes_per_worker'
)
$retainedSql = 'ARRAY[' + (($retainedKeys | ForEach-Object { "'" + $_ + "'" }) -join ',') + ']::text[]'
$removedSql = 'ARRAY[' + (($removedKeys | ForEach-Object { "'" + $_ + "'" }) -join ',') + ']::text[]'

function Save-Audit { $audit | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $auditPath -Encoding utf8 }
function Json([object]$Value) { return ConvertTo-Json -InputObject $Value -Depth 20 -Compress }

function Read-JsonQuery([string]$Query) {
    $output = @(& psql -X --dbname=$dsn --no-password --quiet --tuples-only --no-align --set=ON_ERROR_STOP=1 --command=$Query 2>&1)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读核验失败，未输出连接参数' }
    try { return (($output -join [Environment]::NewLine) | ConvertFrom-Json -ErrorAction Stop) }
    catch { throw 'PostgreSQL 只读核验返回了非法 JSON' }
}

function Read-Facts {
    $factsSql = @'
BEGIN READ ONLY;
SELECT jsonb_build_object(
 'aiSettingCount',(SELECT count(*) FROM system_setting WHERE setting_key LIKE 'ai.%' AND deleted_at IS NULL),
 'retained',(SELECT coalesce(jsonb_agg(jsonb_build_object('key',setting_key,'value',value,'value_type',value_type,'status',status,'is_builtin',is_builtin) ORDER BY setting_key),'[]'::jsonb) FROM system_setting WHERE setting_key = ANY(__RETAINED__) AND deleted_at IS NULL),
 'removedCount',(SELECT count(*) FROM system_setting WHERE setting_key = ANY(__REMOVED__) AND deleted_at IS NULL),
 'policyCheck',(SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname='ck_system_setting_ai_policy'),
 'settingState',(SELECT to_jsonb(s) FROM system_config_cache_state s WHERE s.namespace='system.setting' AND s.scope_key='global'),
 'settingOutbox',(SELECT coalesce(jsonb_agg(to_jsonb(o) ORDER BY o.id),'[]'::jsonb) FROM system_config_cache_outbox o WHERE o.namespace='system.setting' AND o.scope_key='global'),
 'otherSettingsDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(s)::text),'' ORDER BY s.id),'')) FROM system_setting s WHERE s.setting_key NOT LIKE 'ai.%'),
 'otherCacheDigest',(SELECT md5(coalesce(string_agg(fact,'' ORDER BY fact),'')) FROM (
    SELECT 'state:'||to_jsonb(s)::text fact FROM system_config_cache_state s WHERE NOT(s.namespace='system.setting' AND s.scope_key='global')
    UNION ALL SELECT 'outbox:'||to_jsonb(o)::text FROM system_config_cache_outbox o WHERE NOT(o.namespace='system.setting' AND o.scope_key='global')
 ) other_cache),
 'menuDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(m)::text),'' ORDER BY m.id),'')) FROM permission_menu m),
 'platformDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(p)::text),'' ORDER BY p.id),'')) FROM permission_auth_platform p),
 'roleMenuDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(g)::text),'' ORDER BY g.id),'')) FROM permission_role_menu g),
 'aiRunCount',(SELECT count(*) FROM ai_run)
);
COMMIT;
'@
    $factsSql = $factsSql.Replace('__RETAINED__', $retainedSql).Replace('__REMOVED__', $removedSql)
    return Read-JsonQuery $factsSql
}

function Assert-PolicyCheck([object]$Facts) {
    if ([string]::IsNullOrWhiteSpace($Facts.policyCheck)) { throw 'AI policy check 缺失' }
    foreach ($key in $retainedKeys) {
        if ($Facts.policyCheck -notmatch [regex]::Escape($key)) { throw "AI policy check 缺少保留键：$key" }
    }
    foreach ($key in $removedKeys) {
        if ($Facts.policyCheck -match [regex]::Escape($key)) { throw "AI policy check 仍包含已删除键：$key" }
    }
    if ($Facts.policyCheck -notmatch 'value_type = 2' -or $Facts.policyCheck -notmatch 'status = 1' -or $Facts.policyCheck -notmatch 'is_builtin = 1' -or $Facts.policyCheck -notmatch 'deleted_at IS NULL') {
        throw 'AI policy check 语义不完整'
    }
}

function Assert-RetainedUnchanged([object]$Before, [object]$After) {
    if ((Json $Before.retained) -ne (Json $After.retained)) { throw '迁移改写了保留设置' }
}

try {
    $directory = [IO.Path]::GetFullPath($AuditDirectory)
    if (Test-Path -LiteralPath $directory) { throw '审计目录已存在，禁止覆盖已有备份与审计' }
    New-Item -ItemType Directory -Path $directory -ErrorAction Stop | Out-Null
    $auditPath = Join-Path $directory 'audit.json'
    $audit = [ordered]@{
        migration   = '2026-11-05-ai-policy-slim.sql'
        startedAt   = (Get-Date).ToUniversalTime().ToString('o')
        sqlSHA256   = (Get-FileHash -LiteralPath $sqlFile -Algorithm SHA256).Hash
        committed   = $false
        commitState = 'not_started'
        stage       = 'preflight'
    }
    if ($env:OS -eq 'Windows_NT') {
        $owner = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        & icacls.exe $directory /inheritance:r /grant:r ($owner + ':(OI)(CI)F') 'SYSTEM:(OI)(CI)F' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw '限制审计目录访问权限失败，未执行迁移' }
    }
    Save-Audit

    $clients = Read-JsonQuery "BEGIN READ ONLY; SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND backend_type='client backend'; COMMIT;"
    if ($clients -ne 0) { throw '数据库仍有客户端连接；关闭 API、Worker 和数据库工具后重试，脚本不会终止连接' }
    $before = Read-Facts
    $audit.before = $before
    Save-Audit
    if ([string]::IsNullOrWhiteSpace($before.policyCheck)) { throw 'AI policy check 缺失' }
    foreach ($key in $retainedKeys) {
        if ($before.policyCheck -notmatch [regex]::Escape($key)) { throw "AI policy check 缺少保留键：$key" }
    }
    if ($before.aiSettingCount -lt $retainedKeys.Count) { throw 'AI 设置数量不足，先执行 AI Core seed' }
    if ($null -eq $before.settingState -or [bigint]$before.settingState.generation -lt 1) { throw 'system.setting/global generation 缺失，先执行系统设置 seed' }

    $audit.stage = 'backup'
    Save-Audit
    $backup = Join-Path $directory 'before.dump'
    $null = & pg_dump --dbname=$dsn --no-password --format=custom --file=$backup 2>&1
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $backup)) { throw '完整数据库备份失败，未执行迁移' }
    $null = & pg_restore --list $backup 2>&1
    if ($LASTEXITCODE -ne 0) { throw '备份归档不可读，未执行迁移' }
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash

    $audit.stage = 'apply'
    $audit.commitState = 'unknown'
    Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    if ($LASTEXITCODE -ne 0) {
        if ($LASTEXITCODE -eq 3) { $audit.commitState = 'rolled_back'; Save-Audit; throw 'AI 策略收敛事务失败并回滚，备份和审计已保留' }
        throw 'PostgreSQL 提交状态未知；保持写入服务停止并先只读核验'
    }
    $committed = $true
    $audit.committed = $true
    $audit.commitState = 'committed'
    $audit.stage = 'verify'
    Save-Audit

    $after = Read-Facts
    $audit.after = $after
    Save-Audit
    if ($after.aiSettingCount -ne $retainedKeys.Count) { throw '迁移后 AI 设置数量不是 5' }
    if ($after.removedCount -ne 0) { throw '迁移后仍存在未删除的旧 AI 设置' }
    Assert-PolicyCheck $after
    Assert-RetainedUnchanged $before $after
    foreach ($field in @('otherSettingsDigest', 'otherCacheDigest', 'menuDigest', 'platformDigest', 'roleMenuDigest', 'aiRunCount')) {
        if ($after.$field -ne $before.$field) { throw "迁移改写了无关事实：$field" }
    }
    $beforeGeneration = [bigint]$before.settingState.generation
    $afterGeneration = [bigint]$after.settingState.generation
    $expectedDelta = 0
    if ($before.removedCount -gt 0) { $expectedDelta = 1 }
    if ($afterGeneration -ne ($beforeGeneration + $expectedDelta)) { throw 'system.setting generation 推进不符合实际删除量' }
    if ($expectedDelta -eq 1) {
        $newOutbox = @($after.settingOutbox | Where-Object { [bigint]$_.generation -eq $afterGeneration })
        if ($newOutbox.Count -ne 1 -or $null -ne $newOutbox[0].published_at -or $newOutbox[0].attempts -ne 0) {
            throw '新代际 outbox 缺失或已被意外发布'
        }
    }
    if (@($after.settingOutbox | Where-Object { [bigint]$_.generation -gt $afterGeneration }).Count -ne 0) { throw 'system.setting outbox 出现未来代际' }

    $audit.stage = 'replay'
    Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'AI 策略收敛幂等复跑失败，保持服务停止并核验' }
    $replay = Read-Facts
    $audit.replay = $replay
    Save-Audit
    if ((Json $after) -ne (Json $replay)) { throw 'AI 策略收敛幂等复跑改变了事实' }

    $audit.stage = 'completed'
    $audit.completedAt = (Get-Date).ToUniversalTime().ToString('o')
    $audit.cacheNote = 'system.setting generation/outbox 已写库；由 Worker 通用 outbox relay 发布，未手工改写 Redis'
    Save-Audit
    Write-Output "AI 策略 26→5、幂等复跑与无关事实核验通过，备份/审计：$directory"
    Write-Output '恢复服务前先启动新版本 Worker/API；旧版本代码不再兼容 5 个设置。'
} catch {
    if ($audit -and $auditPath) {
        $audit.committed = $committed
        $audit.failedAt = (Get-Date).ToUniversalTime().ToString('o')
        try { Save-Audit } catch { Write-Warning '失败审计写入失败，请保留备份目录' }
        if ($audit.commitState -eq 'unknown') { Write-Warning 'PostgreSQL 提交状态未知；保持服务停止并先只读核验' }
        elseif ($committed) { Write-Warning 'PostgreSQL 已提交，后续校验未完成；保持服务停止，保留备份和审计后核验复跑' }
    }
    throw
} finally {
    $env:PGCLIENTENCODING = $savedEncoding
    $env:PGCONNECT_TIMEOUT = $savedTimeout
    $env:PGOPTIONS = $savedOptions
    $dsn = $null
}
