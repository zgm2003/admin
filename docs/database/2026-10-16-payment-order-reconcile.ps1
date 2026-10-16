param(
    [switch]$CheckOnly,
    [switch]$OldServicesStopped,
    [ValidatePattern('^[a-z][a-z0-9_]{0,62}$')][string]$Schema = 'public',
    [string]$AuditDirectory
)

$ErrorActionPreference = 'Stop'
$sqlPath = Join-Path $PSScriptRoot '2026-10-16-payment-order-reconcile.sql'
$originalAppName = $env:PGAPPNAME
$audit = [ordered]@{
    migration = 'payment-order-reconcile'
    startedAt = [DateTime]::UtcNow.ToString('o')
    postgresCommitState = 'notStarted'
    verified = $false
    checkOnly = [bool]$CheckOnly
}
$auditPath = $null

function Invoke-Query([string]$Query) {
    $prefix = "BEGIN READ ONLY; SET LOCAL search_path TO `"$Schema`",pg_catalog; SET LOCAL statement_timeout='15s';"
    $output = @(& psql -X -w -q -A -t --dbname $script:maintenanceDsn -v ON_ERROR_STOP=1 -v VERBOSITY=sqlstate -c ($prefix + $Query + '; ROLLBACK;') 2>&1)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读查询失败；未输出连接串或原始错误。' }
    return (($output -join "`n") | ConvertFrom-Json)
}

function Read-State {
    return Invoke-Query @'
SELECT jsonb_build_object(
 'schema', current_schema(),
 'missingTables', (SELECT COALESCE(jsonb_agg(name), '[]'::jsonb) FROM unnest(ARRAY[
  'payment_config','payment_config_version','payment_recharge_package',
  'payment_order','payment_wallet','payment_transaction','payment_callback_event'
 ]) AS names(name) WHERE to_regclass(format('%I.%I',current_schema(),name)) IS NULL),
 'targetCount', (SELECT count(*) FROM system_scheduler_schedule WHERE builtin_key='payment.order.reconcile'),
 'validCount', (SELECT count(*) FROM system_scheduler_schedule WHERE builtin_key='payment.order.reconcile'
  AND task_type='payment.order.reconcile' AND params='{}'::jsonb AND deleted_at IS NULL
  AND next_run_at IS NOT NULL AND btrim(cron_expression)<>'' AND btrim(timezone)<>''),
 'targetHash', (SELECT COALESCE(md5(jsonb_agg(to_jsonb(s) ORDER BY id)::text),'empty') FROM system_scheduler_schedule s WHERE builtin_key='payment.order.reconcile'),
 'otherHash', (SELECT COALESCE(md5(jsonb_agg(to_jsonb(s) ORDER BY id)::text),'empty') FROM system_scheduler_schedule s WHERE builtin_key IS DISTINCT FROM 'payment.order.reconcile'),
 'target', (SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'taskType',task_type,'cron',cron_expression,'timezone',timezone,'isEnabled',is_enabled,'nextRunAt',next_run_at) ORDER BY id),'[]'::jsonb) FROM system_scheduler_schedule WHERE builtin_key='payment.order.reconcile')
)
'@
}

