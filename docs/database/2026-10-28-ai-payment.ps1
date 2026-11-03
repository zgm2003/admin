[CmdletBinding()]
param(
    [switch]$ServicesStopped,
    [string]$EnvFile = (Join-Path $PSScriptRoot '../../server/.env'),
    [string]$AuditDirectory = (Join-Path $env:LOCALAPPDATA ('Admin/maintenance/ai-payment/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '-' + [guid]::NewGuid().ToString('N')))
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '停止 API/Worker 及旧账务写入入口后使用 -ServicesStopped 执行' }
foreach ($name in @('psql','pg_dump','pg_restore')) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) { throw "缺少 PostgreSQL 工具：$name" }
}
$envPath = (Resolve-Path -LiteralPath $EnvFile).Path
$lines = @(Get-Content -LiteralPath $envPath | Where-Object { $_ -match '^POSTGRES_DSN=' })
if ($lines.Count -ne 1) { throw '环境文件必须恰好包含一条 POSTGRES_DSN' }
# Keep the connection string in memory. Never print or put it in an audit file.
$dsn = $lines[0].Substring($lines[0].IndexOf('=') + 1).Trim().Trim('"').Trim("'")
if ([string]::IsNullOrWhiteSpace($dsn)) { throw 'POSTGRES_DSN 为空' }
$dsn = $dsn -replace '(?i)\s+TimeZone=(?:''[^'']*''|"[^"]*"|\S+)', ''
$savedEncoding = $env:PGCLIENTENCODING
$savedTimeout = $env:PGCONNECT_TIMEOUT
$savedOptions = $env:PGOPTIONS
$env:PGCLIENTENCODING = 'UTF8'
$env:PGCONNECT_TIMEOUT = '10'
$env:PGOPTIONS = '-c search_path=public -c statement_timeout=60000'
$committed = $false
$audit = $null
$auditPath = $null
$sqlFile = Join-Path $PSScriptRoot '2026-10-28-ai-payment.sql'

function Read-JsonQuery([string]$Query) {
    $output = @(& psql -X --dbname=$dsn --no-password --quiet --tuples-only --no-align --set=ON_ERROR_STOP=1 --command=$Query 2>&1)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读核验失败，未输出连接参数' }
    try { return (($output -join [Environment]::NewLine) | ConvertFrom-Json -ErrorAction Stop) }
    catch { throw 'PostgreSQL 只读核验返回了非法 JSON' }
}

function Read-Facts {
    return Read-JsonQuery @'
BEGIN READ ONLY;
SELECT json_build_object(
 'ledgerCount',(SELECT count(*) FROM payment_transaction),
 'walletCount',(SELECT count(*) FROM payment_wallet),
 'legacyDigest',(SELECT md5(coalesce(string_agg(md5((to_jsonb(t)-'direction'-'balance_before_units'-'ai_usage_charge_id')::text),'' ORDER BY id),'')) FROM payment_transaction t),
 'walletDigest',(SELECT md5(coalesce(string_agg(md5(to_jsonb(w)::text),'' ORDER BY user_id),'')) FROM payment_wallet w),
 'guards',(SELECT count(*) FROM pg_trigger WHERE tgrelid='payment_transaction'::regclass AND NOT tgisinternal AND tgenabled='O' AND tgname IN('payment_transaction_immutable','payment_transaction_no_delete')),
 'extended',(SELECT count(*)=3 FROM pg_attribute WHERE attrelid='payment_transaction'::regclass AND attname IN('direction','balance_before_units','ai_usage_charge_id') AND NOT attisdropped),
 'invalidExtended',(SELECT count(*) FROM payment_transaction t WHERE to_jsonb(t)?'direction' AND (
    (to_jsonb(t)->>'direction')::smallint NOT IN(1,2) OR
    (to_jsonb(t)->>'balance_before_units') IS NULL OR
    CASE (to_jsonb(t)->>'direction')::smallint
      WHEN 1 THEN balance_after_units::numeric<>(to_jsonb(t)->>'balance_before_units')::numeric+amount_units::numeric
      WHEN 2 THEN balance_after_units::numeric<>(to_jsonb(t)->>'balance_before_units')::numeric-amount_units::numeric
      ELSE true END)),
 'cacheDigest',(SELECT md5(coalesce(string_agg(md5(row_value),'' ORDER BY row_value),'')) FROM (
    SELECT to_jsonb(s)::text row_value FROM system_config_cache_state s WHERE namespace IN('system.setting','ai.runtime')
    UNION ALL SELECT to_jsonb(o)::text FROM system_config_cache_outbox o WHERE namespace IN('system.setting','ai.runtime')
 ) cache)
);
COMMIT;
'@
}

