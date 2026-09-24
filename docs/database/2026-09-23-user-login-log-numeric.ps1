<#[.SYNOPSIS
    Converts user_login_log to the numeric register/login/logout audit contract.
#>
[CmdletBinding()]
param([switch]$OldAPIStopped)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $OldAPIStopped) { throw 'Refusing to run: stop API/Worker and explicitly pass -OldAPIStopped.' }

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$envPath = Join-Path $repoRoot 'server\.env'
$sqlPath = Join-Path $PSScriptRoot '2026-09-23-user-login-log-numeric.sql'

function Read-Env([string]$Path) {
    $result = @{}
    if (-not (Test-Path -LiteralPath $Path)) { return $result }
    foreach ($raw in Get-Content -LiteralPath $Path -Encoding utf8) {
        $line = $raw.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        $at = $line.IndexOf('=')
        if ($at -lt 1) { continue }
        $result[$line.Substring(0, $at).Trim()] = $line.Substring($at + 1).Trim().Trim('"').Trim("'")
    }
    return $result
}
function Setting([hashtable]$Values, [string]$Key) {
    $value = [Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    if ($Values.ContainsKey($Key)) { return [string]$Values[$Key] }
    return ''
}
function Query([string]$Dsn, [string]$Sql) {
    $output = @(& psql -X -A -t -v ON_ERROR_STOP=1 -d $Dsn -c $Sql)
    if ($LASTEXITCODE -ne 0) { throw "psql query failed (exit $LASTEXITCODE)." }
    return (($output | Where-Object { $_ -ne '' }) -join "`n").Trim()
}

$values = Read-Env $envPath
$dsn = Setting $values 'POSTGRES_DSN'
if ([string]::IsNullOrWhiteSpace($dsn)) { throw 'POSTGRES_DSN is required.' }
$dsn = $dsn -replace '(?i)(^|\s+)TimeZone=\S+', '$1'
if (-not (Test-Path -LiteralPath $sqlPath)) { throw "Migration SQL not found: $sqlPath" }

$before = Query $dsn "SELECT column_name || ':' || data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name IN ('login_account','account','event_type','login_type','session_id') ORDER BY column_name;"
if ([string]::IsNullOrWhiteSpace($before)) { throw 'Refusing to run: user_login_log is missing or has an unrecognized shape.' }

& psql -X -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath
if ($LASTEXITCODE -ne 0) { throw 'Migration SQL failed; the transaction was rolled back.' }

$shape = Query $dsn "SELECT column_name || ':' || data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name IN ('account','event_type','login_type','session_id') ORDER BY column_name;"
$expected = "account:character varying`nevent_type:smallint`nlogin_type:smallint"
$invalid = Query $dsn "SELECT count(*) FROM user_login_log WHERE event_type NOT IN (1,2,3) OR (event_type = 1 AND login_type NOT IN (2,3)) OR (event_type = 2 AND login_type NOT IN (1,2,3)) OR (event_type IN (1,2) AND login_type IS NULL) OR (event_type = 3 AND login_type IS NOT NULL) OR btrim(account) = '';"
$sessionCount = Query $dsn "SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_login_log' AND column_name='session_id';"
if ($shape -ne $expected -or $invalid -ne '0' -or $sessionCount -ne '0') {
    throw "Verification failed: shape='$shape' invalid=$invalid session_columns=$sessionCount."
}

& psql -X -v ON_ERROR_STOP=1 -d $dsn -f $sqlPath | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Migration SQL failed on the idempotency rerun.' }
$repeat = Query $dsn "SELECT count(*) FROM user_login_log WHERE event_type NOT IN (1,2,3) OR (login_type IS NOT NULL AND login_type NOT IN (1,2,3)) OR btrim(account) = '';"
if ($repeat -ne '0') { throw 'Idempotency verification failed: log enums or account constraints are invalid.' }

Write-Host 'Completed: user_login_log numeric enums, account rename, session_id removal, constraints, and idempotency rerun verified.'
