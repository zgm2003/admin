<#
.SYNOPSIS
    Offline CSV permission/template seed, backup, fixed state sync and idempotency verification.
#>
[CmdletBinding()]
param([switch]$OldAPIStopped)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir = Join-Path $repoRoot 'server'
$sqlPath = Join-Path $PSScriptRoot '2026-09-30-mail-recipient-rule-csv.sql'
$sqlCommitted = $false

function Read-Env([string]$Path) {
    $values = @{}
    if (-not (Test-Path -LiteralPath $Path)) { return $values }
    foreach ($raw in Get-Content -LiteralPath $Path -Encoding utf8) {
        $line = $raw.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        $index = $line.IndexOf('=')
        if ($index -lt 1) { continue }
        $values[$line.Substring(0, $index).Trim()] = $line.Substring($index + 1).Trim().Trim('"').Trim("'")
    }
    return $values
}
function Setting([hashtable]$Values, [string]$Key) {
    $value = [Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    if ($Values.ContainsKey($Key)) { return [string]$Values[$Key] }
    return ''
}
function Assert-Stopped {
    $processes = @(Get-CimInstance Win32_Process | Where-Object {
        $_.Name -in @('api.exe','worker.exe') -or
        ($_.Name -eq 'go.exe' -and $_.CommandLine -match 'run\s+\./cmd/(api|worker)(\s|$)')
    })
    if ($processes.Count -gt 0) {
        throw ('Stop API/Worker first: ' + (($processes | ForEach-Object { "$($_.Name):$($_.ProcessId)" }) -join ', '))
    }
}
function Query([string]$SQL) {
    $output = @(& psql -X -w -At -v ON_ERROR_STOP=1 -d $script:libpqDSN -c $SQL)
    if ($LASTEXITCODE -ne 0) { throw "Read-only psql query failed (exit $LASTEXITCODE)." }
    return (($output | Where-Object { $_ -ne '' }) -join "`n").Trim()
}
function Sync-State([string]$Mode) {
    Push-Location $serverDir
    try {
        if ($Mode -eq 'sync') {
            $result = @(& go run ./cmd/mail-recipient-rule-csv-migration -mode sync -old-api-stopped)
        } else {
            $result = @(& go run ./cmd/mail-recipient-rule-csv-migration -mode inspect)
        }
        if ($LASTEXITCODE -ne 0) { throw "Fixed Redis state $Mode failed (exit $LASTEXITCODE)." }
        return ($result -join "`n").Trim()
    } finally { Pop-Location }
}

# Includes outbox timestamps/counts, not an assumed initial or unpublished state.
$snapshotSQL = @'
SELECT jsonb_build_object(
 'platforms',(SELECT jsonb_agg(jsonb_build_object('id',id,'code',code,'version',menu_version) ORDER BY id) FROM permission_auth_platform WHERE deleted_at IS NULL),
 'actions',(SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY id),'[]'::jsonb) FROM permission_menu m WHERE code IN ('message:mail:rule:import','message:mail:rule:export')),
 'setting',(SELECT COALESCE(jsonb_agg(to_jsonb(s) ORDER BY id),'[]'::jsonb) FROM system_setting s WHERE setting_key='message.mail.recipient_rule.import_template_url'),
 'generations',(SELECT jsonb_agg(to_jsonb(g) ORDER BY namespace,scope_key) FROM system_config_cache_generation g WHERE namespace IN ('system.setting','message.mail')),
 'outbox',(SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY id),'[]'::jsonb) FROM system_config_cache_outbox o WHERE namespace IN ('system.setting','message.mail')),
 'rules',(SELECT count(*) FROM message_mail_recipient_rule),
 'grants',(SELECT count(*) FROM permission_role_menu))::text;
'@
$verifySQL = @'
SELECT
 (SELECT count(*)=2 FROM permission_menu m JOIN permission_auth_platform a ON a.id=m.platform_id JOIN permission_menu p ON p.id=m.parent_id
  WHERE a.code='admin' AND a.deleted_at IS NULL AND p.code='message:mail:view' AND p.platform_id=m.platform_id AND p.menu_type='page' AND p.deleted_at IS NULL
  AND m.code IN ('message:mail:rule:import','message:mail:rule:export') AND m.deleted_at IS NULL AND m.menu_type='action' AND m.is_hidden=1
  AND m.is_enabled IN (0,1) AND m.path IS NULL AND m.component_path IS NULL AND m.icon IS NULL AND m.i18n_key IS NULL)
 AND (SELECT count(*)=1 FROM system_setting WHERE setting_key='message.mail.recipient_rule.import_template_url' AND deleted_at IS NULL AND value_type=1 AND is_builtin=1 AND is_enabled=1)
 AND (SELECT count(*)=2 FROM system_config_cache_generation WHERE namespace IN ('system.setting','message.mail') AND scope_key='global' AND generation>=1);
'@

