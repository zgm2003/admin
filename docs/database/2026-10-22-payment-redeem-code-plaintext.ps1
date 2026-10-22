<#
.SYNOPSIS
备份并将兑换码无损转为明文存储；必须先停止 API/Worker。
.EXAMPLE
.\docs\database\2026-10-22-payment-redeem-code-plaintext.ps1 -CheckOnly
.EXAMPLE
.\docs\database\2026-10-22-payment-redeem-code-plaintext.ps1 -ServicesStopped
.DESCRIPTION
备份含历史密文，存放在仓库外的本地审计目录，不输出兑换码或 APP_SECRET。
转换失败整笔事务回滚；提交结果未知时保持服务停止，核对审计后重跑。
备份可通过 pg_restore 在隔离数据库恢复检查，不自动覆盖业务库。
#>
[CmdletBinding()]
param(
    [switch]$CheckOnly,
    [switch]$ServicesStopped,
    [ValidatePattern('^[a-z][a-z0-9_]{0,62}$')][string]$Schema = 'public',
    [string]$AuditDirectory
)
$ErrorActionPreference = 'Stop'
$serverRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../server'))
$sqlPath = Join-Path $PSScriptRoot '2026-10-22-payment-redeem-code-plaintext.sql'
$oldDsn = $env:POSTGRES_DSN
$oldSecret = $env:APP_SECRET
$oldApp = $env:PGAPPNAME
$auditPath = $null
$audit = [ordered]@{ migration='payment-redeem-code-plaintext'; startedAt=[DateTime]::UtcNow.ToString('o'); postgresCommitState='notStarted'; verified=$false }
function Query([string]$Sql) {
    $output = @(& psql -X -w -q -A -t --dbname $script:cliDsn -v ON_ERROR_STOP=1 -c ("BEGIN READ ONLY; SET LOCAL search_path TO `"$Schema`",pg_catalog; SET LOCAL statement_timeout='15s'; " + $Sql + '; ROLLBACK;') 2>&1)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 只读检查失败。' }
    return (($output -join "`n") | ConvertFrom-Json)
}
function Apply {
    Push-Location $serverRoot
    try {
        $output = @(& go run (Join-Path $PSScriptRoot "2026-10-22-payment-redeem-code-plaintext/main.go") -schema $Schema -sql $sqlPath 2>&1)
        if ($LASTEXITCODE -ne 0) { throw ('兑换码迁移失败；保持服务停止并检查事务结果。' + ($output -join "`n")) }
        return (($output -join "`n") | ConvertFrom-Json)
    } finally { Pop-Location }
}
try {
    foreach ($tool in @('psql','pg_dump','pg_restore','go')) { Get-Command $tool -ErrorAction Stop | Out-Null }
    $values = @{}
    foreach ($raw in Get-Content -LiteralPath (Join-Path $serverRoot '.env') -Encoding UTF8) {
        if ($raw.Trim() -match '^([A-Z][A-Z0-9_]*)\s*=\s*(.*?)\s*$') { $values[$Matches[1]] = $Matches[2].Trim('"').Trim("'") }
    }
    if ([string]::IsNullOrWhiteSpace($env:POSTGRES_DSN)) { $env:POSTGRES_DSN = $values.POSTGRES_DSN }
    if ([string]::IsNullOrWhiteSpace($env:APP_SECRET)) { $env:APP_SECRET = $values.APP_SECRET }
    if ([string]::IsNullOrWhiteSpace($env:POSTGRES_DSN)) { throw '缺少 POSTGRES_DSN。' }
    $script:cliDsn = $env:POSTGRES_DSN
    if ($script:cliDsn -match '^\s*postgres(?:ql)?://') {
        $script:cliDsn = $script:cliDsn -replace '(?i)([?&])TimeZone=[^&]*', '$1' -replace '\?&', '?' -replace '[?&]$', ''
    } else { $script:cliDsn = ($script:cliDsn -replace '(?i)(^|\s+)TimeZone=\S+', '$1').Trim() }
    $env:PGAPPNAME = 'admin-redeem-plaintext-maintenance'
    $before = Query @"
SELECT jsonb_build_object('schema',current_schema(),
 'rows',(SELECT count(*) FROM payment_redeem_code),
 'legacyColumns',(SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='payment_redeem_code' AND column_name IN ('code_hash','code_ciphertext')),
 'plaintextColumns',(SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='payment_redeem_code' AND column_name='code'))
"@
    if ($before.schema -ne $Schema) { throw '目标 schema 不匹配。' }
    if ($CheckOnly) { $before | ConvertTo-Json; return }
    if (-not $ServicesStopped) { throw '先停止 API/Worker，再传入 -ServicesStopped。' }
    $clients = Query "SELECT jsonb_build_object('count',count(*)) FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND pid<>pg_backend_pid()"
    if ($Schema -eq 'public' -and $clients.count -ne 0) { throw '业务库仍有其他连接；未迁移，请先停止服务和数据库客户端。' }
    if ([string]::IsNullOrWhiteSpace($AuditDirectory)) { $AuditDirectory = Join-Path $env:LOCALAPPDATA ('Admin/maintenance/redeem-plaintext/' + [Guid]::NewGuid().ToString('N')) }
    $AuditDirectory = [IO.Path]::GetFullPath($AuditDirectory)
    if ((Test-Path -LiteralPath $AuditDirectory) -and @(Get-ChildItem -LiteralPath $AuditDirectory -Force).Count -gt 0) { throw '审计目录必须为空。' }
    New-Item -ItemType Directory -Path $AuditDirectory -Force | Out-Null
    $auditPath = Join-Path $AuditDirectory 'audit.json'
    $audit.before = $before
    $audit.sqlSHA256 = (Get-FileHash -LiteralPath $sqlPath -Algorithm SHA256).Hash
    $backup = Join-Path $AuditDirectory 'before.dump'
    & pg_dump --format=custom --no-owner --no-privileges --dbname $script:cliDsn --schema $Schema --file $backup 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw '备份失败；未执行迁移。' }
    & pg_restore --list $backup 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw '备份归档不可读；未执行迁移。' }
    & pg_restore --no-owner --no-privileges --file (Join-Path $AuditDirectory 'restore-check.sql') $backup 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw '备份完整读取失败；未执行迁移。' }
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash
    $audit.postgresCommitState = 'unknown'
    $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8
    $audit.after = Apply
    $audit.postgresCommitState = 'committed'
    $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8
    $audit.replayed = Apply
    if ($audit.after.rows -ne $before.rows -or $audit.replayed.rows -ne $before.rows -or $audit.replayed.converted -ne 0 -or $audit.after.factsDigest -ne $audit.replayed.factsDigest -or $audit.after.schemaDigest -ne $audit.replayed.schemaDigest) { throw 'PostgreSQL 已提交，但保真或幂等复核失败。' }
    $audit.verified = $true
    Write-Output "明文迁移完成，保留 $($audit.after.rows) 条兑换码；审计目录：$AuditDirectory"
} catch {
    $audit.error = $_.Exception.Message
    throw
} finally {
    if ($null -ne $auditPath) { $audit | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $auditPath -Encoding UTF8 }
    $env:POSTGRES_DSN = $oldDsn
    $env:APP_SECRET = $oldSecret
    $env:PGAPPNAME = $oldApp
}
