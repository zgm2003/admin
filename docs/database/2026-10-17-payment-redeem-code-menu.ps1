<#
.SYNOPSIS
验证、执行或恢复兑换码菜单维护；不会启动或停止业务服务。
.DESCRIPTION
默认目标为 public。写入和恢复必须由维护者停止所有 API/Worker 后执行。
审计目录必须为空；恢复读取原 result.json、用户 manifest 和备份，不重跑 SQL。
CheckOnly 只读验证菜单、授权和 Redis ready-state；它不代替支付到账验收。
.EXAMPLE
.\docs\database\2026-10-17-payment-redeem-code-menu.ps1 -CheckOnly
.EXAMPLE
.\docs\database\2026-10-17-payment-redeem-code-menu.ps1 -ServicesStopped
.EXAMPLE
.\docs\database\2026-10-17-payment-redeem-code-menu.ps1 -ServicesStopped -RecoverFrom 'TARGET/result.json'
#>
[CmdletBinding()]
param(
    [switch]$CheckOnly,
    [switch]$ServicesStopped,
    [string]$AuditDirectory,
    [string]$Schema = 'public',
    [string]$RecoverFrom
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
    postgresCommitState = 'notStarted'
    postgresCommitted = $false
    redisSynchronized = $false
    verified = $false
    recovered = $false
    failureStage = 'input'
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
    $nextPath = $auditPath + '.next'
    $audit | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $nextPath -Encoding UTF8
    Move-Item -LiteralPath $nextPath -Destination $auditPath -Force
}

