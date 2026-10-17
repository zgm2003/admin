[CmdletBinding()]
param(
    [switch]$CheckOnly,
    [switch]$ServicesStopped,
    [string]$AuditDirectory
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$serverRoot = Join-Path $repoRoot 'server'
$sqlPath = Join-Path $PSScriptRoot '2026-10-17-payment-redeem-code-menu.sql'
$oldDsn = $env:PAYMENT_MENU_MAINTENANCE_DSN
$oldRedisUrl = $env:PAYMENT_MENU_MAINTENANCE_REDIS_URL
$oldAppName = $env:PGAPPNAME
$oldOptions = $env:PGOPTIONS
$audit = [ordered]@{
    migration = 'payment-redeem-code-menu'
    postgresCommitted = $false
    redisSynchronized = $false
    verified = $false
}
$auditPath = $null
$helperDirectory = $null

function Read-EnvFile([string]$Path) {
    $values = @{}
    if (-not (Test-Path -LiteralPath $Path)) { return $values }
    foreach ($raw in Get-Content -LiteralPath $Path -Encoding UTF8) {
        $line = $raw.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        if ($line -match '^(?:export\s+)?([A-Z][A-Z0-9_]*)\s*=\s*(.*?)\s*$') {
            $values[$Matches[1]] = $Matches[2].Trim().Trim('"').Trim("'")
        }
    }
    return $values
}

function Effective([hashtable]$FileValues, [string]$Key) {
    $value = [Environment]::GetEnvironmentVariable($Key)
    if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    if ($FileValues.ContainsKey($Key)) { return [string]$FileValues[$Key] }
    return ''
}

function Write-Audit {
    if ($null -eq $auditPath) { return }
    $audit.finishedAt = [DateTime]::UtcNow.ToString('o')
    $audit | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $auditPath -Encoding UTF8
}

try {
    foreach ($tool in @('go', 'psql', 'pg_dump', 'pg_restore')) { Get-Command $tool -ErrorAction Stop | Out-Null }
    if (-not (Test-Path -LiteralPath $sqlPath)) { throw '找不到兑换码菜单 migration SQL。' }

    $values = Read-EnvFile (Join-Path $serverRoot '.env')
    $dsn = Effective $values 'POSTGRES_DSN'
    $redisUrl = Effective $values 'REDIS_URL'
    if ([string]::IsNullOrWhiteSpace($dsn)) { throw '缺少 POSTGRES_DSN。' }
    if ([string]::IsNullOrWhiteSpace($redisUrl)) { throw '缺少 REDIS_URL。' }
    $dsn = $dsn -replace '(?i)(^|\s+)TimeZone=\S+', '$1'

    if ([string]::IsNullOrWhiteSpace($AuditDirectory)) {
        $AuditDirectory = Join-Path $env:LOCALAPPDATA ('Admin/maintenance/payment-redeem-code-menu/' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss-fff'))
    }
    $AuditDirectory = [IO.Path]::GetFullPath($AuditDirectory)
    if ((Test-Path -LiteralPath $AuditDirectory) -and @(Get-ChildItem -LiteralPath $AuditDirectory -Force).Count -gt 0) {
        throw '审计目录必须为空。'
    }
    New-Item -ItemType Directory -Path $AuditDirectory -Force | Out-Null
    $auditPath = Join-Path $AuditDirectory 'audit.json'
    $audit.startedAt = [DateTime]::UtcNow.ToString('o')
    $audit.sqlSHA256 = (Get-FileHash -LiteralPath $sqlPath -Algorithm SHA256).Hash
    $audit.auditDirectory = $AuditDirectory

    $backupPath = Join-Path $AuditDirectory 'public-before.dump'
    if (-not $CheckOnly) {
        if (-not $ServicesStopped) { throw '请先停止 API/Worker，再传入 -ServicesStopped。' }
        $env:PGAPPNAME = 'admin-payment-redeem-menu-maintenance'
        $env:PGOPTIONS = '-c search_path=public,pg_catalog'
        $sessionCount = @(& psql -X -w -q -A -t --dbname $dsn -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND pid<>pg_backend_pid()" 2>&1)
        if ($LASTEXITCODE -ne 0) { throw '无法检查 PostgreSQL 连接；未执行 migration。' }
        if ([int64]($sessionCount -join '').Trim() -ne 0) { throw '业务库仍有其他客户端连接；请保持服务停止后重试。' }
        & pg_dump --format=custom --no-owner --no-privileges --dbname $dsn --schema public --file $backupPath 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 备份失败；未执行 migration。' }
        & pg_restore --list $backupPath 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw '备份归档校验失败；未执行 migration。' }
        & pg_restore --no-owner --no-privileges --file NUL $backupPath 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw '备份完整读取校验失败；未执行 migration。' }
        $audit.backup = $backupPath
        $audit.backupSHA256 = (Get-FileHash -LiteralPath $backupPath -Algorithm SHA256).Hash
    }

    $helperDirectory = Join-Path $serverRoot ('tmp/payment-redeem-code-menu-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $helperDirectory -Force | Out-Null
    $helperPath = Join-Path $helperDirectory 'main.go'
    $helper = @'
package main

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "net/url"
    "os"
    "path/filepath"
    "reflect"
    "sort"
    "strings"
    "time"

    appdatabase "admin/server/internal/database"
    permissionstate "admin/server/internal/module/permission/state"
    projectredis "admin/server/internal/redis"
    "gorm.io/gorm"
)

type version struct {
    UserID  int64 `json:"userId"`
    Version int64 `json:"version"`
}

type facts struct {
    Database                 string    `json:"database"`
    Schema                   string    `json:"schema"`
    AdminID                  int64     `json:"adminId"`
    MenuVersion              int64     `json:"menuVersion"`
    RegisteredRoleID         int64     `json:"registeredRoleId"`
    TargetMenuCount          int64     `json:"targetMenuCount"`
    RedeemGrantCount         int64     `json:"redeemGrantCount"`
    TargetMenuDigest         string    `json:"targetMenuDigest"`
    UnrelatedGrantDigest     string    `json:"unrelatedGrantDigest"`
    RegisteredUsers           []version `json:"registeredUsers"`
    AccessVersions            []version `json:"accessVersions"`
}

type audit struct {
    Migration          string  `json:"migration"`
    DatabaseIdentity   string  `json:"databaseIdentity"`
    RedisIdentity      string  `json:"redisIdentity"`
    SQLSHA256          string  `json:"sqlSHA256"`
    BackupSHA256       string  `json:"backupSHA256,omitempty"`
    Before             *facts  `json:"before,omitempty"`
    After              *facts  `json:"after,omitempty"`
    PostgresCommitted  bool    `json:"postgresCommitted"`
    RedisSynchronized  bool    `json:"redisSynchronized"`
    Verified           bool    `json:"verified"`
    FailureStage       string  `json:"failureStage,omitempty"`
    Error              string  `json:"error,omitempty"`
    Replayed           bool    `json:"replayed,omitempty"`
    StartedAt          string  `json:"startedAt"`
    FinishedAt         string  `json:"finishedAt"`
}

func main() {
    sqlPath := flag.String("sql", "", "migration SQL")
    auditDir := flag.String("audit", "", "audit directory")
    backupPath := flag.String("backup", "", "backup archive")
    checkOnly := flag.Bool("check", false, "read-only check")
    flag.Parse()
    resultPath := filepath.Join(*auditDir, "result.json")
    report := audit{Migration: "payment-redeem-code-menu", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
    fail := func(stage string, err error) {
        report.FailureStage = stage
        report.Error = err.Error()
        report.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
        _ = writeAudit(resultPath, report)
        fmt.Fprintln(os.Stderr, err.Error())
        os.Exit(1)
    }
    if *sqlPath == "" || *auditDir == "" {
        fail("input", errors.New("sql and audit paths are required"))
    }
    sqlBytes, err := os.ReadFile(*sqlPath)
    if err != nil { fail("input", fmt.Errorf("read migration SQL: %w", err)) }
    sum := sha256.Sum256(sqlBytes)
    report.SQLSHA256 = strings.ToUpper(hex.EncodeToString(sum[:]))
    if *backupPath != "" {
        if backupBytes, readErr := os.ReadFile(*backupPath); readErr == nil {
            backupSum := sha256.Sum256(backupBytes)
            report.BackupSHA256 = strings.ToUpper(hex.EncodeToString(backupSum[:]))
        }
    }

    dsn := firstEnv("PAYMENT_MENU_MAINTENANCE_DSN", "POSTGRES_DSN")
    redisURL := firstEnv("PAYMENT_MENU_MAINTENANCE_REDIS_URL", "REDIS_URL")
    if dsn == "" || redisURL == "" { fail("input", errors.New("maintenance connection settings are missing")) }
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
    defer cancel()
    connection, err := appdatabase.Open(ctx, dsn)
    if err != nil { fail("postgres", err) }
    defer connection.Close()
    rc, err := projectredis.Open(ctx, redisURL)
    if err != nil { fail("redis", err) }
    defer rc.Close()

    report.DatabaseIdentity, err = databaseIdentity(ctx, connection.GORM)
    if err != nil { fail("postgres", err) }
    report.RedisIdentity, err = redisIdentity(redisURL)
    if err != nil { fail("redis", err) }
    before, readErr := readFacts(ctx, connection.GORM)
    if readErr != nil { fail("postgres", readErr) }
    report.Before = &before
    if *checkOnly {
        if err = verifyRedis(ctx, rc, *report.Before); err != nil { fail("redis", err) }
        report.RedisSynchronized = true
        report.After = report.Before
        report.Verified = true
        report.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
        if err = writeAudit(resultPath, report); err != nil { fail("audit", err) }
        return
    }

    // Normalize old ready states before the transaction. This only writes the
    // versions already present in PostgreSQL, so a rolled-back migration leaves
    // Redis consistent with the previous database facts.
    if err = syncRedis(ctx, rc, *report.Before); err != nil { fail("redis-before", err) }
    if err = applyMigration(ctx, connection.GORM, sqlBytes); err != nil { fail("postgres", fmt.Errorf("migration transaction failed; commit state is unknown: %w", err)) }
    report.PostgresCommitted = true
    after, readErr := readFacts(ctx, connection.GORM)
    if readErr != nil { fail("postgres-after", fmt.Errorf("PostgreSQL committed; read-back failed: %w", readErr)) }
    report.After = &after
    if err = verifyTransition(*report.Before, *report.After); err != nil { fail("postgres-after", fmt.Errorf("PostgreSQL committed; transition verification failed: %w", err)) }
    if err = syncRedis(ctx, rc, *report.After); err != nil { fail("redis-after", fmt.Errorf("PostgreSQL committed; Redis ready-state synchronization incomplete: %w", err)) }
    report.RedisSynchronized = true
    if err = verifyRedis(ctx, rc, *report.After); err != nil { fail("redis-after", fmt.Errorf("PostgreSQL committed; Redis verification failed: %w", err)) }

    // Re-run the forward SQL and prove that neither menu/access facts nor
    // versions change on an idempotent replay.
    if err = applyMigration(ctx, connection.GORM, sqlBytes); err != nil { fail("replay", fmt.Errorf("PostgreSQL committed; idempotent replay failed: %w", err)) }
    replay, err := readFacts(ctx, connection.GORM)
    if err != nil { fail("replay", fmt.Errorf("PostgreSQL committed; replay read-back failed: %w", err)) }
    if !sameFacts(*report.After, replay) { fail("replay", errors.New("PostgreSQL committed; idempotent replay changed menu or access facts")) }
    if err = syncRedis(ctx, rc, replay); err != nil { fail("redis-replay", fmt.Errorf("PostgreSQL committed; Redis replay synchronization incomplete: %w", err)) }
    if err = verifyRedis(ctx, rc, replay); err != nil { fail("redis-replay", fmt.Errorf("PostgreSQL committed; Redis replay verification failed: %w", err)) }
    report.Replayed = true
    report.Verified = true
    report.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
    if err = writeAudit(resultPath, report); err != nil { fail("audit", err) }
}

func firstEnv(keys ...string) string {
    for _, key := range keys { if value := strings.TrimSpace(os.Getenv(key)); value != "" { return value } }
    return ""
}

func databaseIdentity(ctx context.Context, db *gorm.DB) (string, error) {
    var value struct { Database string; Schema string }
    if err := db.WithContext(ctx).Raw("SELECT current_database() AS database,current_schema() AS schema").Scan(&value).Error; err != nil { return "", err }
    if value.Database == "" || value.Schema == "" { return "", errors.New("database identity is empty") }
    return value.Database + "/" + value.Schema, nil
}

func redisIdentity(raw string) (string, error) {
    parsed, err := url.Parse(raw)
    if err != nil || parsed.Host == "" { return "", errors.New("Redis identity is invalid") }
    parsed.User = nil
    parsed.RawQuery = ""
    parsed.Fragment = ""
    return parsed.String(), nil
}

func readFacts(ctx context.Context, db *gorm.DB) (facts, error) {
    var f facts
    var identity struct { Database string; Schema string }
    if err := db.WithContext(ctx).Raw("SELECT current_database() AS database,current_schema() AS schema").Scan(&identity).Error; err != nil { return f, err }
    f.Database, f.Schema = identity.Database, identity.Schema
    var platforms []struct { ID int64; MenuVersion int64 }
    if err := db.WithContext(ctx).Raw(`SELECT id,menu_version FROM permission_auth_platform WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL`).Scan(&platforms).Error; err != nil { return f, err }
    if len(platforms) != 1 { return f, fmt.Errorf("exactly one active Admin platform is required") }
    f.AdminID, f.MenuVersion = platforms[0].ID, platforms[0].MenuVersion
    var roles []struct { ID int64 }
    if err := db.WithContext(ctx).Raw(`SELECT id FROM permission_role WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL`).Scan(&roles).Error; err != nil { return f, err }
    if len(roles) != 1 { return f, fmt.Errorf("exactly one active registered_user role is required") }
    f.RegisteredRoleID = roles[0].ID

    targetCodes := []string{"payment:redeemCode:view", "payment:redeemCode:list", "payment:redeemCode:detail", "payment:redeemCode:generate", "payment:redeemCode:void", "payment:redeemCode:export", "payment:myWallet:view", "payment:myWallet:redeem"}
    if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id=? AND deleted_at IS NULL AND code IN ?`, f.AdminID, targetCodes).Scan(&f.TargetMenuCount).Error; err != nil { return f, err }
    if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.role_id=? AND rm.deleted_at IS NULL AND m.platform_id=? AND m.code='payment:myWallet:redeem' AND m.deleted_at IS NULL`, f.RegisteredRoleID, f.AdminID).Scan(&f.RedeemGrantCount).Error; err != nil { return f, err }
    if err := db.WithContext(ctx).Raw(`SELECT md5(coalesce(string_agg(concat_ws('|',id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,is_enabled,is_hidden),';' ORDER BY id),'')) FROM permission_menu WHERE platform_id=? AND deleted_at IS NULL AND code IN ?`, f.AdminID, targetCodes).Scan(&f.TargetMenuDigest).Error; err != nil { return f, err }
    if err := db.WithContext(ctx).Raw(`SELECT md5(coalesce(string_agg(concat_ws('|',rm.id,rm.role_id,rm.menu_id),';' ORDER BY rm.id),'')) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.deleted_at IS NULL AND m.code NOT IN ?`, targetCodes).Scan(&f.UnrelatedGrantDigest).Error; err != nil { return f, err }
    if err := db.WithContext(ctx).Raw(`SELECT ur.user_id,av.version FROM permission_user_role ur JOIN user_account u ON u.id=ur.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL LEFT JOIN permission_access_version av ON av.user_id=ur.user_id WHERE ur.role_id=? AND ur.deleted_at IS NULL ORDER BY ur.user_id`, f.RegisteredRoleID).Scan(&f.RegisteredUsers).Error; err != nil { return f, err }
    for _, user := range f.RegisteredUsers { if user.UserID < 1 || user.Version < 1 { return f, fmt.Errorf("registered_user %d lacks a valid access version", user.UserID) } }
    if err := db.WithContext(ctx).Raw(`SELECT user_id,version FROM permission_access_version ORDER BY user_id`).Scan(&f.AccessVersions).Error; err != nil { return f, err }
    return f, nil
}

func applyMigration(ctx context.Context, db *gorm.DB, sqlBytes []byte) error {
    return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Exec("SET LOCAL search_path TO public,pg_catalog").Error; err != nil { return err }
        return tx.Exec(string(sqlBytes)).Error
    })
}