function Save-Audit {
    $audit | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $auditPath -Encoding utf8
}

try {
    $clients = Read-JsonQuery "BEGIN READ ONLY; SELECT to_json(count(*)) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND backend_type='client backend'; COMMIT;"
    if ($clients -ne 0) { throw '数据库仍有客户端连接，关闭服务和数据库工具后重试；脚本不会终止连接' }
    $before = Read-Facts
    if ($before.guards -ne 2 -or $before.invalidExtended -ne 0) { throw '迁移前资金事实或不可变保护不符合要求' }
    $directory = [IO.Path]::GetFullPath($AuditDirectory)
    if (Test-Path -LiteralPath $directory) { throw '审计目录已存在，请选择新的目录，禁止覆盖已有审计' }
    New-Item -ItemType Directory -Path $directory -ErrorAction Stop | Out-Null
    $auditPath = Join-Path $directory 'audit.json'
    $audit = [ordered]@{
        migration = '2026-10-28-ai-payment.sql'
        startedAt = (Get-Date).ToUniversalTime().ToString('o')
        sqlSHA256 = (Get-FileHash -LiteralPath $sqlFile -Algorithm SHA256).Hash
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
    # Full custom-format archive includes all FK dependencies and sequence state.
    $null = & pg_dump --dbname=$dsn --no-password --format=custom --file=$backup 2>&1
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $backup)) { throw '备份失败，未执行迁移' }
    $null = & pg_restore --list $backup 2>&1
    if ($LASTEXITCODE -ne 0) { throw '备份归档不可读，未执行迁移' }
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash
    $audit.stage = 'apply'
    $audit.commitState = 'unknown'
    Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    $migrationExitCode = $LASTEXITCODE
    if ($migrationExitCode -ne 0) {
        if ($migrationExitCode -eq 3) {
            $audit.commitState = 'rolled_back'
            Save-Audit
            throw '迁移脚本失败并回滚，备份和审计已保留'
        }
        $audit.commitState = 'unknown'
        Save-Audit
        throw 'PostgreSQL 提交状态未知（连接/客户端错误）；保持服务停止并先只读核验，勿按已回滚处理'
    }
    $committed = $true
    $audit.committed = $true
    $audit.commitState = 'committed'
    $audit.stage = 'verify'
    Save-Audit
    $after = Read-Facts
    $audit['after'] = $after
    Save-Audit
    foreach ($field in @('ledgerCount','walletCount','legacyDigest','walletDigest','cacheDigest')) {
        if ($after.$field -ne $before.$field) { throw "迁移后事实核验失败：$field" }
    }
    if (-not $after.extended -or $after.guards -ne 2 -or $after.invalidExtended -ne 0) { throw '迁移后新流水结构或保护不完整' }
    $audit.stage = 'replay'
    Save-Audit
    $null = & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile 2>&1
    if ($LASTEXITCODE -ne 0) { throw '幂等复跑失败' }
    $audit.stage = 'verify_replay'
    $replay = Read-Facts
    $audit['replay'] = $replay
    Save-Audit
    if (($after | ConvertTo-Json -Compress -Depth 8) -ne ($replay | ConvertTo-Json -Compress -Depth 8)) { throw '幂等复跑改变了事实' }
    $audit['completedAt'] = (Get-Date).ToUniversalTime().ToString('o')
    $audit.stage = 'completed'
    $audit['cacheChanges'] = '本迁移不修改缓存、generation/outbox 或菜单，无 Redis cleanup'
    Save-Audit
    Write-Output "AI 资金流水迁移和幂等复跑通过，备份/审计：$directory"
    Write-Output '部署显式写 direction/balance_before_units 的新代码后再恢复服务；旧写入程序已不兼容。'
} catch {
    if ($audit -and $auditPath) {
        $audit['committed'] = $committed
        $audit['failedAt'] = (Get-Date).ToUniversalTime().ToString('o')
        try { Save-Audit } catch { Write-Warning '失败审计写入失败，请保留备份目录' }
    }
    if ($audit -and $audit.commitState -eq 'unknown') { Write-Warning 'PostgreSQL 提交状态未知；保持服务停止，先做只读结构/事实核验' }
    elseif ($committed) { Write-Warning 'PostgreSQL 已提交，后续校验未完成；保持服务停止，保留备份/审计并检查后安全复跑' }
    throw
} finally {
    $env:PGCLIENTENCODING = $savedEncoding
    $env:PGCONNECT_TIMEOUT = $savedTimeout
    $env:PGOPTIONS = $savedOptions
    $dsn = $null
}