try {
    if (-not $OldAPIStopped) { throw 'Stop API/Worker and explicitly pass -OldAPIStopped.' }
    Assert-Stopped
    foreach ($name in @('go','psql','pg_dump','pg_restore')) { Get-Command $name -ErrorAction Stop | Out-Null }
    if (-not (Test-Path -LiteralPath $sqlPath)) { throw 'Migration SQL is missing.' }
    $values = Read-Env (Join-Path $serverDir '.env')
    $dsn = Setting $values 'POSTGRES_DSN'
    $redisURL = Setting $values 'REDIS_URL'
    if ([string]::IsNullOrWhiteSpace($dsn) -or [string]::IsNullOrWhiteSpace($redisURL)) {
        throw 'POSTGRES_DSN and REDIS_URL are required.'
    }
    # libpq does not support pgx TimeZone. Keep credentials out of printed output.
    $libpqDSN = ($dsn -replace '(?i)(^|\s+)TimeZone=\S+', '$1' -replace '(?i)([?&])TimeZone=[^&]*', '$1' -replace '\?&', '?' -replace '[?&]$', '').Trim()
    $env:POSTGRES_DSN = $dsn
    $env:REDIS_URL = $redisURL
    $env:PGCLIENTENCODING = 'UTF8'
    if ((Query 'SELECT current_schema();') -ne 'public') { throw 'Expected public schema.' }
    $before = Query $snapshotSQL
    $stateBefore = Sync-State 'inspect'
    $backupDir = Join-Path $env:LOCALAPPDATA ('Admin\backups\mail-rule-csv-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    New-Item -ItemType Directory -Path $backupDir | Out-Null
    $dump = Join-Path $backupDir 'public-before.dump'
    & pg_dump -w --format=custom --schema=public --no-owner --no-privileges --dbname=$libpqDSN --file=$dump
    if ($LASTEXITCODE -ne 0) { throw 'pg_dump backup failed.' }
    & pg_restore --list $dump | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Backup archive validation failed.' }
    $before | Set-Content -LiteralPath (Join-Path $backupDir 'before.json') -Encoding utf8
    $stateBefore | Set-Content -LiteralPath (Join-Path $backupDir 'redis-before.json') -Encoding utf8
    Assert-Stopped
    & psql -X -w -v ON_ERROR_STOP=1 -d $libpqDSN -f $sqlPath
    if ($LASTEXITCODE -ne 0) { throw 'Forward SQL failed; transaction rolled back.' }
    $sqlCommitted = $true
    if ((Query $verifySQL) -ne 't') { throw 'Seed/action shape verification failed.' }
    $after = Query $snapshotSQL
    $initial = $before | ConvertFrom-Json
    $final = $after | ConvertFrom-Json
    foreach ($platform in $initial.platforms) {
        $actual = @($final.platforms | Where-Object { $_.id -eq $platform.id })
        $expected = [long]$platform.version
        if ($platform.code -eq 'admin') {
            $previousActions = @($initial.actions | Where-Object { $_.platform_id -eq $platform.id -and $null -eq $_.deleted_at })
            if ($previousActions.Count -lt 2) { $expected++ }
        }
        if ($actual.Count -ne 1 -or [long]$actual[0].version -ne $expected) { throw 'Unexpected platform menu version delta.' }
    }
    foreach ($generation in $initial.generations) {
        $actual = @($final.generations | Where-Object { $_.namespace -eq $generation.namespace -and $_.scope_key -eq $generation.scope_key })
        $expected = [long]$generation.generation
        if ($generation.namespace -eq 'system.setting' -and $generation.scope_key -eq 'global' -and @($initial.setting | Where-Object { $null -eq $_.deleted_at }).Count -eq 0) { $expected++ }
        if ($actual.Count -ne 1 -or [long]$actual[0].generation -ne $expected) { throw 'Unexpected config generation delta.' }
    }
    if ($initial.rules -ne $final.rules -or $initial.grants -ne $final.grants) { throw 'Rules/grants changed unexpectedly.' }
    $published = Sync-State 'sync'
    & psql -X -w -v ON_ERROR_STOP=1 -d $libpqDSN -f $sqlPath | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Forward SQL idempotency rerun failed.' }
    if ((Query $snapshotSQL) -ne $after) { throw 'SQL rerun changed menu/settings/generation/outbox facts.' }
    if ((Sync-State 'sync') -ne $published) { throw 'Redis state rerun changed versions.' }
    $after | Set-Content -LiteralPath (Join-Path $backupDir 'after.json') -Encoding utf8
    $published | Set-Content -LiteralPath (Join-Path $backupDir 'redis-after.json') -Encoding utf8
    $snapshotPath = Join-Path $PSScriptRoot 'current.sql'
    & pg_dump -w --schema-only --schema=public --no-owner --no-privileges --dbname=$libpqDSN --file=$snapshotPath
    if ($LASTEXITCODE -ne 0) { throw 'Schema snapshot export failed.' }
    $hash = (Get-FileHash -LiteralPath $dump -Algorithm SHA256).Hash
    Write-Host "Completed: CSV action/template seeds, version publication and two-pass idempotency verified. Redis keys deleted: 0."
    Write-Host "Backup: $dump"
    Write-Host "Backup SHA256: $hash"
    Write-Host "State: $published"
    Write-Host 'API/Worker remain stopped. Start the new version yourself.'
} catch {
    [Console]::Error.WriteLine('Migration incomplete: {0}', $_.Exception.Message)
    if ($sqlCommitted) { [Console]::Error.WriteLine('PostgreSQL committed; state sync/verification may be incomplete. Rerun this runner safely before starting API/Worker.') }
    exit 1
}
