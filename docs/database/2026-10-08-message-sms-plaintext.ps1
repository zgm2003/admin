[CmdletBinding()]
param([switch]$OldAPIStopped)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $OldAPIStopped) { throw 'Refusing migration: stop API/Worker and pass -OldAPIStopped. This runner never stops services.' }
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$server = Join-Path $root 'server'
$sql = Join-Path $PSScriptRoot '2026-10-08-message-sms-plaintext.sql'
foreach ($name in @('go','psql','pg_dump','pg_restore')) { if (-not (Get-Command $name -CommandType Application -ErrorAction SilentlyContinue)) { throw "Required executable is missing: $name" } }
function Read-Setting([string]$key) {
    $envValue = [Environment]::GetEnvironmentVariable($key); if (-not [string]::IsNullOrWhiteSpace($envValue)) { return $envValue }
    foreach ($raw in Get-Content -LiteralPath (Join-Path $server '.env') -Encoding UTF8) { if ($raw.Trim().StartsWith($key + '=')) { return $raw.Trim().Substring($key.Length+1).Trim().Trim('"').Trim("'") } }
    throw "Required environment setting is missing: $key"
}
$dsn = Read-Setting 'POSTGRES_DSN'
$dsn = [regex]::Replace($dsn, '(?i)(?:^|\s+)TimeZone=(?:''(?:[^''\\]|\\.)*''|(?:\\.|[^\s])+)', '')
$backupRoot = Join-Path $env:LOCALAPPDATA 'Admin\backups'
$backup = Join-Path $backupRoot ('message-sms-plaintext-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0,8))
New-Item -ItemType Directory -Path $backup | Out-Null
$helper = Join-Path $backup 'message-sms-plaintext-migration.exe'
$dump = Join-Path $backup 'public-before.dump'
$schema = Join-Path $backup 'public-after.sql'
$committed = $false
function Audit([string]$mode) { $args=@('-mode',$mode); if($mode -eq 'sync'){$args+='-old-api-stopped'}; $raw=@(& $helper @args); if($LASTEXITCODE -ne 0){throw "SMS audit $mode failed"}; return (($raw -join "`n")|ConvertFrom-Json) }
function SaveAudit([string]$name,$value){$value|ConvertTo-Json -Depth 8|Set-Content -LiteralPath (Join-Path $backup $name) -Encoding UTF8}
Push-Location $server
try {
    & go build -o $helper ./cmd/message-sms-plaintext-migration; if($LASTEXITCODE -ne 0){throw 'SMS migration helper build failed'}
    $before=Audit 'inspect'; SaveAudit 'before.json' $before
    if($before.PlaintextSchema){$expected=[long]$before.Generation}else{$expected=[long]$before.Generation+1}
    & pg_dump --format=custom --schema=public --no-owner --no-privileges --no-password --dbname=$dsn --file=$dump; if($LASTEXITCODE -ne 0){throw 'Database backup failed'}
    & pg_restore --list $dump | Out-Null; if($LASTEXITCODE -ne 0){throw 'Backup verification failed'}
    (Get-FileHash -LiteralPath $dump -Algorithm SHA256).Hash | Set-Content -LiteralPath (Join-Path $backup 'public-before.sha256') -Encoding ASCII
    & psql -X --no-password -v ON_ERROR_STOP=1 --dbname=$dsn -c 'SET search_path TO public;' -f $sql; if($LASTEXITCODE -ne 0){throw 'SMS plaintext SQL failed'}
    $committed=$true
    $after=Audit 'sync'; SaveAudit 'after.json' $after
    if(-not $after.PlaintextSchema -or $after.Generation -ne $expected -or $after.RuleRows -ne $before.RuleRows -or $after.LogRows -ne $before.LogRows -or $after.ControlHash -ne $before.ControlHash){throw 'SMS facts/generation/control verification failed'}
    & psql -X --no-password -v ON_ERROR_STOP=1 --dbname=$dsn -c 'SET search_path TO public;' -f $sql; if($LASTEXITCODE -ne 0){throw 'SMS plaintext SQL repeat failed'}
    $repeat=Audit 'sync'; SaveAudit 'repeat.json' $repeat
    foreach($key in @('RuleScopeType','RuleActionType','RuleRows','LogRows','RuleFactsHash','LogFactsHash','Generation','ControlHash','PlaintextSchema','RedisState','RedisGeneration')){if($repeat.$key -ne $after.$key){throw "Idempotency failed: $key"}}
    & pg_dump --schema-only --schema=public --no-owner --no-privileges --no-password --dbname=$dsn --file=$schema; if($LASTEXITCODE -ne 0){throw 'Schema snapshot export failed'}
    Copy-Item -LiteralPath $schema -Destination (Join-Path $root 'docs/database/current.sql')
    Write-Output "SMS plaintext migration completed. Audit: $backup. Generation $($after.Generation). API/Worker remain stopped."
} catch { if($committed){throw "PostgreSQL migration committed; keep services stopped and rerun safely. Audit: $backup. $($_.Exception.Message)"}; throw } finally { Pop-Location }