function Sync-HelperAudit {
    $resultPath = Join-Path $AuditDirectory 'result.json'
    if (-not (Test-Path -LiteralPath $resultPath -PathType Leaf)) { return }
    try {
        $result = Get-Content -LiteralPath $resultPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($result.migration -ne $audit.migration) { throw 'invalid migration' }
        foreach ($field in @('postgresCommitted', 'redisSynchronized', 'verified', 'recovered')) {
            if ($result.$field -isnot [bool]) { throw 'invalid maintenance fact' }
        }
        if ($result.postgresCommitState -notin @('notStarted', 'rolledBack', 'unknown', 'committed')) {
            throw 'invalid commit state'
        }
    }
    catch {
        throw '维护程序结果审计不可读取；请保持服务停止并保留整个审计目录。'
    }
    $audit.result = $result
    $audit.postgresCommitState = $result.postgresCommitState
    $audit.postgresCommitted = $result.postgresCommitted
    $audit.redisSynchronized = $result.redisSynchronized
    $audit.verified = $result.verified
    $audit.recovered = $result.recovered
    $audit.failureStage = $result.failureStage
    if (-not [string]::IsNullOrWhiteSpace($result.backupSHA256)) {
        $audit.backup = Join-Path $AuditDirectory 'schema-before.dump'
        $audit.backupSHA256 = $result.backupSHA256
    }
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
    $audit.schema = $Schema
    $audit.checkOnly = [bool]$CheckOnly
    Write-Audit

    if ($Schema -cnotmatch '^[a-z][a-z0-9_]{0,62}$') { throw 'Schema 必须为有效的小写 PostgreSQL schema 名称。' }
    if ($CheckOnly -and -not [string]::IsNullOrWhiteSpace($RecoverFrom)) { throw '-CheckOnly 与 -RecoverFrom 不能同时使用。' }
    if (-not [string]::IsNullOrWhiteSpace($RecoverFrom)) {
        $RecoverFrom = [IO.Path]::GetFullPath($RecoverFrom)
        if (-not (Test-Path -LiteralPath $RecoverFrom -PathType Leaf)) { throw '找不到待恢复的 result.json；未执行维护。' }
        $audit.recoverFrom = $RecoverFrom
    }

    $backupPath = Join-Path $AuditDirectory 'schema-before.dump'
    if (-not $CheckOnly) {
        if (-not $ServicesStopped) { throw '请先停止 API/Worker，再传入 -ServicesStopped。' }
        $env:PGAPPNAME = 'admin-payment-redeem-menu-maintenance'
        $audit.failureStage = 'offline'
        if ($Schema -ceq 'public') {
            $sessionCount = @(& psql -X -w -q -A -t --dbname $dsn -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND pid<>pg_backend_pid()" 2>&1)
            if ($LASTEXITCODE -ne 0) { throw '无法检查 PostgreSQL 连接；未执行 migration。' }
            if ([int64]($sessionCount -join '').Trim() -ne 0) { throw '业务库仍有其他客户端连接；请保持服务停止后重试。' }
        }
    }
    if (-not $CheckOnly -and [string]::IsNullOrWhiteSpace($RecoverFrom)) {
        $audit.failureStage = 'backup'
        & pg_dump --format=custom --no-owner --no-privileges --dbname $dsn --schema $Schema --file $backupPath 2>&1 | Out-Null
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
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	permissionstate "admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const migrationName = "payment-redeem-code-menu"
const batchSize = 500

type maintenanceError struct{ message string }

func (e *maintenanceError) Error() string { return e.message }
func problem(message string) error        { return &maintenanceError{message: message} }

type version struct {
	UserID  int64 `json:"userId"`
	Version int64 `json:"version"`
}

type userSnapshot struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Count  int64  `json:"count"`
}

type facts struct {
	Database              string `json:"database"`
	Schema                string `json:"schema"`
	AdminID               int64  `json:"adminId"`
	MenuVersion           int64  `json:"menuVersion"`
	RegisteredRoleID      int64  `json:"registeredRoleId"`
	RegisteredUserCount   int64  `json:"registeredUserCount"`
	TargetMenuCount       int64  `json:"targetMenuCount"`
	RedeemGrantCount      int64  `json:"redeemGrantCount"`
	WalletPageGrantCount  int64  `json:"walletPageGrantCount"`
	TargetMenuDigest      string `json:"targetMenuDigest"`
	UnrelatedMenuDigest   string `json:"unrelatedMenuDigest"`
	UnrelatedGrantDigest  string `json:"unrelatedGrantDigest"`
	UnrelatedAccessDigest string `json:"unrelatedAccessDigest"`
	AccessVersionCount    int64  `json:"accessVersionCount"`
}

type audit struct {
	Migration           string        `json:"migration"`
	DatabaseIdentity    string        `json:"databaseIdentity"`
	RedisIdentity       string        `json:"redisIdentity"`
	SQLSHA256           string        `json:"sqlSHA256"`
	BackupSHA256        string        `json:"backupSHA256,omitempty"`
	Before              *facts        `json:"before,omitempty"`
	After               *facts        `json:"after,omitempty"`
	BeforeUsers         *userSnapshot `json:"beforeUsers,omitempty"`
	AfterUsers          *userSnapshot `json:"afterUsers,omitempty"`
	PostgresCommitState string        `json:"postgresCommitState"`
	PostgresCommitted   bool          `json:"postgresCommitted"`
	RedisSynchronized   bool          `json:"redisSynchronized"`
	PublishedUserCount  int64         `json:"publishedUserCount"`
	Verified            bool          `json:"verified"`
	CheckOnly           bool          `json:"checkOnly"`
	Replayed            bool          `json:"replayed"`
	Recovered           bool          `json:"recovered"`
	FailureStage        string        `json:"failureStage,omitempty"`
	Error               string        `json:"error,omitempty"`
	StartedAt           string        `json:"startedAt"`
	FinishedAt          string        `json:"finishedAt"`
}

func main() { os.Exit(runCLI()) }

func runCLI() int {
	sqlPath := flag.String("sql", "", "fixed migration SQL")
	auditDir := flag.String("audit", "", "new audit directory")
	backup := flag.String("backup", "", "verified backup archive")
	schema := flag.String("schema", "", "explicit target schema")
	check := flag.Bool("check", false, "read-only deployment verification")
	recoverFrom := flag.String("recover", "", "previous result.json")
	flag.Parse()
	report := audit{Migration: migrationName, PostgresCommitState: "notStarted", CheckOnly: *check, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), FailureStage: "input"}
	resultPath := filepath.Join(*auditDir, "result.json")
	err := func() error {
		if *sqlPath == "" || *auditDir == "" || (*check && *recoverFrom != "") {
			return problem("invalid maintenance arguments")
		}
		if *schema != "" && !regexp.MustCompile("^[a-z][a-z0-9_]{0,62}$").MatchString(*schema) {
			return problem("invalid target schema")
		}
		if err := os.MkdirAll(*auditDir, 0700); err != nil {
			return problem("audit directory is unavailable")
		}
		sqlBytes, err := os.ReadFile(*sqlPath)
		if err != nil || len(sqlBytes) == 0 || len(sqlBytes) > 4<<20 {
			return problem("migration SQL is unavailable or invalid")
		}
		hash := sha256.Sum256(sqlBytes)
		report.SQLSHA256 = strings.ToUpper(hex.EncodeToString(hash[:]))
		if !*check && *recoverFrom == "" {
			if *backup == "" {
				return problem("verified backup archive is required")
			}
			if info, statErr := os.Stat(*backup); statErr != nil || info.Size() == 0 {
				return problem("verified backup archive is empty or unavailable")
			}
			report.BackupSHA256, err = copyFile(*backup, filepath.Join(*auditDir, "schema-before.dump"))
			if err != nil {
				return problem("verified backup archive cannot be read or archived")
			}
		}
		if err = writeAudit(resultPath, report); err != nil {
			return problem("preflight audit could not be persisted")
		}
		dsn := firstEnv("PAYMENT_MENU_MAINTENANCE_DSN", "POSTGRES_DSN")
		redisURL := firstEnv("PAYMENT_MENU_MAINTENANCE_REDIS_URL", "REDIS_URL")
		if dsn == "" || redisURL == "" {
			return problem("maintenance connection settings are missing")
		}
		report.FailureStage = "postgres"
		config, err := pgx.ParseConfig(dsn)
		if err != nil {
			return problem("PostgreSQL settings are invalid")
		}
		if *schema != "" {
			config.RuntimeParams["search_path"] = *schema + ",pg_catalog"
		}
		config.RuntimeParams["application_name"] = "admin-payment-redeem-menu-maintenance"
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		sqlDB := stdlib.OpenDB(*config)
		defer sqlDB.Close()
		if err = sqlDB.PingContext(ctx); err != nil {
			return problem("PostgreSQL connection failed")
		}
		db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			return problem("PostgreSQL initialization failed")
		}
		report.FailureStage = "redis"
		report.RedisIdentity, err = redisIdentity(redisURL)
		if err != nil {
			return err
		}
		rc, err := projectredis.Open(ctx, redisURL)
		if err != nil {
			return problem("Redis connection failed")
		}
		defer rc.Close()
		report.FailureStage = "postgres"
		return db.WithContext(ctx).Connection(func(connection *gorm.DB) (resultErr error) {
			report.DatabaseIdentity, err = databaseIdentity(ctx, connection)
			if err != nil {
				return err
			}
			var locked bool
			if err = connection.Raw("SELECT pg_try_advisory_lock(20261017,403)").Scan(&locked).Error; err != nil || !locked {
				return problem("another payment menu maintenance may be running")
			}
			defer func() {
				var released bool
				if e := connection.Raw("SELECT pg_advisory_unlock(20261017,403)").Scan(&released).Error; e != nil || !released {
					resultErr = problem("maintenance lock release was not confirmed")
				}
			}()
			if *check {
				return checkDeployment(ctx, connection, rc, *auditDir, &report)
			}
			if err = ensureOffline(ctx, connection); err != nil {
				return err
			}
			if *recoverFrom != "" {
				return recoverRedis(ctx, connection, rc, *auditDir, *recoverFrom, &report)
			}
			return migrate(ctx, connection, rc, sqlBytes, *auditDir, &report)
		})
	}()
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err != nil {
		var public *maintenanceError
		if errors.As(err, &public) {
			report.Error = public.message
		} else {
			report.Error = "maintenance dependency failed; retain the audit for the recorded failure stage"
		}
		report.Verified = false
	} else {
		report.FailureStage = ""
	}
	if *auditDir != "" {
		if saveErr := writeAudit(resultPath, report); saveErr != nil {
			fmt.Fprintln(os.Stderr, "maintenance audit write failed; keep services stopped and retain the audit directory")
			return 1
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "maintenance failed at %s: %s; PostgreSQL commit state=%s, Redis synchronized=%t; keep services stopped\n", report.FailureStage, report.Error, report.PostgresCommitState, report.RedisSynchronized)
		return 1
	}
	return 0
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func databaseIdentity(ctx context.Context, db *gorm.DB) (string, error) {
	var identity string
	if err := db.WithContext(ctx).Raw("SELECT current_database()||'/'||current_schema()||'@'||coalesce(inet_server_addr()::text,'local')||':'||coalesce(inet_server_port()::text,'local')").Scan(&identity).Error; err != nil || identity == "" {
		return "", problem("database identity could not be confirmed")
	}
	return identity, nil
}

func redisIdentity(raw string) (string, error) {
	options, err := goredis.ParseURL(raw)
	if err != nil {
		return "", problem("Redis settings are invalid")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", problem("Redis identity is invalid")
	}
	return fmt.Sprintf("%s://%s/%d", parsed.Scheme, options.Addr, options.DB), nil
}

func ensureOffline(ctx context.Context, db *gorm.DB) error {
	var identity struct {
		Schema       string
		OtherClients int64
	}
	if err := db.WithContext(ctx).Raw("SELECT current_schema() AS schema,(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend' AND pid<>pg_backend_pid()) AS other_clients").Scan(&identity).Error; err != nil {
		return problem("offline maintenance check failed")
	}
	// Isolated schemas permit test fixture connections. Business public maintenance
	// additionally requires no other PostgreSQL clients; the wrapper requires an
	// explicit ServicesStopped attestation for all API/Worker instances.
	if identity.Schema == "public" && identity.OtherClients != 0 {
		return problem("other database clients are connected; stop all API/Worker instances before maintenance")
	}
	return nil
}

func readFacts(ctx context.Context, db *gorm.DB) (facts, error) {
	var f facts
	if err := db.WithContext(ctx).Raw("SELECT current_database() AS database,current_schema() AS schema").Scan(&f).Error; err != nil || f.Schema == "" {
		return f, problem("target schema could not be confirmed")
	}
	var platforms []struct {
		ID          int64
		MenuVersion int64
	}
	if err := db.WithContext(ctx).Raw("SELECT id,menu_version FROM permission_auth_platform WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL").Scan(&platforms).Error; err != nil || len(platforms) != 1 || platforms[0].ID < 1 || platforms[0].MenuVersion < 1 {
		return f, problem("exactly one valid active Admin platform is required")
	}
	f.AdminID, f.MenuVersion = platforms[0].ID, platforms[0].MenuVersion
	var roles []struct{ ID int64 }
	if err := db.WithContext(ctx).Raw("SELECT id FROM permission_role WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL").Scan(&roles).Error; err != nil || len(roles) != 1 || roles[0].ID < 1 {
		return f, problem("exactly one active registered_user role is required")
	}
	f.RegisteredRoleID = roles[0].ID
	targetCodes := []string{"payment:redeemCode:view", "payment:redeemCode:list", "payment:redeemCode:detail", "payment:redeemCode:generate", "payment:redeemCode:void", "payment:redeemCode:export", "payment:myWallet:view", "payment:myWallet:redeem"}
	queries := []struct {
		sql  string
		args []any
		dest any
	}{
		{"SELECT count(*) FROM permission_menu WHERE platform_id=? AND deleted_at IS NULL AND code IN ?", []any{f.AdminID, targetCodes}, &f.TargetMenuCount},
		{"SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.role_id=? AND rm.deleted_at IS NULL AND m.platform_id=? AND m.code='payment:myWallet:redeem' AND m.deleted_at IS NULL", []any{f.RegisteredRoleID, f.AdminID}, &f.RedeemGrantCount},
		{"SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.role_id=? AND rm.deleted_at IS NULL AND m.platform_id=? AND m.code='payment:myWallet:view' AND m.deleted_at IS NULL", []any{f.RegisteredRoleID, f.AdminID}, &f.WalletPageGrantCount},
		{"SELECT md5(coalesce(jsonb_agg(to_jsonb(m) ORDER BY id)::text,'')) FROM permission_menu m WHERE platform_id=? AND deleted_at IS NULL AND code IN ?", []any{f.AdminID, targetCodes}, &f.TargetMenuDigest},
		{"SELECT count(*)::text||':'||coalesce(sum(('x'||substr(md5(to_jsonb(m)::text),1,15))::bit(60)::bigint)::text,'0') FROM permission_menu m WHERE NOT (platform_id=? AND deleted_at IS NULL AND code IN ?)", []any{f.AdminID, targetCodes}, &f.UnrelatedMenuDigest},
		{"SELECT count(*)::text||':'||coalesce(sum(('x'||substr(md5(to_jsonb(rm)::text),1,15))::bit(60)::bigint)::text,'0') FROM permission_role_menu rm WHERE NOT (role_id=? AND menu_id IN (SELECT id FROM permission_menu WHERE platform_id=? AND code='payment:myWallet:redeem' AND deleted_at IS NULL))", []any{f.RegisteredRoleID, f.AdminID}, &f.UnrelatedGrantDigest},
		{"SELECT count(*) FROM user_account u WHERE u.is_enabled=1 AND u.deleted_at IS NULL AND EXISTS(SELECT 1 FROM permission_user_role ur WHERE ur.user_id=u.id AND ur.role_id=? AND ur.deleted_at IS NULL)", []any{f.RegisteredRoleID}, &f.RegisteredUserCount},
		{"SELECT count(*) FROM permission_access_version", nil, &f.AccessVersionCount},
		{"SELECT count(*)::text||':'||coalesce(sum(('x'||substr(md5(to_jsonb(av)::text),1,15))::bit(60)::bigint)::text,'0') FROM permission_access_version av WHERE NOT EXISTS(SELECT 1 FROM user_account u WHERE u.id=av.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL AND EXISTS(SELECT 1 FROM permission_user_role ur WHERE ur.user_id=u.id AND ur.role_id=? AND ur.deleted_at IS NULL))", []any{f.RegisteredRoleID}, &f.UnrelatedAccessDigest},
	}
	for _, query := range queries {
		if err := db.WithContext(ctx).Raw(query.sql, query.args...).Scan(query.dest).Error; err != nil {
			return f, problem("permission fact inspection failed")
		}
	}
	if f.WalletPageGrantCount != 1 {
		return f, problem("P2 registered_user personal wallet page grant is required")
	}
	return f, nil
}

func captureUsers(ctx context.Context, db *gorm.DB, roleID int64, dir, name string) (*userSnapshot, error) {
	path := filepath.Join(dir, name)
	temporary := path + ".next"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, problem("user manifest cannot be created")
	}
	defer file.Close()
	hash := sha256.New()
	encoder := json.NewEncoder(io.MultiWriter(file, hash))
	result := &userSnapshot{File: name}
	var lastID int64
	for {
		var batch []version
		err = db.WithContext(ctx).Raw("SELECT u.id AS user_id,av.version FROM user_account u LEFT JOIN permission_access_version av ON av.user_id=u.id WHERE u.id>? AND u.is_enabled=1 AND u.deleted_at IS NULL AND EXISTS(SELECT 1 FROM permission_user_role ur WHERE ur.user_id=u.id AND ur.role_id=? AND ur.deleted_at IS NULL) ORDER BY u.id LIMIT ?", lastID, roleID, batchSize).Scan(&batch).Error
		if err != nil {
			return nil, problem("affected user version read failed")
		}
		if len(batch) == 0 {
			break
		}
		for _, item := range batch {
			if item.UserID <= lastID || item.Version < 1 {
				return nil, problem("affected user lacks a valid access version")
			}
			if err = encoder.Encode(item); err != nil {
				return nil, problem("user manifest write failed")
			}
			lastID = item.UserID
			result.Count++
		}
	}
	if err = file.Sync(); err != nil {
		return nil, problem("user manifest sync failed")
	}
	if err = file.Close(); err != nil {
		return nil, problem("user manifest close failed")
	}
	if err = os.Rename(temporary, path); err != nil {
		return nil, problem("user manifest publication failed")
	}
	result.SHA256 = strings.ToUpper(hex.EncodeToString(hash.Sum(nil)))
	return result, nil
}

