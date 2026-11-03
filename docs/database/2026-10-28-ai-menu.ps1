[CmdletBinding()]
param(
    [switch]$ServicesStopped,
    [string]$EnvFile = (Join-Path $PSScriptRoot '../../server/.env'),
    [string]$AuditDirectory = (Join-Path $env:LOCALAPPDATA ('Admin/maintenance/ai-menu/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '-' + [guid]::NewGuid().ToString('N')))
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '停止 API/Worker 及权限写入入口后使用 -ServicesStopped 执行' }
foreach ($name in @('psql','pg_dump','pg_restore','go')) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) { throw "缺少维护工具：$name" }
}
$envPath = (Resolve-Path -LiteralPath $EnvFile).Path
$lines = @(Get-Content -LiteralPath $envPath | Where-Object { $_ -match '^POSTGRES_DSN=' })
if ($lines.Count -ne 1) { throw '环境文件必须恰好包含一条 POSTGRES_DSN' }
$dsn = $lines[0].Substring($lines[0].IndexOf('=') + 1).Trim().Trim('"').Trim("'")
if ([string]::IsNullOrWhiteSpace($dsn)) { throw 'POSTGRES_DSN 为空' }
$dsn = $dsn -replace '(?i)\s+TimeZone=(?:''[^'']*''|"[^"]*"|\S+)', ''
$sqlFile = Join-Path $PSScriptRoot '2026-10-28-ai-menu.sql'
$serverDirectory = Join-Path $PSScriptRoot '../../server'
$savedEncoding, $savedTimeout, $savedOptions = $env:PGCLIENTENCODING, $env:PGCONNECT_TIMEOUT, $env:PGOPTIONS
$env:PGCLIENTENCODING = 'UTF8'
$env:PGCONNECT_TIMEOUT = '10'
$env:PGOPTIONS = '-c search_path=public -c statement_timeout=60000'
$audit = $null
$auditPath = $null

function Save-Audit { $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding utf8 }
function Json([object]$Value) { return ConvertTo-Json -InputObject $Value -Depth 10 -Compress }
function Read-MenuState([switch]$Apply) {
    $arguments = @('run','./cmd/ai-menu-state',"--env-file=$envPath")
    if ($Apply) { $arguments += @('--apply','--services-stopped') }
    Push-Location -LiteralPath $serverDirectory
    try {
        $output = @(& go @arguments 2>&1)
        if ($LASTEXITCODE -ne 0) { throw 'Admin 菜单状态检查或发布失败；未输出连接参数' }
        return (($output -join [Environment]::NewLine) | ConvertFrom-Json -ErrorAction Stop)
    } finally { Pop-Location }
}
function Assert-MenuFacts([object]$Before, [object]$After) {
    if ($After.platformId -ne $Before.platformId -or $After.nodes -ne 45 -or $After.directories -ne 1 -or $After.pages -ne 6 -or $After.actions -ne 38) { throw 'Admin AI 菜单结构不完整' }
    $expectedVersion = [bigint]$Before.version
    if ($Before.nodes -eq 0) { $expectedVersion = $expectedVersion + 1 }
    if ([bigint]$After.version -ne $expectedVersion) { throw 'Admin 菜单版本变化不符合真实 seed 结果' }
    foreach ($field in @('otherMenuDigest','otherPlatformDigest','roleMenuDigest')) {
        if ($After.$field -ne $Before.$field) { throw "迁移改写了无关菜单、平台或角色授权：$field" }
    }
    if ($Before.nodes -eq 45 -and $After.menuDigest -ne $Before.menuDigest) { throw '重复执行改写了已有 AI 菜单' }
}

