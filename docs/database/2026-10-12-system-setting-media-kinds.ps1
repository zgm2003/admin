param([string]$EnvFile = (Join-Path $PSScriptRoot '../../server/.env'))

$ErrorActionPreference = 'Stop'
$utf8 = [Text.UTF8Encoding]::new($false)
$savedEnvironment = @{}
$connectionVariables = @{PGHOST='host'; PGPORT='port'; PGUSER='user'; PGPASSWORD='password'; PGDATABASE='dbname'; PGSSLMODE='sslmode'}
$snapshotSQL = @'
SELECT jsonb_build_object(
 'settings',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM system_setting s),
 'states',(SELECT jsonb_agg(to_jsonb(s) ORDER BY namespace,scope_key) FROM system_config_cache_state s),
 'outbox',(SELECT jsonb_agg(jsonb_build_object('id',id,'namespace',namespace,'scopeKey',scope_key,'generation',generation) ORDER BY id) FROM system_config_cache_outbox)
)::text
'@

function Read-Snapshot {
  $result = & psql -X -qAt -v ON_ERROR_STOP=1 -c $snapshotSQL
  if ($LASTEXITCODE -ne 0) { throw 'Cannot read migration snapshot' }
  return ($result -join "`n")
}

try {
  $entry = Get-Content -LiteralPath $EnvFile | Where-Object { $_ -match '^\s*POSTGRES_DSN\s*=' } | Select-Object -First 1
  if (!$entry) { throw 'POSTGRES_DSN is missing' }
  $dsnValue = ($entry -replace '^\s*POSTGRES_DSN\s*=\s*', '').Trim().Trim('"')
  $parts = @{}
  foreach ($match in [regex]::Matches($dsnValue, "(\w+)=(?:'((?:[^'\\]|\\.)*)'|([^\s]+))")) {
    $parts[$match.Groups[1].Value] = if ($match.Groups[2].Success) { $match.Groups[2].Value -replace '\\(.)','$1' } else { $match.Groups[3].Value }
  }
  if (!$parts.ContainsKey('dbname')) { throw 'Expected a PostgreSQL keyword DSN with dbname' }
  foreach ($pair in $connectionVariables.GetEnumerator()) {
    $savedEnvironment[$pair.Key] = [Environment]::GetEnvironmentVariable($pair.Key,'Process')
    [Environment]::SetEnvironmentVariable($pair.Key,$parts[$pair.Value],'Process')
  }
  $auditDirectory = Join-Path $env:LOCALAPPDATA ('Admin/backups/system-setting-media-kinds/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
  New-Item -ItemType Directory -Path $auditDirectory -Force | Out-Null
  & pg_dump --format=custom --no-owner --no-privileges --table=public.system_setting --table=public.system_config_cache_state --table=public.system_config_cache_outbox --file (Join-Path $auditDirectory 'before.dump')
  if ($LASTEXITCODE -ne 0) { throw 'Database backup failed; migration was not executed' }
  $beforeText = Read-Snapshot
  [IO.File]::WriteAllText((Join-Path $auditDirectory 'before.json'),$beforeText,$utf8)
  $script = Join-Path $PSScriptRoot '2026-10-12-system-setting-media-kinds.sql'
  & psql -X -v ON_ERROR_STOP=1 -f $script
  if ($LASTEXITCODE -ne 0) { throw 'Migration failed and its atomic block was rolled back' }
  $afterText = Read-Snapshot
  [IO.File]::WriteAllText((Join-Path $auditDirectory 'after.json'),$afterText,$utf8)
  $before = $beforeText | ConvertFrom-Json
  $after = $afterText | ConvertFrom-Json
  $immutableFields = 'id','setting_key','value','description','is_enabled','is_builtin','created_at','deleted_at'
  if (($before.settings | Select-Object $immutableFields | ConvertTo-Json -Depth 10 -Compress) -cne ($after.settings | Select-Object $immutableFields | ConvertTo-Json -Depth 10 -Compress)) { throw 'PostgreSQL committed, but setting identity/value preservation check failed' }
  $avatar = @($after.settings | Where-Object { $_.setting_key -eq 'app.brand.default_avatar' -and !$_.deleted_at })
  $template = @($after.settings | Where-Object { $_.setting_key -eq 'message.mail.recipient_rule.import_template_object_key' -and !$_.deleted_at })
  if ($avatar.Count -ne 1 -or $avatar[0].value_type -ne 6 -or $template.Count -ne 1 -or $template[0].value_type -ne 5) { throw 'PostgreSQL committed, but builtin media type verification failed' }
  $changed = @($after.settings | Where-Object { $row = $_; ($before.settings | Where-Object id -eq $row.id).value_type -ne $row.value_type }).Count
  $beforeState = $before.states | Where-Object { $_.namespace -eq 'system.setting' -and $_.scope_key -eq 'global' }
  $afterState = $after.states | Where-Object { $_.namespace -eq 'system.setting' -and $_.scope_key -eq 'global' }
  $expectedGeneration = $beforeState.generation + [int]($changed -gt 0)
  if ($afterState.generation -ne $expectedGeneration) { throw 'PostgreSQL committed, but generation verification failed' }
  if ($changed -gt 0 -and !($after.outbox | Where-Object { $_.namespace -eq 'system.setting' -and $_.scopeKey -eq 'global' -and $_.generation -eq $expectedGeneration })) { throw 'PostgreSQL committed, but cache invalidation intent is missing' }
  & psql -X -v ON_ERROR_STOP=1 -f $script
  if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL committed, but migration rerun failed' }
  $rerunText = Read-Snapshot
  [IO.File]::WriteAllText((Join-Path $auditDirectory 'rerun.json'),$rerunText,$utf8)
  if ($rerunText -cne $afterText) { throw 'PostgreSQL committed, but idempotence verification failed' }
  $schemaFile = Join-Path $auditDirectory 'current.sql'
  & pg_dump --schema-only --schema=public --no-owner --no-privileges --restrict-key=AdminCurrentSchemaSnapshot --file=$schemaFile
  if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL committed, but schema export failed' }
  Copy-Item -LiteralPath $schemaFile -Destination (Join-Path $PSScriptRoot 'current.sql')
  Write-Output "Migration verified: $changed media rows changed; generation $($beforeState.generation) -> $($afterState.generation); rerun unchanged."
  Write-Output "Backup and audit: $auditDirectory"
} finally {
  foreach ($pair in $savedEnvironment.GetEnumerator()) { [Environment]::SetEnvironmentVariable($pair.Key,$pair.Value,'Process') }
}