func snapshotPath(dir string, snapshot *userSnapshot) (string, error) {
	if snapshot == nil || (snapshot.File != "before-users.jsonl" && snapshot.File != "after-users.jsonl") || snapshot.Count < 0 || !regexp.MustCompile("^[A-F0-9]{64}$").MatchString(snapshot.SHA256) {
		return "", problem("user manifest reference is invalid")
	}
	return filepath.Join(dir, snapshot.File), nil
}

func decodeVersion(raw []byte) (version, error) {
	var value version
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil || value.UserID < 1 || value.Version < 1 {
		return value, problem("user manifest row is invalid")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return value, problem("user manifest row has trailing data")
	}
	return value, nil
}

func walkUsers(dir string, snapshot *userSnapshot, visit func([]version) error) error {
	path, err := snapshotPath(dir, snapshot)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return problem("user manifest cannot be read")
	}
	defer file.Close()
	hash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(file, hash))
	scanner.Buffer(make([]byte, 256), 512)
	batch := make([]version, 0, batchSize)
	var count, lastID int64
	for scanner.Scan() {
		item, err := decodeVersion(scanner.Bytes())
		if err != nil {
			return err
		}
		if item.UserID <= lastID {
			return problem("user manifest IDs are not strictly ordered")
		}
		lastID = item.UserID
		count++
		batch = append(batch, item)
		if len(batch) == batchSize {
			if visit != nil {
				if err = visit(batch); err != nil {
					return err
				}
			}
			batch = batch[:0]
		}
	}
	if err = scanner.Err(); err != nil {
		return problem("user manifest scan failed")
	}
	if len(batch) > 0 && visit != nil {
		if err = visit(batch); err != nil {
			return err
		}
	}
	if count != snapshot.Count || strings.ToUpper(hex.EncodeToString(hash.Sum(nil))) != snapshot.SHA256 {
		return problem("user manifest hash or row count mismatch")
	}
	return nil
}

