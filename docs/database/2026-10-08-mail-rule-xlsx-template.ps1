[CmdletBinding()]
param([switch]$OldAPIStopped)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $OldAPIStopped) { throw '请确认本项目 API/Worker 已停止，再传入 -OldAPIStopped。脚本不会停止进程。' }

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$server = Join-Path $root 'server'
$envPath = Join-Path $server '.env'
$values = @{}
foreach ($line in Get-Content -LiteralPath $envPath) {
    if ($line -match '^\s*(POSTGRES_DSN|HTTP_ADDR)\s*=\s*(.+?)\s*$') {
        $values[$Matches[1]] = $Matches[2].Trim().Trim('"').Trim("'")
    }
}
if (-not $values.ContainsKey('POSTGRES_DSN') -or -not $values.ContainsKey('HTTP_ADDR')) { throw '缺少数据库或 HTTP 配置。' }
$publicDSN = $values['POSTGRES_DSN']
$dbPassword = $env:PGPASSWORD
if ($publicDSN -match '^(?i:postgres(?:ql)?://)') {
    $dsn = [Uri]$publicDSN
    if ($dsn.UserInfo -match '^[^:]+:(.*)$') { $dbPassword = [Uri]::UnescapeDataString($Matches[1]) }
    $withoutPassword = [UriBuilder]$dsn
    $withoutPassword.Password = ''
    if ($dsn.Query -match '(?i)(?:[?&])(?:password|passfile|sslpassword)=') { throw '数据库 URI 查询参数含凭据，请改为标准用户信息格式后执行。' }
    $publicDSN = $withoutPassword.Uri.AbsoluteUri
} else {
    if ($publicDSN -match '(?i)(?:^|\s)(?:password|passfile|sslpassword)\s*=') { throw '关键字 DSN 不得将凭据传给子进程参数；请使用 PGPASSWORD 或标准 PostgreSQL URI。' }
    # pg_dump does not accept the GORM/libpq session option used by the app DSN.
    $publicDSN = $publicDSN -replace '(?i)(^|\s)TimeZone(?:\s|=)+[^\s]+', '$1'
}
$port = [int]($values['HTTP_ADDR'].Split(':')[-1])
if (Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue) { throw '项目 API 端口仍在监听，请先停止 API。' }
$running = @(Get-CimInstance Win32_Process | Where-Object {
    ($_.ExecutablePath -and $_.ExecutablePath.StartsWith($root + '\', [StringComparison]::OrdinalIgnoreCase) -and $_.Name -match '^(api|worker)(\.exe)?$') -or
    ($_.Name -eq 'go.exe' -and $_.CommandLine -match '(?i)\brun\b' -and $_.CommandLine -match '(?i)([/\\]cmd[/\\](api|worker)|\.\s*[/\\]cmd[/\\](api|worker))') -or
    ($_.Name -match '^(api|worker)\.exe$' -and $_.ExecutablePath -match '(?i)[/\\]go-build[^/\\]*[/\\]')
})
if ($running.Count -gt 0) { throw '检测到可能的 API/Worker 进程。请核实并手动停止后重试；脚本不会终止任何进程。' }

$backup = Join-Path $env:LOCALAPPDATA ('Admin/backups/mail-rule-xlsx-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,8))
New-Item -ItemType Directory -Path $backup | Out-Null
$dump = Join-Path $backup 'public-before.dump'
$oldPassword = $env:PGPASSWORD
$oldOptions = $env:PGOPTIONS
try {
    if ($dbPassword) { $env:PGPASSWORD = $dbPassword }
    $env:PGOPTIONS = '-c timezone=UTC'
    & pg_dump --no-password --dbname $publicDSN --schema public --format custom --file $dump 2> (Join-Path $backup 'pg-dump.stderr.txt')
    if ($LASTEXITCODE -ne 0) { throw 'pg_dump 失败，未发布模板。详情保存在备份目录。' }
    & pg_restore --list $dump > (Join-Path $backup 'restore-list.txt')
    if ($LASTEXITCODE -ne 0) { throw 'pg_restore --list 失败，未发布模板。' }
    (Get-FileHash -LiteralPath $dump -Algorithm SHA256).Hash | Set-Content -LiteralPath (Join-Path $backup 'backup.sha256')
} finally {
    $env:PGPASSWORD = $oldPassword
    $env:PGOPTIONS = $oldOptions
    $dbPassword = $null
    $values.Clear()
}

Push-Location $server
try {
    $binary = Join-Path $backup 'publish.exe'
    & go build -o $binary ./cmd/mail-rule-xlsx-template-publish
    if ($LASTEXITCODE -ne 0) { throw '维护命令构建失败，未发布模板。' }
    & $binary -mode inspect > (Join-Path $backup 'before.json')
    if ($LASTEXITCODE -ne 0) { throw '发布前事实检查失败。' }
    & $binary -mode publish -old-api-stopped > (Join-Path $backup 'manifest.json')
    if ($LASTEXITCODE -ne 0) { throw "发布未完成，可能已有上传或配置提交；请保留 manifest 并重跑。备份：$backup" }
    & $binary -mode verify > (Join-Path $backup 'verified.json')
    if ($LASTEXITCODE -ne 0) { throw '发布后的标准模板服务、Redis 状态或 COS 下载验证失败。请保留 manifest 重跑。' }
    & $binary -mode inspect > (Join-Path $backup 'after.json')
    if ($LASTEXITCODE -ne 0) { throw '发布后事实检查失败。' }
    & $binary -mode publish -old-api-stopped > (Join-Path $backup 'repeat-manifest.json')
    if ($LASTEXITCODE -ne 0) { throw '重复发布验证失败。' }
    & $binary -mode inspect > (Join-Path $backup 'repeat.json')
    if ($LASTEXITCODE -ne 0) { throw '重复发布后检查失败。' }
    if ((Get-Content -LiteralPath (Join-Path $backup 'after.json') -Raw) -cne (Get-Content -LiteralPath (Join-Path $backup 'repeat.json') -Raw)) { throw '重复执行改变了数据库事实。' }
    Write-Output "Excel 模板发布与幂等验证完成。备份及审计：$backup"
    Write-Output '未改表结构、未删除旧 COS 对象、未清 Redis。由维护者启动新版本 API/Worker。'
} finally { Pop-Location }
