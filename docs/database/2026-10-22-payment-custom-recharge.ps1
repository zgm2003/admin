[CmdletBinding()]
param(
    [switch]$ServicesStopped,
    [switch]$AllowIdleNavicat,
    [string]$EnvFile = (Join-Path $PSScriptRoot '../../server/.env')
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '停止 API/Worker 后使用 -ServicesStopped 执行本迁移' }
foreach ($tool in @('psql', 'pg_dump', 'pg_restore')) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "缺少 PostgreSQL 工具：$tool" }
}
$envPath = (Resolve-Path -LiteralPath $EnvFile).Path
$line = Get-Content -LiteralPath $envPath | Where-Object { $_ -match '^POSTGRES_DSN=' } | Select-Object -First 1
if (-not $line) { throw '环境文件缺少 POSTGRES_DSN' }
# 不打印或持久化 DSN；libpq 不接受 GORM 的 TimeZone 参数。
$dsn = $line.Substring($line.IndexOf('=') + 1).Trim().Trim('"').Trim("'")
$dsn = $dsn -replace '(?i)\s+TimeZone=(?:''[^'']*''|"[^"]*"|\S+)', ''
$oldEncoding = $env:PGCLIENTENCODING
$oldConnectTimeout = $env:PGCONNECT_TIMEOUT
$env:PGCLIENTENCODING = 'UTF8'
$env:PGCONNECT_TIMEOUT = '10'
$audit = $null
$committed = $false

function Read-JsonQuery([string]$Query) {
    $output = & psql -X --dbname=$dsn --no-password --tuples-only --no-align --set=ON_ERROR_STOP=1 --command=$Query
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读核验失败' }
    return ($output -join "`n" | ConvertFrom-Json)
}

function Read-Facts {
    return Read-JsonQuery @'
SELECT json_build_object(
 'orders',(SELECT count(*) FROM payment_order),
 'financialDigest',(SELECT md5(COALESCE(string_agg(to_jsonb(p)::text,E'\n' ORDER BY id),'')) FROM payment_order p),
 'generation',(SELECT generation FROM system_config_cache_state WHERE namespace='system.setting' AND scope_key='global'),
 'outbox',(SELECT count(*) FROM system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global'),
 'seedCount',(SELECT count(*) FROM system_setting WHERE setting_key='payment.recharge.minimum_amount'),
 'minimum',(SELECT value FROM system_setting WHERE setting_key='payment.recharge.minimum_amount' AND deleted_at IS NULL),
 'packageNullable',(SELECT NOT attnotnull FROM pg_attribute WHERE attrelid='payment_order'::regclass AND attname='package_id'),
 'packageForeignKey',(SELECT count(*) FROM pg_constraint WHERE conrelid='payment_order'::regclass AND conname='fk_payment_order_package' AND confrelid='payment_recharge_package'::regclass AND convalidated),
 'guards',(SELECT count(*) FROM pg_trigger WHERE tgrelid='payment_order'::regclass AND NOT tgisinternal AND tgenabled='O' AND tgname IN ('payment_order_identity','payment_order_no_delete')),
 'minimumCheck',(SELECT count(*) FROM pg_constraint WHERE conrelid='system_setting'::regclass AND conname='ck_system_setting_recharge_minimum_amount' AND convalidated)
);
'@
}

try {
    $allowedClients = if ($AllowIdleNavicat) { "AND NOT (application_name='Navicat' AND state='idle' AND xact_start IS NULL)" } else { '' }
    $clients = Read-JsonQuery "SELECT to_json(count(*)) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND backend_type='client backend' $allowedClients;"
    if ($clients -ne 0) { throw '数据库还有客户端连接；关闭 API/Worker、数据库工具后重试，脚本不会终止其他连接' }
    $before = Read-Facts
    $directory = Join-Path $env:LOCALAPPDATA ('Admin/backups/payment-custom-recharge-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $directory | Out-Null
    $backup = Join-Path $directory 'before.dump'
    & pg_dump --dbname=$dsn --no-password --format=custom --file=$backup --table=public.payment_order --table=public.system_setting --table=public.system_config_cache_state --table=public.system_config_cache_outbox
    if ($LASTEXITCODE -ne 0) { throw '备份失败，未执行迁移' }
    & pg_restore --list $backup | Out-Null
    if ($LASTEXITCODE -ne 0) { throw '备份归档不可读，未执行迁移' }
    $auditPath = Join-Path $directory 'audit.json'
    $audit = [ordered]@{ startedAt = (Get-Date).ToUniversalTime().ToString('o'); migration = '2026-10-22-payment-custom-recharge.sql'; backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash; allowIdleNavicat = [bool]$AllowIdleNavicat; before = $before; committed = $false }
    $audit | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $auditPath -Encoding utf8
    $sqlFile = Join-Path $PSScriptRoot '2026-10-22-payment-custom-recharge.sql'
    & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile
    if ($LASTEXITCODE -ne 0) { throw '迁移事务失败并回滚；备份与审计已保留' }
    $committed = $true
    $audit.committed = $true
    $audit | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $auditPath -Encoding utf8
    $after = Read-Facts
    if ($after.financialDigest -ne $before.financialDigest -or $after.orders -ne $before.orders -or -not $after.packageNullable -or $after.packageForeignKey -ne 1 -or $after.guards -ne 2 -or $after.minimumCheck -ne 1 -or $after.seedCount -ne 1) { throw '迁移后事实核验失败' }
    $expectedIncrease = if ($before.seedCount -eq 0) { 1 } else { 0 }
    if ($after.generation -ne ($before.generation + $expectedIncrease) -or $after.outbox -ne ($before.outbox + $expectedIncrease)) { throw '配置 generation/outbox 增量不符合预期' }
    & psql -X --dbname=$dsn --no-password --single-transaction --set=ON_ERROR_STOP=1 --file=$sqlFile
    if ($LASTEXITCODE -ne 0) { throw '幂等复跑失败' }
    $replay = Read-Facts
    if (($after | ConvertTo-Json -Compress) -ne ($replay | ConvertTo-Json -Compress)) { throw '幂等复跑改变了事实' }
    $audit['after'] = $after
    $audit['replay'] = $replay
    $audit['completedAt'] = (Get-Date).ToUniversalTime().ToString('o')
    $audit['cacheRecovery'] = '恢复 Worker 后由既有 outbox relay 发布 system.setting/global 新代际；未删除 Redis key'
    $audit | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $auditPath -Encoding utf8
    Write-Output "迁移与幂等复跑通过，历史订单保持不变。备份/审计：$directory"
    Write-Output '部署新代码后恢复 Worker/API，确认 outbox 已发布、充值选项可读取，再开放自定义充值；不要回滚到不支持 NULL package_id 的旧代码。'
} catch {
    if ($audit -and $auditPath) {
        $audit['committed'] = $committed
        $audit['failedAt'] = (Get-Date).ToUniversalTime().ToString('o')
        $audit | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $auditPath -Encoding utf8
    }
    if ($committed) { Write-Error 'PostgreSQL 已提交，但后续检查未完成；保留备份/审计并检查后安全复跑，不得假定已回滚' -ErrorAction Continue }
    throw
} finally {
    $env:PGCLIENTENCODING = $oldEncoding
    $env:PGCONNECT_TIMEOUT = $oldConnectTimeout
    $dsn = $null
}