function Apply-Seed {
    $prefix = "SET LOCAL search_path TO `"$Schema`",pg_catalog;"
    $output = @(& psql -X -w -q --dbname $script:maintenanceDsn --single-transaction -v ON_ERROR_STOP=1 -v VERBOSITY=sqlstate -c $prefix -f $sqlPath 2>&1)
    if ($LASTEXITCODE -ne 0) { throw '巡检 seed 执行失败或提交结果未确认；保持服务停止后重跑核对，脚本可幂等重放。' }
}

try {
    foreach ($name in @('psql', 'pg_dump', 'pg_restore')) { Get-Command $name -ErrorAction Stop | Out-Null }
    if ([string]::IsNullOrWhiteSpace($env:POSTGRES_DSN)) { throw '请显式设置 POSTGRES_DSN；脚本不输出、不写入审计连接串。' }
    # pgx accepts this server parameter directly; libpq needs it via options.
    # The seed uses absolute TIMESTAMPTZ values and does not depend on session timezone.
    $script:maintenanceDsn = $env:POSTGRES_DSN -replace '(?i)(^|\s)TimeZone=\S+', '$1'
    $env:PGAPPNAME = 'admin-payment-reconcile-maintenance'
    if ([string]::IsNullOrWhiteSpace($AuditDirectory)) {
        $AuditDirectory = Join-Path $env:LOCALAPPDATA ('Admin/maintenance/payment-reconcile/' + [Guid]::NewGuid().ToString('N'))
    }
    $AuditDirectory = [IO.Path]::GetFullPath($AuditDirectory)
    if ((Test-Path -LiteralPath $AuditDirectory) -and @(Get-ChildItem -LiteralPath $AuditDirectory -Force).Count -gt 0) {
        throw '审计目录必须为空；请保留上次审计，使用新的目录重跑。'
    }
    New-Item -ItemType Directory -Path $AuditDirectory -Force | Out-Null
    $auditPath = Join-Path $AuditDirectory 'audit.json'
    $audit.sqlSHA256 = (Get-FileHash -LiteralPath $sqlPath -Algorithm SHA256).Hash
    $audit.before = Read-State
    if ($audit.before.schema -ne $Schema) { throw '数据库 schema 与请求不一致；seed 未执行。' }
    if ($CheckOnly) {
        $audit.before | ConvertTo-Json -Depth 8
        return
    }
    if (-not $OldServicesStopped) { throw '先停止 API/Worker，再传入 -OldServicesStopped；本脚本不会启停服务。' }
    if (@($audit.before.missingTables).Count -ne 0) { throw 'P1/P2 支付表尚未部署；先完成对应结构迁移与验收，巡检 seed 未执行。' }
    $connections = Invoke-Query "SELECT jsonb_build_object('count',count(*)) FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND pid<>pg_backend_pid()"
    if ($Schema -eq 'public' -and $connections.count -ne 0) { throw '业务库仍有其他连接；确认服务和维护会话全部退出后重跑。' }

    $backup = Join-Path $AuditDirectory 'schedule-before.dump'
    $output = @(& pg_dump -w --format=custom --no-owner --no-privileges --dbname $script:maintenanceDsn --table "$Schema.system_scheduler_schedule" --file $backup 2>&1)
    if ($LASTEXITCODE -ne 0) { throw '调度计划备份失败；seed 未执行。' }
    $output = @(& pg_restore --list $backup 2>&1)
    if ($LASTEXITCODE -ne 0) { throw '备份目录校验失败；seed 未执行。' }
    $output = @(& pg_restore --no-owner --no-privileges --file (Join-Path $AuditDirectory 'schedule-before.sql') $backup 2>&1)
    if ($LASTEXITCODE -ne 0) { throw '备份完整读取失败；seed 未执行。' }
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash
    $audit.postgresCommitState = 'unknown'
    $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8
    Apply-Seed
    $audit.postgresCommitState = 'committed'
    $audit.after = Read-State
    if ($audit.after.targetCount -ne 1 -or $audit.after.validCount -ne 1 -or $audit.after.otherHash -ne $audit.before.otherHash) {
        throw 'PostgreSQL 已提交，但目标计划或非目标计划保真校验失败；保持服务停止。'
    }
    Apply-Seed
    $audit.replayed = Read-State
    if ($audit.replayed.targetHash -ne $audit.after.targetHash -or $audit.replayed.otherHash -ne $audit.after.otherHash) {
        throw 'PostgreSQL 已提交，但幂等重放改写了调度事实；保持服务停止。'
    }
    $audit.verified = $true
    Write-Output "巡检 seed 已验证；审计与备份：$AuditDirectory"
}
catch {
    $audit.failed = $true
    # Native tool output/DSN/credentials are deliberately never persisted.
    $audit.failure = $_.Exception.Message
    if ($audit.postgresCommitState -ne 'notStarted') {
        [Console]::Error.WriteLine('PostgreSQL 可能已提交；请保持服务停止并依据审计重跑核对。')
    }
    Write-Error $_.Exception.Message -ErrorAction Continue
    exit 1
}
finally {
    $env:PGAPPNAME = $originalAppName
    $audit.finishedAt = [DateTime]::UtcNow.ToString('o')
    if ($null -ne $auditPath) { $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8 }
}
