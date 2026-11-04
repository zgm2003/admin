[CmdletBinding()]
param(
    [switch]$ServicesStopped,
    [string]$EnvFile = (Join-Path $PSScriptRoot '../../server/.env'),
    [string]$AuditDirectory = (Join-Path $env:LOCALAPPDATA ('Admin/maintenance/ai-menu-presentation/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '-' + [guid]::NewGuid().ToString('N')))
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '停止 API/Worker 及其他权限写入入口后使用 -ServicesStopped 执行' }
foreach ($name in @('psql','pg_dump','pg_restore','go')) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) { throw "缺少维护工具：$name" }
}

$envPath = (Resolve-Path -LiteralPath $EnvFile).Path
$lines = @(Get-Content -LiteralPath $envPath | Where-Object { $_ -match '^POSTGRES_DSN=' })
if ($lines.Count -ne 1) { throw '环境文件必须恰好包含一条 POSTGRES_DSN' }
$dsn = $lines[0].Substring($lines[0].IndexOf('=') + 1).Trim().Trim('"').Trim("'")
if ([string]::IsNullOrWhiteSpace($dsn)) { throw 'POSTGRES_DSN 为空' }
$dsn = $dsn -replace '(?i)\s+TimeZone=(?:''[^'']*''|"[^"]*"|\S+)', ''
$sqlFile = Join-Path $PSScriptRoot '2026-11-04-ai-menu-presentation.sql'
$serverDirectory = Join-Path $PSScriptRoot '../../server'
$savedEncoding, $savedTimeout, $savedOptions = $env:PGCLIENTENCODING, $env:PGCONNECT_TIMEOUT, $env:PGOPTIONS
$env:PGCLIENTENCODING = 'UTF8'
$env:PGCONNECT_TIMEOUT = '10'
$env:PGOPTIONS = '-c search_path=public -c statement_timeout=60000'
$audit = $null
$auditPath = $null

function Save-Audit { $audit | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $auditPath -Encoding utf8 }
function Json([object]$Value) { return ConvertTo-Json -InputObject $Value -Depth 12 -Compress }

function Read-JsonQuery([string]$Query) {
    $output = @(& psql -X --dbname=$dsn --no-password --quiet --tuples-only --no-align --set=ON_ERROR_STOP=1 --command=$Query 2>&1)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读菜单核验失败，未输出连接参数' }
    try { return (($output -join [Environment]::NewLine) | ConvertFrom-Json -ErrorAction Stop) }
    catch { throw 'PostgreSQL 菜单核验返回非法 JSON' }
}

function Read-MenuState([switch]$Apply) {
    $arguments = @('run','./cmd/ai-menu-state',"--env-file=$envPath")
    if ($Apply) { $arguments += @('--apply','--services-stopped') }
    Push-Location -LiteralPath $serverDirectory
    try {
        $output = @(& go @arguments 2>&1)
        if ($LASTEXITCODE -ne 0) { throw 'Admin 菜单版本检查或 Redis 发布失败；未输出连接参数' }
        try { return (($output -join [Environment]::NewLine) | ConvertFrom-Json -ErrorAction Stop) }
        catch { throw 'Admin 菜单状态命令返回非法 JSON' }
    } finally { Pop-Location }
}

function Read-PresentationShape {
    return Read-JsonQuery @'
BEGIN READ ONLY;
SELECT coalesce(jsonb_agg(jsonb_build_object('code',m.code,'icon',m.icon,'sortOrder',m.sort_order) ORDER BY m.sort_order,m.id),'[]'::jsonb)
FROM permission_menu m
JOIN permission_auth_platform p ON p.id=m.platform_id
WHERE p.code='admin' AND p.status=1 AND p.deleted_at IS NULL
  AND m.deleted_at IS NULL
  AND (m.code='ai' OR m.code IN('ai:officialModel:view','ai:provider:view','ai:agent:view','ai:chat:view','ai:run:view','ai:tool:view'));
COMMIT;
'@
}

$expectedShape = @(
    [pscustomobject]@{code='ai:officialModel:view';icon='lucide:brain-circuit';sortOrder=10},
    [pscustomobject]@{code='ai:provider:view';icon='lucide:server';sortOrder=20},
    [pscustomobject]@{code='ai:agent:view';icon='lucide:bot';sortOrder=30},
    [pscustomobject]@{code='ai:chat:view';icon='lucide:message-square-more';sortOrder=40},
    [pscustomobject]@{code='ai:run:view';icon='lucide:activity';sortOrder=50},
    [pscustomobject]@{code='ai:tool:view';icon='lucide:settings-2';sortOrder=60},
    [pscustomobject]@{code='ai';icon='lucide:sparkles';sortOrder=900}
)