func verifyTransition(before, after facts) error {
    if after.AdminID != before.AdminID || after.RegisteredRoleID != before.RegisteredRoleID { return errors.New("platform or role identity changed") }
    if after.TargetMenuCount != 8 || after.RedeemGrantCount != 1 { return errors.New("target menu/grant shape is incomplete") }
    if before.UnrelatedGrantDigest != after.UnrelatedGrantDigest { return errors.New("unrelated role grants changed") }
    menuDelta := after.MenuVersion - before.MenuVersion
    if menuDelta < 0 || menuDelta > 1 { return fmt.Errorf("unexpected Admin menu version change %d -> %d", before.MenuVersion, after.MenuVersion) }
    menuChanged := before.TargetMenuCount != after.TargetMenuCount || before.TargetMenuDigest != after.TargetMenuDigest
    if menuChanged && menuDelta != 1 { return errors.New("menu facts changed without one menu version increment") }
    if !menuChanged && menuDelta != 0 { return errors.New("menu version incremented without menu fact change") }
    grantChanged := before.RedeemGrantCount != after.RedeemGrantCount
    if err := verifyAccessVersions(before, after, grantChanged); err != nil { return err }
    return nil
}

func verifyAccessVersions(before, after facts, grantChanged bool) error {
    old := make(map[int64]int64, len(before.AccessVersions)); for _, item := range before.AccessVersions { old[item.UserID] = item.Version }
    next := make(map[int64]int64, len(after.AccessVersions)); for _, item := range after.AccessVersions { next[item.UserID] = item.Version }
    if len(old) != len(next) { return errors.New("access version row count changed unexpectedly") }
    changedUsers := make(map[int64]struct{}, len(before.RegisteredUsers)); for _, item := range before.RegisteredUsers { changedUsers[item.UserID] = struct{}{} }
    for userID, version := range old {
        expected := version
        if grantChanged { if _, ok := changedUsers[userID]; ok { expected++ } }
        if next[userID] != expected { return fmt.Errorf("access version for user %d changed from %d to %d, want %d", userID, version, next[userID], expected) }
    }
    return nil
}