func verifyUsers(dir string, before, after *userSnapshot, increment bool) error {
	if err := walkUsers(dir, before, nil); err != nil {
		return err
	}
	if err := walkUsers(dir, after, nil); err != nil {
		return err
	}
	if before.Count != after.Count {
		return problem("affected user set changed")
	}
	path, _ := snapshotPath(dir, before)
	file, err := os.Open(path)
	if err != nil {
		return problem("before user manifest cannot be read")
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	return walkUsers(dir, after, func(batch []version) error {
		for _, next := range batch {
			if !scanner.Scan() {
				return problem("before user manifest is incomplete")
			}
			old, err := decodeVersion(scanner.Bytes())
			if err != nil {
				return err
			}
			want := old.Version
			if increment {
				if want == int64(9223372036854775807) {
					return problem("access version overflow")
				}
				want++
			}
			if next.UserID != old.UserID || next.Version != want {
				return problem("affected access version transition is invalid")
			}
		}
		return nil
	})
}

func verifyTransition(before, after facts) error {
	if before.Database != after.Database || before.Schema != after.Schema || before.AdminID != after.AdminID || before.RegisteredRoleID != after.RegisteredRoleID {
		return problem("platform, role or database identity changed")
	}
	if after.TargetMenuCount != 8 || after.RedeemGrantCount != 1 || after.WalletPageGrantCount != 1 {
		return problem("target menu or personal grants are incomplete")
	}
	if before.UnrelatedMenuDigest != after.UnrelatedMenuDigest || before.UnrelatedGrantDigest != after.UnrelatedGrantDigest || before.UnrelatedAccessDigest != after.UnrelatedAccessDigest || before.AccessVersionCount != after.AccessVersionCount || before.RegisteredUserCount != after.RegisteredUserCount {
		return problem("unrelated menu, grant or user facts changed")
	}
	delta := after.MenuVersion - before.MenuVersion
	changed := before.TargetMenuCount != after.TargetMenuCount || before.TargetMenuDigest != after.TargetMenuDigest
	if delta < 0 || delta > 1 || (changed && delta != 1) || (!changed && delta != 0) {
		return problem("Admin menu version transition is invalid")
	}
	if before.RedeemGrantCount != 0 && before.RedeemGrantCount != 1 {
		return problem("personal redeem grant was duplicated")
	}
	return nil
}

func lockFacts(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Exec("SET LOCAL lock_timeout='5s'; SET LOCAL statement_timeout='10min'; LOCK TABLE permission_auth_platform,permission_menu,permission_role,permission_role_menu,permission_user_role,permission_access_version,user_account IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
		return problem("permission facts could not be locked")
	}
	return nil
}

func applyMigration(ctx context.Context, db *gorm.DB, sqlBytes []byte) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(string(sqlBytes)).Error })
}

