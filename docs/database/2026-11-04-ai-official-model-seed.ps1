[CmdletBinding()]
param(
    [switch]$ServicesStopped,
    [string]$EnvFile = (Join-Path $PSScriptRoot '../../server/.env'),
    [string]$AuditDirectory = (Join-Path $env:LOCALAPPDATA ('Admin/maintenance/ai-official-model/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '-' + [guid]::NewGuid().ToString('N')))
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '停止 API/Worker 及其他数据库写入入口后使用 -ServicesStopped 执行' }
foreach ($name in @('psql', 'pg_dump', 'pg_restore')) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) { throw "缺少 PostgreSQL 工具：$name" }
}

$sqlFile = Join-Path $PSScriptRoot '2026-11-04-ai-official-model-seed.sql'
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

function Save-Audit { $audit | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $auditPath -Encoding utf8 }
function Json([object]$Value) { return ConvertTo-Json -InputObject $Value -Depth 20 -Compress }

function Read-JsonQuery([string]$Query) {
    $output = @(& psql -X --dbname=$dsn --no-password --quiet --tuples-only --no-align --set=ON_ERROR_STOP=1 --command=$Query 2>&1)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读核验失败，未输出连接参数' }
    try { return (($output -join [Environment]::NewLine) | ConvertFrom-Json -ErrorAction Stop) }
    catch { throw 'PostgreSQL 只读核验返回了非法 JSON' }
}

