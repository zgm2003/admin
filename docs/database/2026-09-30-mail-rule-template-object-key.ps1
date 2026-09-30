<# Offline, backed-up conversion from a template URL to a standard storage object key. #>
[CmdletBinding()]
param([switch]$OldAPIStopped)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir = Join-Path $repoRoot 'server'
$sqlPath = Join-Path $PSScriptRoot '2026-09-30-mail-rule-template-object-key.sql'
$committed = $false

function Assert-Stopped {
    $running = @(Get-CimInstance Win32_Process | Where-Object {
        $_.Name -in @('api.exe','worker.exe') -or
        ($_.Name -eq 'go.exe' -and $_.CommandLine -match 'run\s+\./cmd/(api|worker)(\s|$)')
    })
    if ($running.Count -gt 0) { throw 'Stop API/Worker before running this migration.' }
}
function Query([string]$SQL) {
    $output = @(& psql -X -w -At -v ON_ERROR_STOP=1 -d $script:dsn -c $SQL)
    if ($LASTEXITCODE -ne 0) { throw 'Read-only SQL failed.' }
    return (($output | Where-Object { $_ -ne '' }) -join "`n").Trim()
}
function Template-Command([string]$Mode) {
    Push-Location $serverDir
    try {
        $result = @(& go run ./cmd/mail-rule-template-object-key-migration -mode $Mode -manifest $script:manifestPath -old-api-stopped)
        if ($LASTEXITCODE -ne 0) { throw "Template $Mode failed; keep the manifest and rerun." }
        return ($result -join "`n").Trim()
    } finally { Pop-Location }
}
function Sync-State {
    Push-Location $serverDir
    try {
        & go run ./cmd/mail-recipient-rule-csv-migration -mode sync -old-api-stopped
        if ($LASTEXITCODE -ne 0) { throw 'Fixed Redis state publication failed.' }
    } finally { Pop-Location }
}
$snapshotSQL = @'
SELECT jsonb_build_object(
 'settings',(SELECT COALESCE(jsonb_agg(to_jsonb(s) ORDER BY id),'[]'::jsonb) FROM system_setting s WHERE setting_key IN ('message.mail.recipient_rule.import_template_url','message.mail.recipient_rule.import_template_object_key')),
 'generations',(SELECT jsonb_agg(to_jsonb(g) ORDER BY namespace,scope_key) FROM system_config_cache_generation g),
 'outbox',(SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY id),'[]'::jsonb) FROM system_config_cache_outbox o WHERE namespace='system.setting'),
 'rules',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM storage_upload_rule r),
 'protected',jsonb_build_object(
  'platforms',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permission_auth_platform p),
  'uploadCodes',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM storage_upload_rule_code c),
  'ruleOtherFields',(SELECT jsonb_agg(to_jsonb(r)-'allowed_extensions'-'allowed_mime_types'-'updated_at' ORDER BY id) FROM storage_upload_rule r),
  'mailRules',(SELECT count(*) FROM message_mail_recipient_rule),
  'roleGrants',(SELECT count(*) FROM permission_role_menu)))::text;
