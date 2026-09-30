<# Offline, backed-up migration of the setting media value type. No COS operations. #>
[CmdletBinding()]
param([switch]$OldAPIStopped)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir=Join-Path $repoRoot 'server'
$sqlPath=Join-Path $PSScriptRoot '2026-09-30-system-setting-media-value-type.sql'
$committed=$false
function Assert-Stopped {
    $running=@(Get-CimInstance Win32_Process | Where-Object {
        $_.Name -in @('api.exe','worker.exe') -or ($_.Name -eq 'go.exe' -and $_.CommandLine -match 'run\s+\./cmd/(api|worker)(\s|$)')
    })
    if($running.Count -gt 0){throw 'Stop API/Worker before this migration.'}
}
function Query([string]$SQL){
    $result=@(& psql -X -w -At -v ON_ERROR_STOP=1 -d $script:dsn -c $SQL)
    if($LASTEXITCODE -ne 0){throw 'SQL inspection failed.'}
    return (($result | Where-Object {$_ -ne ''}) -join "`n").Trim()
}
function Sync-State {
    Push-Location $serverDir
    try {
        $result=@(& go run ./cmd/mail-recipient-rule-csv-migration -mode sync -old-api-stopped)
        if($LASTEXITCODE -ne 0){throw 'Redis version publication failed.'}
        return ($result -join "`n").Trim()
    } finally {Pop-Location}
}
$snapshotSQL=@'
SELECT jsonb_build_object(
 'settings',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM system_setting s),
 'generations',(SELECT jsonb_agg(to_jsonb(g) ORDER BY namespace,scope_key) FROM system_config_cache_generation g),
 'outbox',(SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY id),'[]'::jsonb) FROM system_config_cache_outbox o WHERE namespace='system.setting'),
 'constraint',(SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='system_setting'::regclass AND conname='ck_system_setting_value_type'),
 'comment',col_description('system_setting'::regclass,(SELECT attnum FROM pg_attribute WHERE attrelid='system_setting'::regclass AND attname='value_type')),
 'protected',jsonb_build_object(
  'settingValues',(SELECT jsonb_agg(to_jsonb(s)-'value_type'-'updated_at' ORDER BY id) FROM system_setting s),
  'otherSettings',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM system_setting s WHERE setting_key NOT IN ('app.brand.default_avatar','message.mail.recipient_rule.import_template_object_key')),
  'valueColumn',(SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='system_setting' AND column_name='value'),
  'platforms',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permission_auth_platform p),
  'uploadRules',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM storage_upload_rule r),
  'roleGrants',(SELECT count(*) FROM permission_role_menu)))::text;