func syncRedis(ctx context.Context, rc *projectredis.Client, f facts) error {
    if err := permissionstate.NewMenuStore(rc).RebuildReadyState(ctx, f.AdminID, f.MenuVersion); err != nil { return err }
    store := permissionstate.NewStore(rc)
    const batchSize = 500
    for start := 0; start < len(f.RegisteredUsers); start += batchSize {
        end := start + batchSize; if end > len(f.RegisteredUsers) { end = len(f.RegisteredUsers) }
        batch := make([]permissionstate.Version, 0, end-start)
        for _, item := range f.RegisteredUsers[start:end] { batch = append(batch, permissionstate.Version{UserID: item.UserID, Version: item.Version}) }
        if err := store.RebuildReadyState(ctx, batch); err != nil { return err }
    }
    return nil
}

func verifyRedis(ctx context.Context, rc *projectredis.Client, f facts) error {
    menu, found, err := permissionstate.NewMenuStore(rc).Read(ctx, f.AdminID)
    if err != nil { return err }; if !found || menu.State != permissionstate.StateReady || menu.Version != f.MenuVersion { return errors.New("Admin menu Redis state is not ready at the database version") }
    store := permissionstate.NewStore(rc)
    for _, expected := range f.RegisteredUsers {
        state, found, err := store.Read(ctx, expected.UserID)
        if err != nil { return err }; if !found || state.State != permissionstate.StateReady || state.Version != expected.Version { return fmt.Errorf("access Redis state for user %d is not ready at the database version", expected.UserID) }
    }
    return nil
}

