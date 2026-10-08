<#
.SYNOPSIS
    Offline Mail recipient-rule numeric migration with backup and audit checks.
.DESCRIPTION
    Stop this project's API and Worker first. This runner never stops services,
    deletes Redis keys, sends mail, or restarts old/new binaries.
#>
[CmdletBinding()]
param([switch]$OldAPIStopped)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $OldAPIStopped) {
    throw 'Refusing migration: stop API/Worker and explicitly pass -OldAPIStopped.'
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverRoot = Join-Path $repoRoot 'server'
$sqlPath = Join-Path $PSScriptRoot '2026-10-08-mail-recipient-rule-numeric.sql'
$snapshotPath = Join-Path $PSScriptRoot 'current.sql'
if (-not (Test-Path -LiteralPath $sqlPath)) { throw 'Migration SQL is missing.' }
foreach ($name in @('go', 'psql', 'pg_dump', 'pg_restore')) {
    if (-not (Get-Command $name -CommandType Application -ErrorAction SilentlyContinue)) {
        throw "Required executable is missing: $name"
    }
}

function Read-Setting([string]$Key) {
    $value = [Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    foreach ($raw in Get-Content -LiteralPath (Join-Path $serverRoot '.env') -Encoding UTF8) {
        $line = $raw.Trim()
        if ($line.StartsWith($Key + '=')) {
            return $line.Substring($Key.Length + 1).Trim().Trim('"').Trim("'")
        }
    }
    throw "Required environment setting is missing: $Key"
}

$dsn = Read-Setting 'POSTGRES_DSN'
# TimeZone is a pgx runtime parameter, not a libpq connection keyword.
$dsn = [regex]::Replace($dsn, '(?i)(?:^|\s+)TimeZone=(?:''(?:[^''\\]|\\.)*''|(?:\\.|[^\s])+)', '')
$backupRoot = Join-Path $env:LOCALAPPDATA 'Admin\backups'
$backupDir = Join-Path $backupRoot ('mail-rule-numeric-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Path $backupDir | Out-Null
$helperPath = Join-Path $backupDir 'mail-recipient-rule-numeric-migration.exe'
$dumpPath = Join-Path $backupDir 'public-before.dump'
$afterSchemaPath = Join-Path $backupDir 'public-after.sql'
$sqlCommitted = $false

function Inspect-State([string]$Mode) {
    $arguments = @('-mode', $Mode)
    if ($Mode -eq 'sync') { $arguments += '-old-api-stopped' }
    $lines = @(& $helperPath @arguments)
    if ($LASTEXITCODE -ne 0) { throw "Mail state $Mode failed (exit $LASTEXITCODE)." }
    $json = ($lines -join "`n").Trim()
    if ($json -eq '') { throw 'Mail state command returned no audit result.' }
    return ($json | ConvertFrom-Json)
}

function Write-Audit([string]$Name, $Value) {
    $json = $Value | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText((Join-Path $backupDir $Name), $json, (New-Object Text.UTF8Encoding($false)))
}

function Apply-SQL {
    # Pin public explicitly even if the application DSN overrides search_path.
    & psql -X --no-password -v ON_ERROR_STOP=1 --dbname=$dsn -c 'SET search_path TO public;' -f $sqlPath
    if ($LASTEXITCODE -ne 0) {
        throw 'SQL execution failed or its outcome is uncertain; inspect the database before restarting any service.'
    }
}

function Assert-NumericState($State) {
    if ($State.scopeType -ne 'smallint' -or $State.actionType -ne 'smallint' -or
        -not $State.mailStateFound -or $State.mailState -ne 'ready' -or
        $State.mailStateGeneration -ne $State.mailGeneration) {
        throw 'Numeric schema or PostgreSQL/Redis generation verification failed.'
    }
}

Push-Location $serverRoot
try {
    & go build -o $helperPath ./cmd/mail-recipient-rule-numeric-migration
    if ($LASTEXITCODE -ne 0) { throw 'Maintenance helper build failed.' }
    $before = Inspect-State 'inspect'
    Write-Audit 'before.json' $before
    if ($before.mailStateFound -and ($before.mailState -ne 'ready' -or $before.mailStateGeneration -gt $before.mailGeneration)) {
        throw 'Mail cache is mutating or ahead of PostgreSQL; migration is refused.'
    }
    $legacy = $before.scopeType -eq 'character varying' -and $before.actionType -eq 'character varying'
    $numeric = $before.scopeType -eq 'smallint' -and $before.actionType -eq 'smallint'
    if (-not ($legacy -or $numeric)) { throw 'Unsupported or partially migrated rule schema.' }

    & pg_dump --format=custom --schema=public --no-owner --no-privileges --no-password --dbname=$dsn --file=$dumpPath
    if ($LASTEXITCODE -ne 0) { throw 'Database backup failed; no migration was executed.' }
    & pg_restore --list $dumpPath | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Backup archive verification failed; no migration was executed.' }
    $hash = (Get-FileHash -LiteralPath $dumpPath -Algorithm SHA256).Hash
    [IO.File]::WriteAllText((Join-Path $backupDir 'public-before.sha256'), $hash, (New-Object Text.UTF8Encoding($false)))
    Write-Host "Backup verified: $dumpPath SHA256=$hash"

    Apply-SQL
    $sqlCommitted = $true
    $after = Inspect-State 'sync'
    Write-Audit 'after.json' $after
    Assert-NumericState $after
    $expectedGeneration = [long]$before.mailGeneration
    if ($legacy) { $expectedGeneration += 1 }
    if ($after.mailGeneration -ne $expectedGeneration -or $after.rowCount -ne $before.rowCount -or
        $after.ruleFactsHash -ne $before.ruleFactsHash -or $after.controlHash -ne $before.controlHash) {
        throw 'Rule preservation, generation delta or unrelated facts verification failed.'
    }

    Apply-SQL
    $repeat = Inspect-State 'sync'
    Write-Audit 'repeat.json' $repeat
    Assert-NumericState $repeat
    foreach ($key in @('scopeType','actionType','rowCount','ruleFactsHash','mailGeneration','controlHash','mailOutboxHash','mailStateFound','mailState','mailStateGeneration')) {
        if ($repeat.$key -ne $after.$key) { throw "Idempotency verification failed: $key" }
    }

    & pg_dump --schema-only --schema=public --no-owner --no-privileges --no-password --dbname=$dsn --file=$afterSchemaPath
    if ($LASTEXITCODE -ne 0) { throw 'Schema snapshot export failed.' }
    Copy-Item -LiteralPath $afterSchemaPath -Destination $snapshotPath
    Write-Host "Completed: scope/action numeric, rules preserved, mail generation=$($after.mailGeneration), two-pass SQL/Redis verification passed."
    Write-Host "Audit directory: $backupDir. API/Worker remain stopped; start the new version manually."
} catch {
    if ($sqlCommitted) {
        throw "PostgreSQL migration committed; remaining publication/verification failed. Keep services stopped and safely rerun. Audit: $backupDir. $($_.Exception.Message)"
    }
    throw
} finally {
    Pop-Location
}
