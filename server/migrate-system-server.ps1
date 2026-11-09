param(
    [switch]$ServicesStopped,
    [Parameter(Mandatory = $true)][string]$AuditDirectory
)
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '先停止全部 API/Worker，再传入 -ServicesStopped；脚本不会操作服务' }
$auditPath = [System.IO.Path]::GetFullPath($AuditDirectory)
Push-Location $PSScriptRoot
try {
    & go run ./cmd/migrate-system-server -services-stopped -audit-dir $auditPath
    if ($LASTEXITCODE -ne 0) { throw '服务器权限迁移未完成，请按工具输出和审计修复；不要启动服务' }
} finally {
    Pop-Location
}