func migrate(ctx context.Context, db *gorm.DB, rc *projectredis.Client, sqlBytes []byte, dir string, report *audit) error {
	resultPath := filepath.Join(dir, "result.json")
	report.FailureStage = "postgres-before"
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockFacts(ctx, tx); err != nil {
			return err
		}
		before, err := readFacts(ctx, tx)
		if err != nil {
			return err
		}
		report.Before = &before
		report.BeforeUsers, err = captureUsers(ctx, tx, before.RegisteredRoleID, dir, "before-users.jsonl")
		if err != nil {
			return err
		}
		if report.BeforeUsers.Count != before.RegisteredUserCount {
			return problem("affected user count changed")
		}
		if err = writeAudit(resultPath, *report); err != nil {
			return problem("before audit could not be persisted")
		}
		report.FailureStage = "redis-before"
		if err = checkRedis(ctx, rc, dir, report.Before, report.BeforeUsers, nil, nil, true); err != nil {
			return err
		}
		report.FailureStage = "postgres"
		if err = tx.Exec(string(sqlBytes)).Error; err != nil {
			return problem("menu migration transaction rejected; PostgreSQL changes were rolled back")
		}
		after, err := readFacts(ctx, tx)
		if err != nil {
			return err
		}
		report.After = &after
		report.AfterUsers, err = captureUsers(ctx, tx, after.RegisteredRoleID, dir, "after-users.jsonl")
		if err != nil {
			return err
		}
		if err = verifyTransition(before, after); err != nil {
			return err
		}
		if err = verifyUsers(dir, report.BeforeUsers, report.AfterUsers, before.RedeemGrantCount != after.RedeemGrantCount); err != nil {
			return err
		}
		// Persist both expected outcomes BEFORE commit. A lost commit reply is
		// resolved against these exact facts and streaming version manifests.
		report.PostgresCommitState = "unknown"
		if err = writeAudit(resultPath, *report); err != nil {
			return problem("prepared commit audit could not be persisted")
		}
		return nil
	})
	if err != nil {
		if report.PostgresCommitState != "unknown" {
			report.PostgresCommitState = "rolledBack"
		}
		return problem("PostgreSQL transaction failed; inspect the audited commit state before recovery")
	}
	report.PostgresCommitted = true
	report.PostgresCommitState = "committed"
	if err = writeAudit(resultPath, *report); err != nil {
		return problem("PostgreSQL committed; commit audit could not be persisted")
	}
	report.FailureStage = "replay"
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockFacts(ctx, tx); err != nil {
			return err
		}
		if err := tx.Exec(string(sqlBytes)).Error; err != nil {
			return problem("idempotent SQL replay failed")
		}
		replayed, err := readFacts(ctx, tx)
		if err != nil {
			return err
		}
		users, err := captureUsers(ctx, tx, replayed.RegisteredRoleID, dir, "replay-users.jsonl")
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(replayed, *report.After) || users.Count != report.AfterUsers.Count || users.SHA256 != report.AfterUsers.SHA256 {
			return problem("idempotent replay changed permission facts")
		}
		return nil
	})
	if err != nil {
		return problem("PostgreSQL committed; idempotent replay verification failed")
	}
	report.Replayed = true
	report.FailureStage = "redis-after"
	return publishAndVerify(ctx, db, rc, dir, report)
}

