param(
    [Parameter(Mandatory = $true)][switch]$ServicesStopped,
    [string]$AuditDirectory = (Join-Path $env:LOCALAPPDATA ("Admin/maintenance/status-columns/" + (Get-Date -Format 'yyyyMMdd-HHmmss-fff')))
)

$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw 'Pause this project API and Worker before running this migration.' }
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$server = Join-Path $root 'server'
$tool = Join-Path $server 'tmp/migrate-status-columns.exe'
$committed = $false
$saved = @{}
Push-Location $server
try {
    New-Item -ItemType Directory -Path (Split-Path $tool) -Force | Out-Null
    & go build -o $tool ./cmd/migrate-status-columns
    if ($LASTEXITCODE -ne 0) { throw 'Maintenance command build failed.' }
    # This output contains credentials: capture in memory only, never log it.
    $pg = (& $tool -mode pg-env | ConvertFrom-Json)
    if ($LASTEXITCODE -ne 0) { throw 'Read local PostgreSQL configuration failed.' }
    foreach ($property in $pg.PSObject.Properties) {
        $saved[$property.Name] = [Environment]::GetEnvironmentVariable($property.Name, 'Process')
        [Environment]::SetEnvironmentVariable($property.Name, $property.Value, 'Process')
    }
    $saved['PGOPTIONS'] = $env:PGOPTIONS
    $env:PGOPTIONS = '-c search_path=public -c statement_timeout=120000'
    New-Item -ItemType Directory -Path $AuditDirectory -ErrorAction Stop | Out-Null
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    & icacls.exe $AuditDirectory /inheritance:r /grant:r ($identity + ':(OI)(CI)F') 'SYSTEM:(OI)(CI)F' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Restrict maintenance audit directory access failed.' }
    & $tool -mode preflight | Set-Content -Encoding utf8 (Join-Path $AuditDirectory 'cache-preflight.json')
    if ($LASTEXITCODE -ne 0) { throw 'Cache preflight failed; database not changed.' }
    & $tool -mode inspect | Set-Content -Encoding utf8 (Join-Path $AuditDirectory 'before.json')
    if ($LASTEXITCODE -ne 0) { throw 'Database preflight failed; database not changed.' }
    & pg_dump --format=custom --file (Join-Path $AuditDirectory 'before.dump')
    if ($LASTEXITCODE -ne 0) { throw 'Database backup failed; database not changed.' }
    & pg_restore --list (Join-Path $AuditDirectory 'before.dump') | Set-Content -Encoding utf8 (Join-Path $AuditDirectory 'backup-contents.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Database backup archive validation failed.' }
    & psql -X --single-transaction --set=ON_ERROR_STOP=1 --file (Join-Path $PSScriptRoot '2026-10-23-status-columns.sql') *> (Join-Path $AuditDirectory 'migration.log')
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL migration failed and rolled back.' }
    $committed = $true
    & $tool -mode cleanup -audit $AuditDirectory | Set-Content -Encoding utf8 (Join-Path $AuditDirectory 'cache-cleanup.json')
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 已提交、Redis cleanup 未完成；保持服务停止，检查审计后在新审计目录安全重跑。' }
    & $tool -mode verify | Set-Content -Encoding utf8 (Join-Path $AuditDirectory 'after.json')
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 已提交；结构或 Redis 定向验证未通过。' }
    & psql -X --single-transaction --set=ON_ERROR_STOP=1 --file (Join-Path $PSScriptRoot '2026-10-23-status-columns.sql') *> (Join-Path $AuditDirectory 'replay.log')
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 已提交；幂等复跑未通过。' }
    & $tool -mode verify | Set-Content -Encoding utf8 (Join-Path $AuditDirectory 'replayed.json')
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 已提交；幂等复跑后验证未通过。' }
    $before = Get-Content -Raw -Encoding utf8 (Join-Path $AuditDirectory 'before.json') | ConvertFrom-Json
    $after = Get-Content -Raw -Encoding utf8 (Join-Path $AuditDirectory 'after.json') | ConvertFrom-Json
    $replayed = Get-Content -Raw -Encoding utf8 (Join-Path $AuditDirectory 'replayed.json') | ConvertFrom-Json
    foreach ($entry in $before.digests.PSObject.Properties) {
        if ($after.digests.($entry.Name) -ne $entry.Value -or $replayed.digests.($entry.Name) -ne $entry.Value) {
            throw "PostgreSQL 已提交；业务或版本事实校验失败：$($entry.Name)"
        }
    }
    & pg_dump --schema-only --schema=public --no-owner --no-privileges --file (Join-Path $AuditDirectory 'current.sql')
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 已提交；结构快照导出失败。' }
    Copy-Item -LiteralPath (Join-Path $AuditDirectory 'current.sql') -Destination (Join-Path $PSScriptRoot 'current.sql')
    Get-FileHash -Algorithm SHA256 (Join-Path $AuditDirectory 'before.dump'), (Join-Path $PSScriptRoot '2026-10-23-status-columns.sql') |
        Select-Object Path, Hash | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $AuditDirectory 'sha256.json')
    Write-Output "Migration and replay verified. Audit: $AuditDirectory"
} catch {
    if ($committed) { Write-Error ("PostgreSQL 已提交；服务应保持停止直到排查完成。" + $_.Exception.Message) }
    throw
} finally {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
    Pop-Location
}