'@
try {
    if (-not $OldAPIStopped) { throw 'Explicitly stop API/Worker and pass -OldAPIStopped.' }
    Assert-Stopped
    foreach ($command in @('go','psql','pg_dump','pg_restore')) { Get-Command $command -ErrorAction Stop | Out-Null }
    $values = @{}
    foreach ($line in Get-Content -LiteralPath (Join-Path $serverDir '.env') -Encoding utf8) {
        if ($line -match '^\s*([^#=\s]+)\s*=(.*)$') { $values[$matches[1]] = $matches[2].Trim().Trim('"').Trim("'") }
    }
    foreach ($name in @('POSTGRES_DSN','REDIS_URL')) {
        $value = [Environment]::GetEnvironmentVariable($name)
        if ([string]::IsNullOrWhiteSpace($value) -and $values.ContainsKey($name)) { $value = $values[$name] }
        if ([string]::IsNullOrWhiteSpace($value)) { throw "$name is required." }
        [Environment]::SetEnvironmentVariable($name,$value,'Process')
    }
    $dsn = ($env:POSTGRES_DSN -replace '(?i)(^|\s+)TimeZone=\S+', '$1' -replace '(?i)([?&])TimeZone=[^&]*', '$1' -replace '\?&', '?' -replace '[?&]$', '').Trim()
    $env:PGCLIENTENCODING = 'UTF8'
    if ((Query 'SELECT current_schema();') -ne 'public') { throw 'Expected public schema.' }
    $backupDir = Join-Path $env:LOCALAPPDATA ('Admin\backups\mail-rule-template-object-key-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    New-Item -ItemType Directory -Path $backupDir | Out-Null
    $dump = Join-Path $backupDir 'public-before.dump'
    & pg_dump -w --format=custom --schema=public --no-owner --no-privileges --dbname=$dsn --file=$dump
    if ($LASTEXITCODE -ne 0) { throw 'Backup failed.' }
    & pg_restore --list $dump | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Backup archive verification failed.' }
    $before = Query $snapshotSQL
    $before | Set-Content -LiteralPath (Join-Path $backupDir 'before.json') -Encoding utf8
    $manifestDir = Join-Path $env:LOCALAPPDATA 'Admin\maintenance'
    New-Item -ItemType Directory -Force -Path $manifestDir | Out-Null
    $manifestPath = Join-Path $manifestDir 'mail-rule-template-object-key.json'
    if (Test-Path -LiteralPath $manifestPath) { Copy-Item -LiteralPath $manifestPath -Destination (Join-Path $backupDir 'manifest-before.json') }
    Assert-Stopped
    $prepared = Template-Command 'prepare'
    $manifest = $prepared | ConvertFrom-Json
    $prepared | Set-Content -LiteralPath (Join-Path $backupDir 'template.json') -Encoding utf8
    & psql -X -w -v ON_ERROR_STOP=1 -v "template_source_url=$($manifest.sourceUrl)" -v "template_object_key=$($manifest.objectKey)" -d $dsn -f $sqlPath
    if ($LASTEXITCODE -ne 0) { throw 'Forward SQL failed; setting rename rolled back.' }
    $committed = $true
    $after = Query $snapshotSQL
    $initial = $before | ConvertFrom-Json
    $final = $after | ConvertFrom-Json
    if (($initial.protected | ConvertTo-Json -Depth 30 -Compress) -ne ($final.protected | ConvertTo-Json -Depth 30 -Compress)) { throw 'Unrelated platform/upload/mail/grant facts changed.' }
    foreach ($generation in $initial.generations) {
        $actual = @($final.generations | Where-Object { $_.namespace -eq $generation.namespace -and $_.scope_key -eq $generation.scope_key })
        $expected = [long]$generation.generation
        if ($generation.namespace -eq 'system.setting' -and $generation.scope_key -eq 'global' -and @($initial.settings | Where-Object { $_.setting_key -eq 'message.mail.recipient_rule.import_template_url' -and $null -eq $_.deleted_at }).Count -eq 1) { $expected++ }
        if ($actual.Count -ne 1 -or [long]$actual[0].generation -ne $expected) { throw 'Unexpected generation delta.' }
    }
    foreach ($rule in $initial.rules) {
        $actual = @($final.rules | Where-Object { $_.id -eq $rule.id })
        $extensions = @($rule.allowed_extensions)
        $mimeTypes = @($rule.allowed_mime_types)
        if ($rule.id -eq $manifest.ruleId) {
            if ($extensions -notcontains 'csv') { $extensions += 'csv' }
            if ($mimeTypes.Count -gt 0 -and $mimeTypes -notcontains 'text/csv') { $mimeTypes += 'text/csv' }
        }
        if ($actual.Count -ne 1 -or ($extensions -join '|') -ne ($actual[0].allowed_extensions -join '|') -or ($mimeTypes -join '|') -ne ($actual[0].allowed_mime_types -join '|')) { throw 'Unexpected upload allowlist change.' }
    }
    Sync-State
    $verified = Template-Command 'verify'
    # Repeat preparation and SQL, then compare complete setting/rule/version/outbox facts.
    Template-Command 'prepare' | Out-Null
    & psql -X -w -v ON_ERROR_STOP=1 -v "template_source_url=$($manifest.sourceUrl)" -v "template_object_key=$($manifest.objectKey)" -d $dsn -f $sqlPath | Out-Null
    if ($LASTEXITCODE -ne 0 -or (Query $snapshotSQL) -ne $after) { throw 'Idempotency verification failed.' }
    Sync-State
    Template-Command 'verify' | Out-Null
    $after | Set-Content -LiteralPath (Join-Path $backupDir 'after.json') -Encoding utf8
    Write-Host 'Completed: standard object key uploaded, setting renamed, cache published, downloads and idempotency verified.'
    Write-Host "Backup: $dump"
    Write-Host "Backup SHA256: $((Get-FileHash -LiteralPath $dump -Algorithm SHA256).Hash)"
    Write-Host "Verified template: $verified"
    Write-Host 'No schema change, Redis deletion, Canvas change or service restart. Start the new API/Worker yourself.'
} catch {
    [Console]::Error.WriteLine('Migration incomplete: {0}', $_.Exception.Message)
    if ($committed) { [Console]::Error.WriteLine('PostgreSQL setting rename committed; publication or verification incomplete. Keep the manifest and rerun before starting services.') }
    else { [Console]::Error.WriteLine('Upload rule/COS preparation may have completed. Keep the manifest and rerun; no objects are deleted automatically.') }
    exit 1
}
