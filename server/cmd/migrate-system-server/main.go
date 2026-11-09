// migrate-system-server is an explicit, offline maintainer command, never an API startup hook.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	permissionstate "admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type facts struct {
	PlatformID int64 `json:"platformId"`
	Version    int64 `json:"menuVersion"`
	Pages      int   `json:"pages"`
	Actions    int   `json:"actions"`
	Valid      bool  `json:"valid"`
}
type audit struct {
	StartedAt         time.Time `json:"startedAt"`
	SQLSHA256         string    `json:"sqlSha256"`
	Before            facts     `json:"before"`
	After             facts     `json:"after"`
	PostgresCommitted bool      `json:"postgresCommitted"`
	RedisSynchronized bool      `json:"redisSynchronized"`
	Verified          bool      `json:"verified"`
	Error             string    `json:"error,omitempty"`
}
type menuState interface {
	Read(context.Context, int64) (permissionstate.State, bool, error)
	RebuildReadyState(context.Context, int64, int64) error
}

func main() {
	stopped := flag.Bool("services-stopped", false, "confirm all API/Worker instances are stopped")
	directory := flag.String("audit-dir", "", "directory for migration audit")
	flag.Parse()
	if !*stopped || *directory == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "先停止全部 API/Worker，传入 -services-stopped -audit-dir；工具不会操作服务")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := run(ctx, *directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, directory string) error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return errors.New("读取 server/.env 失败")
	}
	dsn, redisURL := os.Getenv("POSTGRES_DSN"), os.Getenv("REDIS_URL")
	if dsn == "" || redisURL == "" {
		return errors.New("缺少 POSTGRES_DSN / REDIS_URL")
	}
	sql, err := os.ReadFile("../docs/database/2026-11-06-system-server.sql")
	if err != nil {
		return errors.New("读取服务器权限 SQL 失败")
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		return errors.New("创建审计目录失败")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return errors.New("PostgreSQL 配置无效")
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	db, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("连接 PostgreSQL 失败")
	}
	defer func() { _ = db.Close(ctx) }()
	r, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		return errors.New("连接 Redis 失败，未执行迁移")
	}
	defer func() { _ = r.Close() }()
	path := filepath.Join(directory, "system-server-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".json")
	if err = execute(ctx, db, permissionstate.NewMenuStore(r), sql, path); err != nil {
		return err
	}
	fmt.Printf("服务器权限迁移与 Redis 菜单版本同步完成；审计：%s\n", path)
	return nil
}

func execute(ctx context.Context, db *pgx.Conn, store menuState, sql []byte, path string) (resultErr error) {
	sum := sha256.Sum256(sql)
	a := audit{StartedAt: time.Now().UTC(), SQLSHA256: hex.EncodeToString(sum[:])}
	defer func() {
		if resultErr != nil {
			a.Error = resultErr.Error()
		}
		if err := saveAudit(path, a); err != nil {
			resultErr = errors.Join(resultErr, errors.New("保存迁移审计失败"))
		}
		if resultErr != nil && a.PostgresCommitted && !a.RedisSynchronized {
			resultErr = errors.Join(errors.New("PostgreSQL 已提交、Redis 同步未完成；禁止启动服务，修复后重复执行同一 runner"), resultErr)
		}
	}()
	var locked bool
	if err := db.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended(current_schema() || ':system-server-2026-11-06',0))`).Scan(&locked); err != nil || !locked {
		return errors.New("另一迁移正在执行，未修改数据")
	}
	defer func() {
		_, _ = db.Exec(ctx, `SELECT pg_advisory_unlock(hashtextextended(current_schema() || ':system-server-2026-11-06',0))`)
	}()
	before, err := readFacts(ctx, db)
	if err != nil {
		return errors.New("读取迁移前事实失败")
	}
	a.Before = before
	state, found, err := store.Read(ctx, before.PlatformID)
	if err != nil || (found && (state.State != permissionstate.StateReady || state.Version > before.Version)) {
		return errors.New("Redis 菜单状态损坏、正在变更或领先 PostgreSQL；未执行迁移")
	}
	if err = saveAudit(path, a); err != nil {
		return errors.New("保存迁移前审计失败，未修改数据")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return errors.New("开始迁移事务失败")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, string(sql)); err != nil {
		return errors.New("服务器菜单存在冲突或 SQL 执行失败，事务将回滚；检查平台、父目录及服务器菜单形状")
	}
	if err = tx.Commit(ctx); err != nil {
		return errors.New("PostgreSQL 提交结果未确认；禁止启动服务，检查数据库后重复执行 runner")
	}
	a.PostgresCommitted = true
	after, err := readFacts(ctx, db)
	if err != nil {
		return errors.New("读取迁移后事实失败")
	}
	a.After = after
	delta := int64(0)
	if before.Pages != 1 {
		delta = 1
	}
	if after.PlatformID != before.PlatformID || after.Version != before.Version+delta || after.Pages != 1 || after.Actions != 0 || !after.Valid {
		return errors.New("服务器菜单形状或版本核验失败；禁止启动服务")
	}
	if err = saveAudit(path, a); err != nil {
		return errors.New("保存提交后审计失败")
	}
	// Fixed platform, monotonic CAS; never delete keys or overwrite an active mutation.
	if err = store.RebuildReadyState(ctx, after.PlatformID, after.Version); err != nil {
		return errors.New("Redis admin 菜单版本同步失败")
	}
	state, found, err = store.Read(ctx, after.PlatformID)
	if err != nil || !found || state.State != permissionstate.StateReady || state.Version != after.Version {
		return errors.New("Redis admin 菜单版本核验失败")
	}
	a.RedisSynchronized = true
	final, err := readFacts(ctx, db)
	if err != nil || final != after {
		return errors.New("最终 PostgreSQL 事实变化；禁止启动服务")
	}
	a.Verified = true
	return nil
}

func readFacts(ctx context.Context, db *pgx.Conn) (facts, error) {
	var f facts
	err := db.QueryRow(ctx, `SELECT p.id,p.menu_version,
      (SELECT count(*) FROM permission_menu WHERE platform_id=p.id AND code='system:server:view'),
      (SELECT count(*) FROM permission_menu WHERE platform_id=p.id AND code='system:server:detail'),
      EXISTS(SELECT 1 FROM permission_menu page
        JOIN permission_menu directory ON directory.id=page.parent_id AND directory.platform_id=page.platform_id
        WHERE page.platform_id=p.id AND page.code='system:server:view' AND page.menu_type='page'
          AND page.path='/system/server' AND page.component_path='system/server' AND page.i18n_key='navigation.systemServer'
          AND page.status=1 AND page.deleted_at IS NULL AND page.is_hidden=0
          AND directory.code='system' AND directory.menu_type='directory' AND directory.status=1 AND directory.deleted_at IS NULL AND directory.parent_id IS NULL)
      FROM permission_auth_platform p WHERE p.code='admin' AND p.status=1 AND p.deleted_at IS NULL`).Scan(&f.PlatformID, &f.Version, &f.Pages, &f.Actions, &f.Valid)
	return f, err
}

func saveAudit(path string, a audit) error {
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(raw)
	return errors.Join(writeErr, f.Sync(), f.Close())
}