func readRedisVersions(ctx context.Context, rc *projectredis.Client, keys []string, want, prior []int64, allowMissing bool) error {
	pipe := rc.UniversalClient().Pipeline()
	commands := make([]*goredis.StringCmd, len(keys))
	for i, key := range keys {
		commands[i] = pipe.Get(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, goredis.Nil) {
		return problem("Redis state inspection failed")
	}
	for i, command := range commands {
		raw, err := command.Result()
		if errors.Is(err, goredis.Nil) {
			if allowMissing {
				continue
			}
			return problem("Redis ready-state is missing")
		}
		if err != nil {
			return problem("Redis state inspection failed")
		}
		var current struct {
			SchemaVersion int    `json:"schemaVersion"`
			State         string `json:"state"`
			Version       int64  `json:"version"`
		}
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&current); err != nil || current.SchemaVersion != permissionstate.SchemaVersion || current.State != permissionstate.StateReady || current.Version < 1 {
			return problem("Redis state is corrupt or held by a mutation; it was not overwritten")
		}
		if err = decoder.Decode(&struct{}{}); err != io.EOF {
			return problem("Redis state has trailing data")
		}
		if current.Version != want[i] && (prior == nil || current.Version != prior[i]) {
			return problem("Redis version differs from the audited before/after versions; it was not overwritten")
		}
	}
	return nil
}

func checkRedis(ctx context.Context, rc *projectredis.Client, dir string, next *facts, users *userSnapshot, base *facts, baseUsers *userSnapshot, allowMissing bool) error {
	if err := walkUsers(dir, users, nil); err != nil {
		return err
	}
	var oldMenu []int64
	var file *os.File
	var scanner *bufio.Scanner
	if base != nil {
		oldMenu = []int64{base.MenuVersion}
		if err := walkUsers(dir, baseUsers, nil); err != nil {
			return err
		}
		path, _ := snapshotPath(dir, baseUsers)
		var err error
		file, err = os.Open(path)
		if err != nil {
			return problem("before user manifest cannot be read")
		}
		defer file.Close()
		scanner = bufio.NewScanner(file)
	}
	if err := readRedisVersions(ctx, rc, []string{permissionstate.MenuStateKey(next.AdminID)}, []int64{next.MenuVersion}, oldMenu, allowMissing); err != nil {
		return err
	}
	return walkUsers(dir, users, func(batch []version) error {
		keys := make([]string, len(batch))
		desired := make([]int64, len(batch))
		var prior []int64
		if scanner != nil {
			prior = make([]int64, len(batch))
		}
		for i, item := range batch {
			keys[i] = permissionstate.StateKey(item.UserID)
			desired[i] = item.Version
			if scanner != nil {
				if !scanner.Scan() {
					return problem("before user manifest is incomplete")
				}
				old, err := decodeVersion(scanner.Bytes())
				if err != nil || old.UserID != item.UserID {
					return problem("before/after user IDs differ")
				}
				prior[i] = old.Version
			}
		}
		return readRedisVersions(ctx, rc, keys, desired, prior, allowMissing)
	})
}

