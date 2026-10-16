[CmdletBinding()]
param(
    [switch]$CheckOnly,
    [switch]$ServicesStopped,
    [ValidatePattern('^[a-z][a-z0-9_]{0,62}$')][string]$Schema = 'public',
    [string]$AuditDirectory
)

$ErrorActionPreference = 'Stop'
$sqlPath = Join-Path $PSScriptRoot '2026-10-17-payment-redeem-code.sql'
$oldAppName = $env:PGAPPNAME
$committed = $false
$auditPath = $null
$audit = $null

function Read-EnvFile([string]$Path) {
    $result = @{}
    if (-not (Test-Path -LiteralPath $Path)) { return $result }
    foreach ($raw in Get-Content -LiteralPath $Path -Encoding UTF8) {
        $line = $raw.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        $i = $line.IndexOf('=')
        if ($i -gt 0) { $result[$line.Substring(0,$i).Trim()] = $line.Substring($i + 1).Trim().Trim('"').Trim("'") }
    }
    return $result
}
function Effective([hashtable]$FileValues, [string]$Key) {
    $v = [Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($v)) { return $v }
    if ($FileValues.ContainsKey($Key)) { return [string]$FileValues[$Key] }
    return ''
}
function Query([string]$Sql) {
    $prefix = "BEGIN READ ONLY; SET LOCAL search_path TO `"$Schema`",pg_catalog; SET LOCAL statement_timeout='15s';"
    $output = @(& psql -X -w -q -A -t --dbname $script:dsn -v ON_ERROR_STOP=1 -c ($prefix + $Sql + '; ROLLBACK;') 2>&1)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读核对失败。' }
    return (($output -join "`n") | ConvertFrom-Json)
}
function State {
    $objects = Query "SELECT jsonb_build_object('schema',current_schema(),'p2Exists',to_regclass(format('%I.payment_transaction',current_schema())) IS NOT NULL,'batchExists',to_regclass(format('%I.payment_redeem_code_batch',current_schema())) IS NOT NULL,'codeExists',to_regclass(format('%I.payment_redeem_code',current_schema())) IS NOT NULL)"
    if (-not $objects.p2Exists) { throw 'P2 流水表尚未部署；未执行 P4 migration。' }
    if ($objects.batchExists -ne $objects.codeExists) { throw 'P4 结构不完整；未执行 migration。' }
    $batchFacts = "'0'"
    $codeFacts = "'0'"
    if ($objects.batchExists) {
        $batchFacts = "(SELECT count(*)::text||':'||COALESCE(sum(('x'||substr(md5(to_jsonb(b)::text),1,15))::bit(60)::bigint)::text,'0') FROM payment_redeem_code_batch b)"
        $codeFacts = "(SELECT count(*)::text||':'||COALESCE(sum(('x'||substr(md5(to_jsonb(c)::text),1,15))::bit(60)::bigint)::text,'0') FROM payment_redeem_code c)"
    }
    $facts = Query @"
SELECT jsonb_build_object(
 'schema',current_schema(),
 'batchExists',$($objects.batchExists.ToString().ToLowerInvariant()),
 'batchFacts',$batchFacts,
 'codeFacts',$codeFacts,
 'transactionFacts',(SELECT count(*)::text||':'||COALESCE(sum(('x'||substr(md5(to_jsonb(t)::text),1,15))::bit(60)::bigint)::text,'0') FROM payment_transaction t),
 'sourceConstraint',(SELECT count(*) FROM pg_constraint WHERE conrelid='payment_transaction'::regclass AND conname='ck_payment_transaction_source_xor'),
 'schemaDigest',md5(jsonb_build_object(
   'constraints',(SELECT jsonb_agg(jsonb_build_object('table',c.conrelid::regclass::text,'name',c.conname,'def',pg_get_constraintdef(c.oid)) ORDER BY c.conrelid,c.conname) FROM pg_constraint c WHERE c.conrelid IN (to_regclass('payment_transaction'),to_regclass('payment_redeem_code_batch'),to_regclass('payment_redeem_code'))),
   'indexes',(SELECT jsonb_agg(indexdef ORDER BY indexname) FROM pg_indexes WHERE schemaname=current_schema() AND tablename IN ('payment_transaction','payment_redeem_code_batch','payment_redeem_code')),
   'triggers',(SELECT jsonb_agg(pg_get_triggerdef(oid) ORDER BY tgrelid,tgname) FROM pg_trigger WHERE NOT tgisinternal AND tgrelid IN (to_regclass('payment_transaction'),to_regclass('payment_redeem_code_batch'),to_regclass('payment_redeem_code'))),
   'functions',(SELECT jsonb_agg(pg_get_functiondef(oid) ORDER BY proname) FROM pg_proc WHERE oid IN (to_regprocedure('payment_redeem_batch_guard()'),to_regprocedure('payment_redeem_code_guard()'),to_regprocedure('payment_transaction_immutable_guard()')))
 )::text)
)
"@
    return $facts
}
function Apply-Migration {
    $prefix = "SET LOCAL search_path TO `"$Schema`",pg_catalog;"
    $output = @(& psql -X -w -q --dbname $script:dsn --single-transaction -v ON_ERROR_STOP=1 -v VERBOSITY=sqlstate -c $prefix -f $sqlPath 2>&1)
    if ($LASTEXITCODE -ne 0) { throw '兑换码 migration 失败或提交结果未确认；保持服务停止后重跑核对。' }
}

try {
    foreach ($tool in @('psql','pg_dump','pg_restore')) { Get-Command $tool -ErrorAction Stop | Out-Null }
    $values = Read-EnvFile (Join-Path $PSScriptRoot '..\..\server\.env')
    $script:dsn = Effective $values 'POSTGRES_DSN'
    if ([string]::IsNullOrWhiteSpace($script:dsn)) { throw '缺少 POSTGRES_DSN。' }
    if ($script:dsn -match '^\s*postgres(?:ql)?://') {
        $script:dsn = $script:dsn -replace '(?i)([?&])TimeZone=[^&]*', '$1' -replace '\?&', '?' -replace '[?&]$', ''
    } else { $script:dsn = ($script:dsn -replace '(?i)(^|\s+)TimeZone=\S+', '$1').Trim() }
    if (-not (Test-Path -LiteralPath $sqlPath)) { throw '找不到兑换码 migration SQL。' }
    $env:PGAPPNAME = 'admin-payment-redeem-maintenance'
    if ([string]::IsNullOrWhiteSpace($AuditDirectory)) { $AuditDirectory = Join-Path $env:LOCALAPPDATA ('Admin/maintenance/payment-redeem/' + [Guid]::NewGuid().ToString('N')) }
    $AuditDirectory = [IO.Path]::GetFullPath($AuditDirectory)
    if ((Test-Path -LiteralPath $AuditDirectory) -and @(Get-ChildItem -LiteralPath $AuditDirectory -Force).Count -gt 0) { throw '审计目录必须为空。' }
    New-Item -ItemType Directory -Path $AuditDirectory -Force | Out-Null
    $auditPath = Join-Path $AuditDirectory 'audit.json'
    $audit = [ordered]@{ migration='payment-redeem-code'; startedAt=[DateTime]::UtcNow.ToString('o'); checkOnly=[bool]$CheckOnly; sqlSHA256=(Get-FileHash -LiteralPath $sqlPath -Algorithm SHA256).Hash; postgresCommitState='notStarted' }
    $audit.before = State
    if ($audit.before.schema -ne $Schema) { throw '目标 schema 不存在或与请求不一致；未执行 migration。' }
    if ($CheckOnly) { $audit.verified = $true; $audit | ConvertTo-Json -Depth 10; return }
    if (-not $ServicesStopped) { throw '请先停止 API/Worker，再传入 -ServicesStopped。' }
    $connections = Query "SELECT jsonb_build_object('count',count(*)) FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND pid<>pg_backend_pid()"
    if ($Schema -eq 'public' -and $connections.count -ne 0) { throw '业务库仍有其他客户端连接；请保持服务停止后重跑。' }
    $backup = Join-Path $AuditDirectory 'public-before.dump'
    & pg_dump --format=custom --no-owner --no-privileges --dbname $script:dsn --schema $Schema --file $backup 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 备份失败；未执行 migration。' }
    & pg_restore --list $backup 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw '备份归档校验失败；未执行 migration。' }
    $output = @(& pg_restore --no-owner --no-privileges --file (Join-Path $AuditDirectory 'schema-before.sql') $backup 2>&1)
    if ($LASTEXITCODE -ne 0) { throw '备份完整读取校验失败；未执行 migration。' }
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash
    $audit.postgresCommitState = 'unknown'
    $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8
    Apply-Migration
    $committed = $true
    $audit.postgresCommitState = 'committed'
    $audit.after = State
    if (-not $audit.after.batchExists -or $audit.after.sourceConstraint -ne 1 -or $audit.after.transactionFacts -ne $audit.before.transactionFacts) { throw 'PostgreSQL 已提交，但 P4 结构或原流水事实核对失败。' }
    if ($audit.before.batchExists -and ($audit.after.batchFacts -ne $audit.before.batchFacts -or $audit.after.codeFacts -ne $audit.before.codeFacts)) { throw 'PostgreSQL 已提交，但原兑换码事实核对失败。' }
    $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8
    Apply-Migration
    $audit.replayed = State
    if (($audit.replayed | ConvertTo-Json -Compress) -ne ($audit.after | ConvertTo-Json -Compress)) { throw 'PostgreSQL 已提交，但幂等重放改写了结构或资金事实。' }
    $audit.verified = $true
    Write-Output "兑换码结构迁移已验证；审计目录：$AuditDirectory"
}
catch {
    if ($null -ne $auditPath) {
        if ($null -eq $audit) { $audit = [ordered]@{ migration='payment-redeem-code' } }
        $audit.failed = $true; $audit.error = $_.Exception.Message
        $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8
    }
    if ($committed) { [Console]::Error.WriteLine('PostgreSQL 已提交；保持服务停止并依据审计重跑核对。') }
    throw
}
finally {
    $env:PGAPPNAME = $oldAppName
    if ($null -ne $auditPath -and $null -ne $audit) {
        $audit.finishedAt = [DateTime]::UtcNow.ToString('o')
        $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8
    }
}