try {
    $directory = [IO.Path]::GetFullPath($AuditDirectory)
    if (Test-Path -LiteralPath $directory) { throw '审计目录已存在，禁止覆盖已有备份与审计' }
    New-Item -ItemType Directory -Path $directory -ErrorAction Stop | Out-Null
    $auditPath = Join-Path $directory 'audit.json'
    $audit = [ordered]@{migration='2026-11-04-ai-menu-presentation.sql';startedAt=(Get-Date).ToUniversalTime().ToString('o');sqlSHA256=(Get-FileHash -LiteralPath $sqlFile -Algorithm SHA256).Hash;committed=$false;commitState='not_started';redisPublished=$false;stage='permissions'}
    if ($env:OS -eq 'Windows_NT') {
        $owner = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        & icacls.exe $directory /inheritance:r /grant:r ($owner + ':(OI)(CI)F') 'SYSTEM:(OI)(CI)F' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw '限制审计目录访问权限失败，未执行迁移' }
    }
    Save-Audit

    $audit.stage = 'preflight'; Save-Audit
    $clients = Read-JsonQuery "BEGIN READ ONLY; SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND backend_type='client backend'; COMMIT;"
    if ($clients -ne 0) { throw '数据库仍有客户端连接；关闭 API、Worker 和数据库工具后重试，脚本不会终止连接' }
    $before = Read-MenuState
    $beforeShape = @(Read-PresentationShape)
    $audit.before = $before; $audit.beforeShape = $beforeShape; Save-Audit
    if ($before.facts.nodes -ne 45 -or $before.facts.directories -ne 1 -or $before.facts.pages -ne 6 -or $before.facts.actions -ne 38) { throw 'AI 菜单结构不完整，未执行迁移' }
    if (-not $before.redisFound -or $before.redisState -ne 'ready') { throw 'Redis Admin 菜单状态缺失或正在修改，未执行迁移' }
    $beforeMatchesExpected = (Json $beforeShape) -eq (Json $expectedShape)
    $databaseVersion = [bigint]$before.facts.version
    $redisVersion = [bigint]$before.redisVersion
    $recoveryPublishOnly = $beforeMatchesExpected -and $databaseVersion -gt 1 -and [bigint]::Add($redisVersion, [bigint]1) -eq $databaseVersion
    if ($redisVersion -ne $databaseVersion -and -not $recoveryPublishOnly) { throw 'Redis Admin 菜单版本不一致或正在修改，未执行迁移' }
    $audit.recoveryPublishOnly = $recoveryPublishOnly; Save-Audit

    $audit.stage = 'backup'; Save-Audit
    $backup = Join-Path $directory 'before.dump'
    $null = & pg_dump --dbname=$dsn --no-password --format=custom --file=$backup 2>&1
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $backup)) { throw '完整数据库备份失败，未执行迁移' }
    $null = & pg_restore --list $backup 2>&1
    if ($LASTEXITCODE -ne 0) { throw '备份归档不可读，未执行迁移' }
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash

    $audit.stage = 'apply'; $audit.commitState = 'unknown'; Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    if ($LASTEXITCODE -ne 0) {
        if ($LASTEXITCODE -eq 3) { $audit.commitState='rolled_back'; Save-Audit; throw '菜单 presentation 事务失败并回滚，备份和审计已保留' }
        throw 'PostgreSQL 提交状态未知；保持写入服务停止并先只读核验'
    }

    $audit.committed = $true; $audit.commitState = 'committed'; $audit.stage = 'verify'; Save-Audit
    $after = Read-MenuState
    $afterShape = @(Read-PresentationShape)
    $audit.after = $after; $audit.afterShape = $afterShape; Save-Audit
    if ($after.facts.platformId -ne $before.facts.platformId -or $after.facts.nodes -ne 45 -or
        $after.facts.directories -ne 1 -or $after.facts.pages -ne 6 -or $after.facts.actions -ne 38 -or
        (Json $afterShape) -ne (Json $expectedShape)) { throw 'AI 菜单图标或顺序核验失败' }
    foreach ($field in @('otherMenuDigest','otherPlatformDigest','roleMenuDigest')) {
        if ($after.facts.$field -ne $before.facts.$field) { throw "迁移改写无关权限事实：$field" }
    }
    $expectedVersion = [bigint]$before.facts.version
    if ((Json $beforeShape) -ne (Json $expectedShape)) { $expectedVersion = [bigint]::Add($expectedVersion, [bigint]1) }
    if ([bigint]$after.facts.version -ne $expectedVersion) { throw 'Admin menu_version 变化不符合迁移实际修改' }

    $audit.stage = 'replay'; Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    if ($LASTEXITCODE -ne 0) { throw '菜单 presentation 幂等复跑失败，保持服务停止并核验' }
    $replay = Read-MenuState
    $replayShape = @(Read-PresentationShape)
    $audit.replay = $replay; $audit.replayShape = $replayShape; Save-Audit
    if ((Json $replay.facts) -ne (Json $after.facts) -or (Json $replayShape) -ne (Json $expectedShape)) { throw '菜单 presentation 幂等复跑改变了事实' }

    $audit.stage = 'publish'; Save-Audit
    $published = Read-MenuState -Apply
    if ($published.dryRun -or -not $published.redisFound -or $published.redisState -ne 'ready' -or
        [bigint]$published.redisVersion -ne [bigint]$replay.facts.version -or
        (Json $published.facts) -ne (Json $replay.facts)) { throw 'Admin Redis 菜单发布未能核验' }
    $audit.publication = $published; $audit.redisPublished = $true
    $audit.stage = 'completed'; $audit.completedAt = (Get-Date).ToUniversalTime().ToString('o'); Save-Audit
    Write-Output "AI 菜单图标和顺序迁移、幂等复跑及 Redis 版本发布通过，备份/审计：$directory"
} catch {
    if ($audit -and $auditPath) {
        $audit.failedAt = (Get-Date).ToUniversalTime().ToString('o')
        try { Save-Audit } catch { Write-Warning '失败审计写入失败，请保留备份目录' }
        if ($audit.commitState -eq 'unknown') { Write-Warning 'PostgreSQL 提交状态未知；保持服务停止并只读核验' }
        elseif ($audit.committed) { Write-Warning 'PostgreSQL 已提交，Redis 或后续核验未完成；保持服务停止，检查审计并安全重试' }
    }
    throw
} finally {
    $env:PGCLIENTENCODING = $savedEncoding
    $env:PGCONNECT_TIMEOUT = $savedTimeout
    $env:PGOPTIONS = $savedOptions
    $dsn = $null
}