function Read-Facts {
    return Read-JsonQuery @'
BEGIN READ ONLY;
SELECT jsonb_build_object(
 'coreTableCount',(SELECT count(*) FROM pg_tables WHERE schemaname=current_schema() AND tablename LIKE 'ai\_%' ESCAPE '\'),
 'coreSchemaStamp',(SELECT obj_description(c.oid,'pg_class') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relname='ai_provider' AND c.relkind='r'),
 'officialModels',(SELECT coalesce(jsonb_agg(to_jsonb(m) ORDER BY m.vendor_code,m.model_key),'[]'::jsonb) FROM ai_official_model m),
 'runtimeState',(SELECT to_jsonb(s) FROM system_config_cache_state s WHERE s.namespace='ai.runtime' AND s.scope_key='global'),
 'runtimeOutbox',(SELECT coalesce(jsonb_agg(to_jsonb(o) ORDER BY o.id),'[]'::jsonb) FROM system_config_cache_outbox o WHERE o.namespace='ai.runtime' AND o.scope_key='global'),
 'otherSettingsDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(s)::text),'' ORDER BY s.id),'')) FROM system_setting s WHERE s.setting_key NOT LIKE 'ai.%'),
 'otherCacheDigest',(SELECT md5(coalesce(string_agg(fact,'' ORDER BY fact),'')) FROM (
    SELECT 'state:'||to_jsonb(s)::text fact FROM system_config_cache_state s WHERE NOT(s.namespace='ai.runtime' AND s.scope_key='global')
    UNION ALL SELECT 'outbox:'||to_jsonb(o)::text FROM system_config_cache_outbox o WHERE NOT(o.namespace='ai.runtime' AND o.scope_key='global')
 ) other_cache),
 'menuDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(m)::text),'' ORDER BY m.id),'')) FROM permission_menu m),
 'platformDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(p)::text),'' ORDER BY p.id),'')) FROM permission_auth_platform p),
 'roleMenuDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(g)::text),'' ORDER BY g.id),'')) FROM permission_role_menu g)
);
COMMIT;
'@
}

function Get-ModelKeySet([object[]]$Rows) {
    return @($Rows | ForEach-Object { "$($_.vendor_code)|$($_.model_key)" })
}

try {
    $directory = [IO.Path]::GetFullPath($AuditDirectory)
    if (Test-Path -LiteralPath $directory) { throw '审计目录已存在，禁止覆盖已有备份与审计' }
    New-Item -ItemType Directory -Path $directory -ErrorAction Stop | Out-Null
    $auditPath = Join-Path $directory 'audit.json'
    $audit = [ordered]@{
        migration       = '2026-11-04-ai-official-model-seed.sql'
        startedAt       = (Get-Date).ToUniversalTime().ToString('o')
        sqlSHA256       = (Get-FileHash -LiteralPath $sqlFile -Algorithm SHA256).Hash
        committed       = $false
        commitState     = 'not_started'
        stage           = 'preflight'
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
    if ($before.coreTableCount -ne 21 -or $before.coreSchemaStamp -notmatch '^admin-ai-core-v1:[a-z0-9]+$') { throw 'AI Core 结构不完整，未执行迁移' }
    if ($null -eq $before.runtimeState -or [bigint]$before.runtimeState.generation -lt 1) { throw 'ai.runtime/global generation 缺失，先执行 AI Core seed' }
    $beforeKeys = @(Get-ModelKeySet @($before.officialModels))
    $audit.beforeModelCount = $beforeKeys.Count
    Save-Audit

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
        if ($LASTEXITCODE -eq 3) { $audit.commitState = 'rolled_back'; Save-Audit; throw '官方模型 seed 事务失败并回滚，备份和审计已保留' }
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
    $afterKeys = @(Get-ModelKeySet @($after.officialModels))
    $inserted = @($afterKeys | Where-Object { $beforeKeys -notcontains $_ })
    if ($afterKeys.Count -ne ($beforeKeys.Count + $inserted.Count)) { throw '官方模型事实差异无法解释' }
    if ($beforeKeys.Count -eq 0) {
        $counts = Read-JsonQuery @'
BEGIN READ ONLY;
SELECT jsonb_build_object(
 'total',(SELECT count(*) FROM ai_official_model),
 'openai',(SELECT count(*) FROM ai_official_model WHERE vendor_code='openai'),
 'anthropic',(SELECT count(*) FROM ai_official_model WHERE vendor_code='anthropic'),
 'chat',(SELECT count(*) FROM ai_official_model WHERE operations=ARRAY['text.generate']::text[]),
 'image',(SELECT count(*) FROM ai_official_model WHERE operations=ARRAY['image.generate']::text[])
);
COMMIT;
'@
        if ($counts.total -ne 24 -or $counts.openai -ne 14 -or $counts.anthropic -ne 10 -or $counts.chat -ne 23 -or $counts.image -ne 1) {
            throw '官方模型目录分布不符合定义'
        }
        $audit.modelDistribution = $counts
    }
    $afterByIdentity = @{}
    foreach ($row in @($after.officialModels)) { $afterByIdentity["$($row.vendor_code)|$($row.model_key)"] = $row }
    foreach ($row in @($before.officialModels)) {
        $key = "$($row.vendor_code)|$($row.model_key)"
        if ($null -eq $afterByIdentity[$key]) { throw '迁移删除了已有官方模型' }
        if ((Json $afterByIdentity[$key]) -ne (Json $row)) { throw '迁移改写了管理员维护的官方模型事实' }
    }
    foreach ($field in @('otherSettingsDigest', 'otherCacheDigest', 'menuDigest', 'platformDigest', 'roleMenuDigest')) {
        if ($after.$field -ne $before.$field) { throw "迁移改写了无关事实：$field" }
    }
    $beforeGeneration = [bigint]$before.runtimeState.generation
    $afterGeneration = [bigint]$after.runtimeState.generation
    $expectedDelta = 0
    if ($inserted.Count -gt 0) { $expectedDelta = 1 }
    if ($afterGeneration -ne ($beforeGeneration + $expectedDelta)) { throw 'ai.runtime generation 推进不符合实际 seed 变更' }
    $currentOutbox = @($after.runtimeOutbox | Where-Object { $_.namespace -eq 'ai.runtime' -and $_.scope_key -eq 'global' -and [bigint]$_.generation -eq $afterGeneration })
    if ($currentOutbox.Count -ne 1) { throw 'ai.runtime 当前代际缺少唯一 outbox' }
    $beforeGenerations = @($before.runtimeOutbox | ForEach-Object { [bigint]$_.generation })
    if ($beforeGenerations -notcontains $afterGeneration) {
        if ($null -ne $currentOutbox[0].published_at -or $currentOutbox[0].attempts -ne 0) { throw '新建 ai.runtime outbox 已被意外发布' }
    }
    if (@($after.runtimeOutbox | Where-Object { [bigint]$_.generation -gt $afterGeneration }).Count -ne 0) { throw 'ai.runtime outbox 出现未来代际' }

    $audit.stage = 'replay'
    Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    if ($LASTEXITCODE -ne 0) { throw '官方模型 seed 幂等复跑失败，保持服务停止并核验' }
    $replay = Read-Facts
    $audit.replay = $replay
    Save-Audit
    if ((Json $after) -ne (Json $replay)) { throw '官方模型 seed 幂等复跑改变了事实' }

    $audit.stage = 'completed'
    $audit.completedAt = (Get-Date).ToUniversalTime().ToString('o')
    $audit.cacheNote = 'ai.runtime generation/outbox 已写库；配置由 Worker 通用 outbox relay 发布，未手工改写 Redis'
    Save-Audit
    Write-Output "AI 官方模型目录 seed、幂等复跑与无关事实核验通过，备份/审计：$directory"
    Write-Output 'API/Worker 恢复后应先由 Worker relay 消化 ai.runtime outbox，再进行 AI 接入配置。'
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