func publishAndVerify(ctx context.Context, db *gorm.DB, rc *projectredis.Client, dir string, report *audit) error {
	if err := checkRedis(ctx, rc, dir, report.After, report.AfterUsers, report.Before, report.BeforeUsers, true); err != nil {
		return err
	}
	if err := permissionstate.NewMenuStore(rc).RebuildReadyState(ctx, report.After.AdminID, report.After.MenuVersion); err != nil {
		return problem("PostgreSQL committed; Redis menu synchronization incomplete")
	}
	store := permissionstate.NewStore(rc)
	report.PublishedUserCount = 0
	err := walkUsers(dir, report.AfterUsers, func(batch []version) error {
		versions := make([]permissionstate.Version, len(batch))
		for i, item := range batch {
			versions[i] = permissionstate.Version{UserID: item.UserID, Version: item.Version}
		}
		if err := store.RebuildReadyState(ctx, versions); err != nil {
			return problem("PostgreSQL committed; Redis access synchronization incomplete")
		}
		report.PublishedUserCount += int64(len(batch))
		if err := writeAudit(filepath.Join(dir, "result.json"), *report); err != nil {
			return problem("Redis publication checkpoint could not be persisted")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err = checkRedis(ctx, rc, dir, report.After, report.AfterUsers, nil, nil, false); err != nil {
		return err
	}
	report.RedisSynchronized = true
	current, err := readFacts(ctx, db)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, *report.After) {
		return problem("database facts changed during Redis synchronization")
	}
	if err = ensureOffline(ctx, db); err != nil {
		return err
	}
	report.Verified = true
	return nil
}

func verifyMenuShape(ctx context.Context, db *gorm.DB, platformID int64) error {
	type menu struct {
		ID            int64
		Code          string
		MenuType      string
		ParentID      *int64
		Path          *string
		ComponentPath *string
		I18nKey       *string
		Icon          *string
		IsEnabled     int16
		IsHidden      int16
	}
	codes := []string{"payment", "payment:redeemCode:view", "payment:redeemCode:list", "payment:redeemCode:detail", "payment:redeemCode:generate", "payment:redeemCode:void", "payment:redeemCode:export", "payment:myWallet:view", "payment:myWallet:redeem"}
	var rows []menu
	if err := db.WithContext(ctx).Raw("SELECT id,code,menu_type,parent_id,path,component_path,i18n_key,icon,is_enabled,is_hidden FROM permission_menu WHERE platform_id=? AND deleted_at IS NULL AND code IN ? LIMIT 10", platformID, codes).Scan(&rows).Error; err != nil {
		return problem("menu shape inspection failed")
	}
	if len(rows) != len(codes) {
		return problem("payment menu codes are missing or duplicated")
	}
	byCode := make(map[string]menu, len(codes))
	for _, row := range rows {
		if _, found := byCode[row.Code]; found || row.ID < 1 || row.IsEnabled != 1 {
			return problem("payment menu code is duplicated, invalid or disabled")
		}
		byCode[row.Code] = row
	}
	equals := func(value *string, expected string) bool { return value != nil && *value == expected }
	directory := byCode["payment"]
	if directory.MenuType != "directory" || directory.ParentID != nil || directory.Path != nil || directory.ComponentPath != nil || !equals(directory.I18nKey, "navigation.payment") || !equals(directory.Icon, "lucide:circle-dollar-sign") || directory.IsHidden != 0 {
		return problem("payment directory shape is invalid")
	}
	for _, page := range []struct{ code, component, i18n string }{
		{"payment:redeemCode:view", "payment/redeemCode", "navigation.paymentRedeemCode"},
		{"payment:myWallet:view", "payment/myWallet", "navigation.paymentMyWallet"},
	} {
		row := byCode[page.code]
		if row.MenuType != "page" || !equals(row.Path, "/"+page.component) || !equals(row.ComponentPath, page.component) || !equals(row.I18nKey, page.i18n) || !equals(row.Icon, "lucide:circle-dollar-sign") || row.IsHidden != 0 {
			return problem("payment page shape is invalid")
		}
		if page.code == "payment:myWallet:view" {
			if row.ParentID != nil {
				return problem("personal wallet page must be a root page")
			}
		} else if row.ParentID == nil || *row.ParentID != directory.ID {
			return problem("redeem-code page parent is invalid")
		}
	}
	for _, code := range []string{"payment:redeemCode:list", "payment:redeemCode:detail", "payment:redeemCode:generate", "payment:redeemCode:void", "payment:redeemCode:export", "payment:myWallet:redeem"} {
		parent := byCode["payment:redeemCode:view"].ID
		if code == "payment:myWallet:redeem" {
			parent = byCode["payment:myWallet:view"].ID
		}
		row := byCode[code]
		if row.MenuType != "action" || row.ParentID == nil || *row.ParentID != parent || row.Path != nil || row.ComponentPath != nil || row.I18nKey != nil || row.Icon != nil || row.IsHidden != 1 {
			return problem("payment action shape is invalid")
		}
	}
	var conflicts int64
	if err := db.WithContext(ctx).Raw("SELECT count(*) FROM permission_menu WHERE platform_id=? AND deleted_at IS NULL AND (((path='/payment/redeemCode' OR component_path='payment/redeemCode') AND code<>'payment:redeemCode:view') OR ((path='/payment/myWallet' OR component_path='payment/myWallet') AND code<>'payment:myWallet:view'))", platformID).Scan(&conflicts).Error; err != nil || conflicts != 0 {
		return problem("payment route conflicts with another menu")
	}
	return nil
}

func checkDeployment(ctx context.Context, db *gorm.DB, rc *projectredis.Client, dir string, report *audit) error {
	report.FailureStage = "check"
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY").Error; err != nil {
			return problem("read-only verification could not start")
		}
		current, err := readFacts(ctx, tx)
		if err != nil {
			return err
		}
		report.Before = &current
		report.After = &current
		if current.TargetMenuCount != 8 || current.RedeemGrantCount != 1 {
			return problem("redeem-code menus and grants are not fully deployed")
		}
		if err = verifyMenuShape(ctx, tx, current.AdminID); err != nil {
			return err
		}
		report.AfterUsers, err = captureUsers(ctx, tx, current.RegisteredRoleID, dir, "after-users.jsonl")
		if err != nil {
			return err
		}
		if err = checkRedis(ctx, rc, dir, &current, report.AfterUsers, nil, nil, false); err != nil {
			return err
		}
		report.RedisSynchronized = true
		report.Verified = true
		return nil
	})
}

