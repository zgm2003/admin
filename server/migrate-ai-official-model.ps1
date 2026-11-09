param(
    [switch]$ServicesStopped,
    [Parameter(Mandatory = $true)][string]$AuditDirectory
)
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw 'Stop all API/Worker instances before running this migration; this script does not stop services.' }
$auditPath = [System.IO.Path]::GetFullPath($AuditDirectory)
Push-Location $PSScriptRoot
try {
    & go run ./cmd/migrate-ai-official-model -services-stopped -audit-dir $auditPath
    if ($LASTEXITCODE -ne 0) { throw 'AI official model permission migration did not complete; do not start services until the audit is fixed.' }
} finally {
    Pop-Location
}