try {
    $directory = [IO.Path]::GetFullPath($AuditDirectory)
    if (Test-Path -LiteralPath $directory) { throw '审计目录已存在，禁止覆盖已有备份与审计' }
    New-Item -ItemType Directory -Path $directory -ErrorAction Stop | Out-Null
    $auditPath = Join-Path $directory 'audit.json'
    $audit = [ordered]@{migration='2026-10-28-ai-menu.sql';startedAt=(Get-Date).ToUniversalTime().ToString('o');sqlSHA256=(Get-FileHash -LiteralPath $sqlFile -Algorithm SHA256).Hash;committed=$false;commitState='not_started';redisPublished=$false;stage='permissions'}
    if ($env:OS -eq 'Windows_NT') {
        $owner = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        & icacls.exe $directory /inheritance:r /grant:r ($owner + ':(OI)(CI)F') 'SYSTEM:(OI)(CI)F' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw '限制备份目录访问权限失败，未执行迁移' }
    }
    Save-Audit
    $audit.stage = 'preflight'; Save-Audit
    $clients = @(& psql -X --dbname=$dsn --no-password --quiet --tuples-only --no-align --set=ON_ERROR_STOP=1 --command="BEGIN READ ONLY; SELECT to_json(count(*)) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND backend_type='client backend'; COMMIT;" 2>&1)
    if ($LASTEXITCODE -ne 0) { throw '数据库只读连接检查失败，未输出连接参数' }
    if (($clients -join '') | ConvertFrom-Json) { throw '数据库仍有客户端连接；关闭服务和数据库工具后重试，脚本不会终止连接' }
    $before = Read-MenuState
    $audit.before = $before; Save-Audit
    if ($before.facts.nodes -notin @(0,45)) { throw 'AI 菜单已有部分或冲突节点，未执行迁移' }
    if ($before.redisFound -and ($before.redisState -ne 'ready' -or [bigint]$before.redisVersion -gt [bigint]$before.facts.version)) { throw 'Redis 菜单处于修改中或版本领先，未执行迁移' }
    $audit.stage = 'backup'; Save-Audit
    $backup = Join-Path $directory 'before.dump'
    $null = & pg_dump --dbname=$dsn --no-password --format=custom --file=$backup 2>&1
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $backup)) { throw '备份失败，未执行迁移' }
    $null = & pg_restore --list $backup 2>&1
    if ($LASTEXITCODE -ne 0) { throw '备份归档不可读，未执行迁移' }
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash
    $audit.stage = 'apply'; $audit.commitState = 'unknown'; Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    if ($LASTEXITCODE -ne 0) {
        if ($LASTEXITCODE -eq 3) { $audit.commitState='rolled_back'; Save-Audit; throw '菜单事务失败并回滚，备份和审计已保留' }
        throw 'PostgreSQL 提交状态未知（连接/客户端错误）；保持服务停止，先只读核验'
    }
    $audit.committed = $true; $audit.commitState = 'committed'; $audit.stage = 'verify'; Save-Audit
    $after = Read-MenuState
    $audit.after = $after; Save-Audit
    Assert-MenuFacts $before.facts $after.facts
    $audit.stage = 'replay'; Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    if ($LASTEXITCODE -ne 0) { throw '菜单幂等复跑未完成，保持服务停止并核验' }
    $replay = Read-MenuState
    $audit.replay = $replay; Save-Audit
    Assert-MenuFacts $after.facts $replay.facts
    if ((Json $after.facts) -ne (Json $replay.facts)) { throw '菜单幂等复跑改变了事实' }
    $audit.stage = 'publish'; Save-Audit
    $published = Read-MenuState -Apply
    if ($published.dryRun -or -not $published.redisFound -or $published.redisState -ne 'ready' -or [bigint]$published.redisVersion -ne [bigint]$replay.facts.version -or (Json $published.facts) -ne (Json $replay.facts)) { throw 'Admin 菜单 Redis 发布结果不一致' }
    $audit.publication = $published; $audit.redisPublished = $true
    $audit.stage = 'completed'; $audit.completedAt = (Get-Date).ToUniversalTime().ToString('o'); Save-Audit
    Write-Output "AI 菜单迁移、幂等复跑及 Admin 菜单版本发布通过，备份/审计：$directory"
    Write-Output '未授予角色、未启用调度任务、未启停服务；确认新版 API/Worker 和六个页面已部署后由维护者恢复服务。'
} catch {
    if ($audit -and $auditPath) {
        $audit.failedAt = (Get-Date).ToUniversalTime().ToString('o')
        try { Save-Audit } catch { Write-Warning '失败审计写入失败，请保留备份目录' }
        if ($audit.commitState -eq 'unknown') { Write-Warning 'PostgreSQL 提交状态未知；保持服务停止，先只读核验，不按已回滚处理' }
        elseif ($audit.committed) { Write-Warning 'PostgreSQL 已提交，后续核验或 Redis 发布未完成；保持服务停止，保留审计后可安全复跑' }
    }
    throw
} finally {
    $env:PGCLIENTENCODING = $savedEncoding
    $env:PGCONNECT_TIMEOUT = $savedTimeout
    $env:PGOPTIONS = $savedOptions
    $dsn = $null
}
