# Forward-only offline runner. Does not stop services or accept arbitrary Redis patterns.
[CmdletBinding()]
param(
 [switch]$OldServicesStopped,
 [string]$AuditDirectory = (Join-Path $env:LOCALAPPDATA 'Admin\maintenance\remove-system-dictionary')
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $OldServicesStopped) { throw 'Stop API and Worker first, then pass -OldServicesStopped. This runner never stops services.' }
$workspace = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$server = Join-Path $workspace 'server'
$running = @(Get-CimInstance Win32_Process | Where-Object {
 $_.ExecutablePath -and ($_.ExecutablePath -eq (Join-Path $server 'api.exe') -or $_.ExecutablePath -eq (Join-Path $server 'worker.exe')) -or
 $_.CommandLine -and ($_.CommandLine -match '(?i)go(?:\.exe)?\s+run\s+\.?[/\\]cmd[/\\](api|worker)' -or $_.CommandLine -match '(?i)go-build.*[/\\](api|worker)(?:\.exe)?(?:\s|$)')
})
if ($running.Count -gt 0) { throw 'Detected running API/Worker. Stop them before offline migration.' }
$audit = [IO.Path]::GetFullPath($AuditDirectory)
New-Item -ItemType Directory -Path $audit -Force | Out-Null
$command = Join-Path $audit 'remove-system-dictionary.exe'
$manifest = Join-Path $audit 'manifest.json'
$sql = Join-Path $PSScriptRoot '2026-10-10-remove-system-dictionary.sql'
# Keep manifest across retries: it stores the original menu_version expectations.
Push-Location $server
try {
 & go build -o $command ./cmd/remove-system-dictionary
 if ($LASTEXITCODE -ne 0) { throw 'Build maintenance command failed; no migration executed.' }
 & $command -old-services-stopped -sql $sql -audit $manifest
 if ($LASTEXITCODE -ne 0) { throw "Maintenance incomplete (exit=$LASTEXITCODE). Preserve $manifest and rerun before starting services. PostgreSQL may already be committed; see command output." }
 Write-Host "Completed. Durable audit: $manifest. Start only the new API/Worker version after reviewing the audit."
} finally { Pop-Location }
