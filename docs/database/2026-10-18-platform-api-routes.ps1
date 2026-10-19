[CmdletBinding()]
param([switch]$ServicesStopped)
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '请先停止全部 API/Worker 实例，再传入 -ServicesStopped；脚本不会操作服务' }
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$audit = Join-Path $env:LOCALAPPDATA ('Admin/maintenance/platform-api-routes/' + [DateTime]::Now.ToString('yyyyMMdd-HHmmss-fff') + '-' + [Guid]::NewGuid().ToString('N'))
$oldLocation = Get-Location
try {
    Set-Location -LiteralPath (Join-Path $root 'server')
    & go run ./cmd/migrate-platform-routes -services-stopped -audit-dir $audit
    if ($LASTEXITCODE -ne 0) { throw "迁移未完成；禁止启动服务，查看 $audit/audit.json，修复后可安全重复执行" }
} finally {
    Set-Location -LiteralPath $oldLocation
}