func sameFacts(left, right facts) bool {
    left.RegisteredUsers = append([]version(nil), left.RegisteredUsers...); right.RegisteredUsers = append([]version(nil), right.RegisteredUsers...)
    left.AccessVersions = append([]version(nil), left.AccessVersions...); right.AccessVersions = append([]version(nil), right.AccessVersions...)
    sort.Slice(left.RegisteredUsers, func(i,j int) bool { return left.RegisteredUsers[i].UserID < left.RegisteredUsers[j].UserID }); sort.Slice(right.RegisteredUsers, func(i,j int) bool { return right.RegisteredUsers[i].UserID < right.RegisteredUsers[j].UserID })
    sort.Slice(left.AccessVersions, func(i,j int) bool { return left.AccessVersions[i].UserID < left.AccessVersions[j].UserID }); sort.Slice(right.AccessVersions, func(i,j int) bool { return right.AccessVersions[i].UserID < right.AccessVersions[j].UserID })
    return reflect.DeepEqual(left, right)
}

func writeAudit(path string, report audit) error {
    bytes, err := json.MarshalIndent(report, "", "  "); if err != nil { return err }
    if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { return err }
    temporary := path + ".next"
    if err := os.WriteFile(temporary, bytes, 0600); err != nil { return err }
    return os.Rename(temporary, path)
}
'@
    $helper | Set-Content -LiteralPath $helperPath -Encoding UTF8
    $audit.helperSHA256 = (Get-FileHash -LiteralPath $helperPath -Algorithm SHA256).Hash
    $env:PAYMENT_MENU_MAINTENANCE_DSN = $dsn
    $env:PAYMENT_MENU_MAINTENANCE_REDIS_URL = $redisUrl
    $env:PGAPPNAME = 'admin-payment-redeem-menu-maintenance'
    $env:PGOPTIONS = '-c search_path=public,pg_catalog'
    Push-Location $serverRoot
    try {
        $executablePath = Join-Path $AuditDirectory 'payment-redeem-code-menu-maintenance.exe'
        $relativeHelper = './tmp/' + (Split-Path $helperDirectory -Leaf)
        & go build -o $executablePath $relativeHelper
        if ($LASTEXITCODE -ne 0) { throw '维护程序编译失败；未执行 migration。' }
        $arguments = @('-sql', $sqlPath, '-audit', $AuditDirectory)
        if (-not $CheckOnly) { $arguments += @('-backup', $backupPath) } else { $arguments += '-check' }
        & $executablePath @arguments
        if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL/Redis 维护未完成；请依据 result.json 保持服务停止后重试。' }
    }
    finally {
        Pop-Location
    }
    $resultPath = Join-Path $AuditDirectory 'result.json'
    if (Test-Path -LiteralPath $resultPath) { $audit.result = Get-Content -LiteralPath $resultPath -Raw | ConvertFrom-Json }
    $audit.verified = $true
    Write-Output "兑换码菜单迁移已验证；审计目录：$AuditDirectory"
}
catch {
    $audit.failed = $true
    $audit.error = $_.Exception.Message
    Write-Error $_.Exception.Message -ErrorAction Continue
    exit 1
}
finally {
    $env:PAYMENT_MENU_MAINTENANCE_DSN = $oldDsn
    $env:PAYMENT_MENU_MAINTENANCE_REDIS_URL = $oldRedisUrl
    $env:PGAPPNAME = $oldAppName
    $env:PGOPTIONS = $oldOptions
    Write-Audit
    if ($null -ne $helperDirectory) {
        $resolvedHelper = [IO.Path]::GetFullPath($helperDirectory)
        $expectedParent = [IO.Path]::GetFullPath((Join-Path $serverRoot 'tmp')) + [IO.Path]::DirectorySeparatorChar
        if ($resolvedHelper.StartsWith($expectedParent, [StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $resolvedHelper -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