func recoverRedis(ctx context.Context, db *gorm.DB, rc *projectredis.Client, dir, path string, report *audit) error {
	report.FailureStage = "recovery"
	file, err := os.Open(path)
	if err != nil {
		return problem("recovery audit cannot be read")
	}
	defer file.Close()
	var previous audit
	decoder := json.NewDecoder(io.LimitReader(file, 128<<10))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&previous); err != nil {
		return problem("recovery audit is invalid")
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return problem("recovery audit has trailing data")
	}
	if previous.Migration != migrationName || previous.DatabaseIdentity != report.DatabaseIdentity || previous.RedisIdentity != report.RedisIdentity || previous.SQLSHA256 != report.SQLSHA256 {
		return problem("recovery audit belongs to a different database, Redis endpoint or SQL version")
	}
	if previous.Before == nil || previous.After == nil || previous.BeforeUsers == nil || previous.AfterUsers == nil {
		return problem("recovery audit lacks prepared before/after facts; inspect it before starting a new run")
	}
	sourceDir := filepath.Dir(path)
	if err = walkUsers(sourceDir, previous.BeforeUsers, nil); err != nil {
		return err
	}
	if err = walkUsers(sourceDir, previous.AfterUsers, nil); err != nil {
		return err
	}
	backupHash, err := copyFile(filepath.Join(sourceDir, "schema-before.dump"), filepath.Join(dir, "schema-before.dump"))
	if err != nil || backupHash != previous.BackupSHA256 || backupHash == "" {
		return problem("recovery backup hash does not match the original audit")
	}
	report.BackupSHA256 = backupHash
	beforePath, err := snapshotPath(sourceDir, previous.BeforeUsers)
	if err != nil {
		return err
	}
	if _, err = copyFile(beforePath, filepath.Join(dir, "before-users.jsonl")); err != nil {
		return problem("recovery user manifest cannot be archived")
	}
	report.Before = previous.Before
	report.BeforeUsers = &userSnapshot{File: "before-users.jsonl", Count: previous.BeforeUsers.Count, SHA256: previous.BeforeUsers.SHA256}
	current, err := readFacts(ctx, db)
	if err != nil {
		return err
	}
	report.After = &current
	report.AfterUsers, err = captureUsers(ctx, db, current.RegisteredRoleID, dir, "after-users.jsonl")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, *previous.After) || report.AfterUsers.SHA256 != previous.AfterUsers.SHA256 || report.AfterUsers.Count != previous.AfterUsers.Count {
		if reflect.DeepEqual(current, *previous.Before) && report.AfterUsers.SHA256 == previous.BeforeUsers.SHA256 {
			report.PostgresCommitState = "rolledBack"
			return problem("previous transaction did not commit; keep the audit and start a fresh maintenance run")
		}
		return problem("database no longer matches the prepared committed facts; manual review is required")
	}
	if err = verifyTransition(*report.Before, *report.After); err != nil {
		return err
	}
	if err = verifyUsers(dir, report.BeforeUsers, report.AfterUsers, report.Before.RedeemGrantCount != report.After.RedeemGrantCount); err != nil {
		return err
	}
	report.PostgresCommitted = true
	report.PostgresCommitState = "committed"
	report.Replayed = previous.Replayed
	report.Recovered = true
	if err = writeAudit(filepath.Join(dir, "result.json"), *report); err != nil {
		return problem("confirmed recovery audit could not be persisted")
	}
	report.FailureStage = "redis-recovery"
	return publishAndVerify(ctx, db, rc, dir, report)
}

func copyFile(source, destination string) (string, error) {
	if filepath.Clean(source) == filepath.Clean(destination) {
		return hashFile(source)
	}
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	output, err := os.OpenFile(destination+".next", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer output.Close()
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(output, hash), input)
	if err != nil {
		return "", problem("archive copy failed")
	}
	if err = output.Sync(); err != nil {
		return "", err
	}
	if err = output.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(destination+".next", destination); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", problem("archive read failed")
	}
	return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
}

func writeAudit(path string, report audit) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path+".next", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(path+".next", path)
}
'@
    $helper | Set-Content -LiteralPath $helperPath -Encoding UTF8
    $audit.helperSHA256 = (Get-FileHash -LiteralPath $helperPath -Algorithm SHA256).Hash
    $env:PAYMENT_MENU_MAINTENANCE_DSN = $dsn
    $env:PAYMENT_MENU_MAINTENANCE_REDIS_URL = $redisUrl
    $env:PGAPPNAME = 'admin-payment-redeem-menu-maintenance'
    Push-Location $serverRoot
    try {
        $audit.failureStage = 'build'
        $executablePath = Join-Path $AuditDirectory 'payment-redeem-code-menu-maintenance.exe'
        $relativeHelper = './tmp/' + (Split-Path $helperDirectory -Leaf)
        & go build -o $executablePath $relativeHelper
        if ($LASTEXITCODE -ne 0) { throw '维护程序编译失败；未执行 migration。' }
        $arguments = @('-sql', $sqlPath, '-audit', $AuditDirectory, '-schema', $Schema)
        if ($CheckOnly) {
            $arguments += '-check'
        }
        elseif (-not [string]::IsNullOrWhiteSpace($RecoverFrom)) {
            $arguments += @('-recover', $RecoverFrom)
        }
        else {
            $arguments += @('-backup', $backupPath)
        }
        $audit.failureStage = 'maintenance'
        try {
            & $executablePath @arguments
            $helperExitCode = $LASTEXITCODE
        }
        finally {
            Sync-HelperAudit
        }
        if ($helperExitCode -ne 0) { throw "维护未完成：PostgreSQL 提交状态=$($audit.postgresCommitState)，Redis 同步=$($audit.redisSynchronized)；请依据 result.json 保持服务停止并用 -RecoverFrom 恢复。" }
        if (-not $audit.verified) { throw '维护程序未确认验证成功；请保持服务停止并保留审计目录。' }
    }
    finally {
        Pop-Location
    }
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
    try {
        Write-Audit
    }
    finally {
        if ($null -ne $helperDirectory) {
            $resolvedHelper = [IO.Path]::GetFullPath($helperDirectory)
            $expectedParent = [IO.Path]::GetFullPath((Join-Path $serverRoot 'tmp')) + [IO.Path]::DirectorySeparatorChar
            if ($resolvedHelper.StartsWith($expectedParent, [StringComparison]::OrdinalIgnoreCase)) {
                Remove-Item -LiteralPath $resolvedHelper -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