'@
try {
    if(-not $OldAPIStopped){throw 'Explicitly stop services and pass -OldAPIStopped.'}
    Assert-Stopped
    foreach($name in @('go','psql','pg_dump','pg_restore')){Get-Command $name -ErrorAction Stop | Out-Null}
    $envValues=@{}
    foreach($line in Get-Content -LiteralPath (Join-Path $serverDir '.env') -Encoding utf8){
        if($line -match '^\s*([^#=\s]+)\s*=(.*)$'){$envValues[$matches[1]]=$matches[2].Trim().Trim('"').Trim("'")}
    }
    foreach($name in @('POSTGRES_DSN','REDIS_URL')){
        $value=[Environment]::GetEnvironmentVariable($name)
        if([string]::IsNullOrWhiteSpace($value) -and $envValues.ContainsKey($name)){$value=$envValues[$name]}
        if([string]::IsNullOrWhiteSpace($value)){throw "$name is required."}
        [Environment]::SetEnvironmentVariable($name,$value,'Process')
    }
    $dsn=($env:POSTGRES_DSN -replace '(?i)(^|\s+)TimeZone=\S+', '$1' -replace '(?i)([?&])TimeZone=[^&]*', '$1' -replace '\?&', '?' -replace '[?&]$', '').Trim()
    $env:PGCLIENTENCODING='UTF8'
    if((Query 'SELECT current_schema();') -ne 'public'){throw 'Expected public schema.'}
    $backupDir=Join-Path $env:LOCALAPPDATA ('Admin\backups\system-setting-media-type-'+(Get-Date -Format 'yyyyMMdd-HHmmss'))
    New-Item -ItemType Directory -Path $backupDir | Out-Null
    $dump=Join-Path $backupDir 'public-before.dump'
    & pg_dump -w --format=custom --schema=public --no-owner --no-privileges --dbname=$dsn --file=$dump
    if($LASTEXITCODE -ne 0){throw 'Backup failed.'}
    & pg_restore --list $dump | Out-Null
    if($LASTEXITCODE -ne 0){throw 'Backup archive verification failed.'}
    $before=Query $snapshotSQL
    $before | Set-Content -LiteralPath (Join-Path $backupDir 'before.json') -Encoding utf8
    Assert-Stopped
    & psql -X -w -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath
    if($LASTEXITCODE -ne 0){throw 'Migration failed and rolled back.'}
    $committed=$true
    $after=Query $snapshotSQL
    $initial=$before | ConvertFrom-Json
    $final=$after | ConvertFrom-Json
    if(($initial.protected | ConvertTo-Json -Depth 30 -Compress) -ne ($final.protected | ConvertTo-Json -Depth 30 -Compress)){throw 'Protected IDs/keys/values/platforms/upload rules changed.'}
    if($final.constraint -ne 'CHECK ((value_type = ANY (ARRAY[1, 2, 3, 4, 5])))'){throw 'Media enum constraint verification failed.'}
    $media=@($final.settings | Where-Object {$_.setting_key -in @('app.brand.default_avatar','message.mail.recipient_rule.import_template_object_key') -and $null -eq $_.deleted_at})
    if($media.Count -ne 2 -or @($media | Where-Object {$_.value_type -ne 5 -or $_.is_builtin -ne 1 -or $_.is_enabled -ne 1}).Count -ne 0){throw 'Media metadata verification failed.'}
    $changed=$initial.constraint -ne $final.constraint -or @($initial.settings | Where-Object {$_.setting_key -in @('app.brand.default_avatar','message.mail.recipient_rule.import_template_object_key') -and $null -eq $_.deleted_at -and $_.value_type -eq 1}).Count -gt 0
    foreach($generation in $initial.generations){
        $actual=@($final.generations | Where-Object {$_.namespace -eq $generation.namespace -and $_.scope_key -eq $generation.scope_key})
        $expected=[long]$generation.generation
        if($changed -and $generation.namespace -eq 'system.setting' -and $generation.scope_key -eq 'global'){$expected++}
        if($actual.Count -ne 1 -or [long]$actual[0].generation -ne $expected){throw 'Unexpected generation delta.'}
    }
    $published=Sync-State
    & psql -X -w -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath | Out-Null
    if($LASTEXITCODE -ne 0 -or (Query $snapshotSQL) -ne $after){throw 'SQL idempotency verification failed.'}
    if((Sync-State) -ne $published){throw 'Redis publication idempotency failed.'}
    $after | Set-Content -LiteralPath (Join-Path $backupDir 'after.json') -Encoding utf8
    $published | Set-Content -LiteralPath (Join-Path $backupDir 'redis-after.json') -Encoding utf8
    $schemaPath=Join-Path $PSScriptRoot 'current.sql'
    & pg_dump -w --schema-only --schema=public --no-owner --no-privileges --dbname=$dsn --file=$schemaPath
    if($LASTEXITCODE -ne 0){throw 'Schema snapshot export failed.'}
    Write-Host 'Completed: media type 5 enabled; values remain TEXT object keys; two-pass idempotency verified.'
    Write-Host "Backup: $dump"
    Write-Host "Backup SHA256: $((Get-FileHash -LiteralPath $dump -Algorithm SHA256).Hash)"
    Write-Host "State: $published"
    Write-Host 'No Redis keys deleted, COS changes or service restart. Start the new API/Worker yourself.'
} catch {
    [Console]::Error.WriteLine('Migration incomplete: {0}', $_.Exception.Message)
    if($committed){[Console]::Error.WriteLine('PostgreSQL committed; publication/verification may be incomplete. Rerun safely before starting services.')}
    exit 1
}
