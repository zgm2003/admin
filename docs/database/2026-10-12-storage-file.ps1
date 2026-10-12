param(
    [switch]$CheckOnly,
    [switch]$OldServicesStopped
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$sqlPath = Join-Path $PSScriptRoot '2026-10-12-storage-file.sql'
$envPath = Join-Path $repoRoot 'server/.env'
$audit = [ordered]@{ migration = 'storage-file'; postgresCommitted = $false; verified = $false }
$auditPath = $null
$originalOptions = $env:PGOPTIONS
$originalAppName = $env:PGAPPNAME

function Invoke-MaintenanceQuery([string]$Query) {
    $output = @(& psql -X -q -A -t -d $script:maintenanceDsn -v ON_ERROR_STOP=1 -c $Query 2> $script:errorPath)
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 查询失败，连接信息不输出；检查本地 maintenance-errors.log。' }
    return (($output -join "`n") | ConvertFrom-Json)
}

function Read-FileState {
    return Invoke-MaintenanceQuery @'
BEGIN READ ONLY;
SET LOCAL statement_timeout='30s';
SELECT json_build_object(
 'database',current_database(), 'schema',current_schema(),
 'tableExists',to_regclass('public.storage_file') IS NOT NULL,
 'menus',(SELECT count(*) FROM permission_menu WHERE deleted_at IS NULL AND code IN ('storage:file:view','storage:file:list','storage:file:detail','storage:file:delete')),
 'platforms',(SELECT json_agg(t) FROM (SELECT id,code,menu_version FROM permission_auth_platform WHERE deleted_at IS NULL ORDER BY id)t),
 'scanSchedule',(SELECT count(*) FROM system_scheduler_schedule WHERE builtin_key='storage.file.scan' AND task_type='storage.file.scan' AND deleted_at IS NULL),
 'pageShape',(SELECT count(*) FROM permission_menu m JOIN permission_auth_platform p ON p.id=m.platform_id JOIN permission_menu parent ON parent.id=m.parent_id WHERE p.code='admin' AND p.deleted_at IS NULL AND parent.code='storage' AND parent.menu_type='directory' AND m.deleted_at IS NULL AND m.code='storage:file:view' AND m.menu_type='page' AND m.path='/storage/file' AND m.component_path='storage/file' AND m.i18n_key='menu.storage.file' AND m.is_hidden=0 AND m.is_enabled=1),
 'actionShape',(SELECT count(*) FROM permission_menu m JOIN permission_menu parent ON parent.id=m.parent_id JOIN permission_auth_platform p ON p.id=m.platform_id WHERE p.code='admin' AND p.deleted_at IS NULL AND parent.code='storage:file:view' AND parent.deleted_at IS NULL AND m.deleted_at IS NULL AND m.code IN ('storage:file:list','storage:file:detail','storage:file:delete') AND m.menu_type='action' AND m.is_hidden=1 AND m.is_enabled=1 AND m.path IS NULL AND m.component_path IS NULL AND m.icon IS NULL AND m.i18n_key IS NULL),
 'defaultGrants',(SELECT count(*) FROM permission_role_menu rm JOIN permission_role r ON r.id=rm.role_id JOIN permission_menu m ON m.id=rm.menu_id JOIN permission_auth_platform p ON p.id=m.platform_id WHERE p.code='admin' AND p.deleted_at IS NULL AND r.code='registered_user' AND r.deleted_at IS NULL AND rm.deleted_at IS NULL AND m.deleted_at IS NULL AND m.code IN ('storage:file:view','storage:file:list','storage:file:detail','storage:file:delete')),
 'originalGrantDigest',(SELECT md5(coalesce(string_agg(rm.id::text||':'||rm.role_id::text||':'||rm.menu_id::text,',' ORDER BY rm.id),'')) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.deleted_at IS NULL AND m.code NOT IN ('storage:file:view','storage:file:list','storage:file:detail','storage:file:delete')),
 'originalGrants',(SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.deleted_at IS NULL AND m.code NOT IN ('storage:file:view','storage:file:list','storage:file:detail','storage:file:delete'))
);
COMMIT;
'@
}


# RESP is used directly so credentials are never passed as command-line arguments.
function Write-RedisCommand([System.IO.Stream]$Stream, [string[]]$Parts) {
    $utf8 = [System.Text.Encoding]::UTF8
    $header = $utf8.GetBytes('*' + $Parts.Count + "`r`n")
    $Stream.Write($header,0,$header.Length)
    foreach ($part in $Parts) {
        $bytes = $utf8.GetBytes($part)
        $header = $utf8.GetBytes('$' + $bytes.Length + "`r`n")
        $Stream.Write($header,0,$header.Length)
        $Stream.Write($bytes,0,$bytes.Length)
        $Stream.WriteByte(13); $Stream.WriteByte(10)
    }
    $Stream.Flush()
    $buffer = [System.Collections.Generic.List[byte]]::new()
    while ($true) {
        $value = $Stream.ReadByte()
        if ($value -lt 0) { throw 'Redis 连接提前关闭。' }
        if ($value -eq 13) {
            if ($Stream.ReadByte() -ne 10) { throw 'Redis 响应格式错误。' }
            break
        }
        $buffer.Add([byte]$value)
        if ($buffer.Count -gt 4096) { throw 'Redis 响应超限。' }
    }
    $reply = $utf8.GetString($buffer.ToArray())
    if ($reply.StartsWith('-')) { throw 'Redis 菜单状态同步失败。' }
    return $reply
}
function Sync-MenuVersion([string]$Url,[long]$PlatformId,[long]$Version) {
    $uri = [Uri]$Url
    if ($uri.Scheme -notin @('redis','rediss')) { throw 'Redis URL 协议错误。' }
    $tcp = [System.Net.Sockets.TcpClient]::new()
    $stream = $null
    try {
        $portNumber = $uri.Port
        if ($portNumber -lt 1) { $portNumber = 6379 }
        $connect = $tcp.ConnectAsync($uri.Host,$portNumber)
        if (-not $connect.Wait(5000)) { throw 'Redis 连接超时。' }
        $tcp.ReceiveTimeout = 5000; $tcp.SendTimeout = 5000
        $stream = $tcp.GetStream()
        if ($uri.Scheme -eq 'rediss') {
            $stream = [System.Net.Security.SslStream]::new($stream,$false)
            $stream.AuthenticateAsClient($uri.Host)
        }
        if ($uri.UserInfo) {
            $credentials = $uri.UserInfo.Split(':',2)
            if ($credentials.Length -ne 2) { throw 'Redis 认证格式错误。' }
            $username = [Uri]::UnescapeDataString($credentials[0])
            $passwordValue = [Uri]::UnescapeDataString($credentials[1])
            $authParts = @('AUTH',$passwordValue)
            if ($username) { $authParts = @('AUTH',$username,$passwordValue) }
            if ((Write-RedisCommand $stream $authParts) -ne '+OK') { throw 'Redis 认证失败。' }
        }
        $dbNumber = $uri.AbsolutePath.Trim('/')
        if ($dbNumber) {
            if ($dbNumber -notmatch '^\d+$') { throw 'Redis 数据库格式错误。' }
            if ((Write-RedisCommand $stream @('SELECT',$dbNumber)) -ne '+OK') { throw 'Redis 数据库选择失败。' }
        }
        $lua = "local v=redis.call('GET',KEYS[1]); if v then local ok,s=pcall(cjson.decode,v); if not ok then return redis.error_reply('corrupt') end; if s.state=='invalidating' then return redis.error_reply('updating') end; if tonumber(s.version)>tonumber(ARGV[1]) then return redis.error_reply('newer') end end; redis.call('SET',KEYS[1],ARGV[2]); return 1"
        $payload = @{schemaVersion=2;state='ready';version=$Version} | ConvertTo-Json -Compress
        if ((Write-RedisCommand $stream @('EVAL',$lua,'1',"authz:menu-state:v1:$PlatformId",[string]$Version,$payload)) -ne ':1') { throw 'Redis 菜单版本同步失败。' }
        $verify = "local s=cjson.decode(redis.call('GET',KEYS[1])); if s.schemaVersion==2 and s.state=='ready' and s.version==tonumber(ARGV[1]) then return 1 end; return 0"
        if ((Write-RedisCommand $stream @('EVAL',$verify,'1',"authz:menu-state:v1:$PlatformId",[string]$Version)) -ne ':1') { throw 'Redis 菜单版本验证失败。' }
    } catch { throw 'PostgreSQL 已提交，Redis 菜单版本衔接未完成；请保持服务停止并重复执行。' }
    finally { if ($stream) { $stream.Dispose() }; $tcp.Dispose() }
}

try {
    foreach ($toolName in @('psql','pg_dump','pg_restore')) { Get-Command $toolName -ErrorAction Stop | Out-Null }
    $fileValues = @{}
    if (Test-Path -LiteralPath $envPath) {
        foreach ($line in Get-Content -LiteralPath $envPath -Encoding UTF8) {
            if ($line -match '^\s*(?:export\s+)?([A-Z][A-Z0-9_]*)\s*=\s*(.*?)\s*$') {
                $fileValues[$Matches[1]] = $Matches[2].Trim().Trim('"').Trim("'")
            }
        }
    }
    $postgresDsn = [Environment]::GetEnvironmentVariable('POSTGRES_DSN')
    if ([string]::IsNullOrWhiteSpace($postgresDsn)) { $postgresDsn = $fileValues['POSTGRES_DSN'] }
    if ([string]::IsNullOrWhiteSpace($postgresDsn)) { throw '缺少 POSTGRES_DSN 配置。' }
    $script:maintenanceDsn = $postgresDsn -replace '(?i)\s+TimeZone=\S+', ''
    $env:PGOPTIONS = '-c search_path=public,pg_catalog'
    $env:PGAPPNAME = 'admin-storage-file-maintenance'
    $runDirectory = Join-Path $env:LOCALAPPDATA ('Admin/maintenance/storage-file/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
    New-Item -ItemType Directory -Path $runDirectory -Force | Out-Null
    $auditPath = Join-Path $runDirectory 'audit.json'
    $script:errorPath = Join-Path $runDirectory 'maintenance-errors.log'
    $audit.startedAt = (Get-Date).ToString('o')
    $audit.sqlSHA256 = (Get-FileHash -LiteralPath $sqlPath -Algorithm SHA256).Hash
    $audit.before = Read-FileState
    if ($CheckOnly) {
        $audit.checkOnly = $true
        $audit.before | ConvertTo-Json
        return
    }
    if (-not $OldServicesStopped) { throw '迁移需要先停止本项目 API/Worker，再显式传入 -OldServicesStopped。' }
    $sessions = Invoke-MaintenanceQuery @'
SELECT json_build_object('otherConnections',count(*),
 'blockingConnections',count(*) FILTER (WHERE NOT (
 application_name='Navicat' AND state='idle' AND xact_start IS NULL
 AND NOT EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_stat_activity.pid))),
 'idleNavicatConnections',count(*) FILTER (WHERE
 application_name='Navicat' AND state='idle' AND xact_start IS NULL
 AND NOT EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_stat_activity.pid))) FROM pg_stat_activity
WHERE datname=current_database() AND backend_type='client backend' AND pid<>pg_backend_pid();
'@
    $audit.clientSessions = $sessions
    if ($sessions.blockingConnections -ne 0) { throw '当前数据库还有服务连接、活动查询或事务，迁移未执行；停止 API/Worker 并结束活动查询/事务后重试。' }
    $backupPath = Join-Path $runDirectory 'public-before.dump'
    & pg_dump --format=custom --schema=public --no-owner --no-privileges --dbname=$script:maintenanceDsn --file=$backupPath 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw '迁移前备份失败，迁移未执行。' }
    & pg_restore --list $backupPath > (Join-Path $runDirectory 'backup-list.txt') 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw '备份归档检查失败，迁移未执行。' }
    & pg_restore --no-owner --no-acl --file=NUL $backupPath 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw '备份完整读取检查失败，迁移未执行。' }
    $audit.backup = $backupPath
    $audit.backupSHA256 = (Get-FileHash -LiteralPath $backupPath -Algorithm SHA256).Hash
    & psql -X -q -d $script:maintenanceDsn --single-transaction -v ON_ERROR_STOP=1 -f $sqlPath 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw '迁移事务失败，PostgreSQL 已回滚；查看本地维护日志。' }
    $audit.postgresCommitted = $true
    $audit.after = Read-FileState
    if (-not $audit.after.tableExists -or $audit.after.menus -ne 4 -or $audit.after.scanSchedule -ne 1 -or $audit.after.pageShape -ne 1 -or $audit.after.actionShape -ne 3 -or $audit.after.defaultGrants -ne 4 -or $audit.before.originalGrants -ne $audit.after.originalGrants -or $audit.before.originalGrantDigest -ne $audit.after.originalGrantDigest) {
        throw 'PostgreSQL 已提交，但目标结构/原授权验证失败。'
    }
    # Services remain stopped until exact platform menu state is synchronized.
    $redisUrl = [Environment]::GetEnvironmentVariable('REDIS_URL')
    if ([string]::IsNullOrWhiteSpace($redisUrl)) { $redisUrl = $fileValues['REDIS_URL'] }
    if ([string]::IsNullOrWhiteSpace($redisUrl)) { throw 'PostgreSQL 已提交，但缺少 REDIS_URL，菜单版本衔接未完成。' }
    foreach ($platform in $audit.after.platforms) {
        if ($platform.code -eq 'admin') { Sync-MenuVersion $redisUrl $platform.id $platform.menu_version }
    }
    $audit.redisSynchronized = $true
    $audit.verified = $true
    $snapshotTemp = Join-Path $runDirectory 'current.sql'
    & pg_dump --schema-only --schema=public --no-owner --no-privileges --dbname=$script:maintenanceDsn --file=$snapshotTemp 2> $script:errorPath
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 已提交且验证通过，但 current.sql 导出失败。' }
    Copy-Item -LiteralPath $snapshotTemp -Destination (Join-Path $PSScriptRoot 'current.sql') -Force
    $audit.snapshotUpdated = $true
    $audit.after | ConvertTo-Json
    Write-Output "迁移完成；备份与审计：$runDirectory"
}
catch {
    $audit.failed = $true
    $audit.error = $_.Exception.Message
    Write-Error $_.Exception.Message -ErrorAction Continue
    exit 1
}
finally {
    $env:PGOPTIONS = $originalOptions
    $env:PGAPPNAME = $originalAppName
    $audit.finishedAt = (Get-Date).ToString('o')
    if ($null -ne $auditPath) { $audit | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $auditPath -Encoding UTF8 }
}
