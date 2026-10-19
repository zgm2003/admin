// migrate-platform-routes 是维护者停机执行的一次性迁移工具，不被 API/Worker 调用。
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

	projectredis "admin/server/internal/redis"
	cache "admin/server/internal/shared/configCache"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

var scope = cache.Scope{Namespace: "payment.config", ScopeKey: "global"}

type facts struct {
	Versions            int64  `json:"versions"`
	Legacy              int64  `json:"legacy"`
	PendingLegacyOrders int64  `json:"pendingLegacyOrders"`
	Generation          int64  `json:"generation"`
	Events              int64  `json:"events"`
	ProtectedDigest     string `json:"protectedDigest"`
	ConfigDigest        string `json:"configDigest"`
	ProtectionReady     bool   `json:"protectionReady"`
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

func main() {
	stopped := flag.Bool("services-stopped", false, "confirm all API/Worker instances have stopped")
	directory := flag.String("audit-dir", "", "directory for non-secret migration audit")
	flag.Parse()
	if !*stopped || *directory == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "必须停止全部 API/Worker，并传入 -services-stopped 和 -audit-dir；工具不会操作服务")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := run(ctx, *directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, directory string) (resultErr error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return errors.New("读取 server/.env 失败")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return errors.New("创建审计目录失败，未执行迁移")
	}
	a := audit{StartedAt: time.Now().UTC()}
	auditPath := filepath.Join(directory, "audit.json")
	defer func() {
		if resultErr != nil {
			a.Error = resultErr.Error()
		}
		if err := saveAudit(auditPath, a); err != nil {
			resultErr = errors.Join(resultErr, errors.New("保存审计失败"))
		}
		if resultErr != nil && a.PostgresCommitted && !a.RedisSynchronized {
			resultErr = errors.Join(errors.New("PostgreSQL 已提交、Redis 同步未完成；禁止启动服务，修复后重复执行同一 runner"), resultErr)
		}
	}()
	sql, err := os.ReadFile("../docs/database/2026-10-18-platform-api-routes.sql")
	if err != nil {
		return errors.New("读取平台路由迁移 SQL 失败")
	}
	sum := sha256.Sum256(sql)
	a.SQLSHA256 = hex.EncodeToString(sum[:])
	dsn, redisURL := os.Getenv("POSTGRES_DSN"), os.Getenv("REDIS_URL")
	if dsn == "" || redisURL == "" {
		return errors.New("缺少 POSTGRES_DSN / REDIS_URL")
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
	var locked bool
	if err = db.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended(current_schema() || ':platform-api-routes-2026-10-18',0))`).Scan(&locked); err != nil || !locked {
		return errors.New("另一迁移正在执行，未修改数据")
	}
	r, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		return errors.New("连接 Redis 失败，未执行迁移")
	}
	defer func() { _ = r.Close() }()
	store := cache.NewStore(r)
	a.Before, err = readFacts(ctx, db)
	if err != nil {
		return errors.New("读取迁移前事实失败")
	}
	if err := preflight(a.Before); err != nil {
		return err
	}
	state, found, err := store.Read(ctx, scope)
	if err != nil || (found && (state.State != cache.StateReady || state.Generation > a.Before.Generation)) {
		return errors.New("Redis 配置状态损坏、正在变更或领先 PostgreSQL；未执行迁移")
	}
	if err := saveAudit(auditPath, a); err != nil {
		return errors.New("保存迁移前审计失败，未执行迁移")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return errors.New("开始迁移事务失败")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, string(sql)); err != nil {
		return errors.New("SQL 迁移失败，事务将回滚；检查旧待支付订单、数据库结构和 outbox 冲突")
	}
	if err = tx.Commit(ctx); err != nil {
		return errors.New("PostgreSQL 提交结果未确认；禁止启动服务，检查数据库后重复运行 runner")
	}
	a.PostgresCommitted = true
	a.After, err = readFacts(ctx, db)
	if err != nil {
		return errors.New("读取迁移后事实失败")
	}
	if err = verifyFacts(a.Before, a.After); err != nil {
		return err
	}
	if err := saveAudit(auditPath, a); err != nil {
		return errors.New("保存提交后审计失败")
	}
	// 固定 scope、单调发布；不删除 key，不覆盖 active mutation，不清理其他业务缓存。
	if _, err = store.Reconcile(ctx, scope, a.After.Generation); err != nil {
		return errors.New("Redis payment.config/global 代际同步失败")
	}
	state, found, err = store.Read(ctx, scope)
	if err != nil || !found || state.State != cache.StateReady || state.Generation != a.After.Generation {
		return errors.New("Redis payment.config/global 代际核验失败")
	}
	a.RedisSynchronized = true
	final, err := readFacts(ctx, db)
	if err != nil || final != a.After {
		return errors.New("最终 PostgreSQL 事实发生变化；禁止启动服务")
	}
	a.Verified = true
	fmt.Printf("迁移完成：凭据版本 %d 条，回调迁移 %d 条，generation %d -> %d；审计 %s\n", a.After.Versions, a.Before.Legacy, a.Before.Generation, a.After.Generation, auditPath)
	return nil
}

func saveAudit(path string, a audit) error {
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(raw)
	return errors.Join(writeErr, f.Sync(), f.Close())
}

func preflight(before facts) error {
	if before.PendingLegacyOrders > 0 {
		return fmt.Errorf("仍有 %d 笔待支付订单指向旧回调路径；先通过可信查单结清或关闭，再执行路由迁移", before.PendingLegacyOrders)
	}
	return nil
}

func readFacts(ctx context.Context, db *pgx.Conn) (facts, error) {
	var f facts
	err := db.QueryRow(ctx, `SELECT count(*), count(*) FILTER(WHERE notify_url ~ '/api/v1/payment/callback/alipay/'),
	 (SELECT count(*) FROM payment_order o JOIN payment_config_version v ON v.config_id=o.config_id AND v.version=o.config_version
	  WHERE o.status=1 AND v.notify_url ~ '/api/v1/payment/callback/alipay/'),
	 (SELECT generation FROM system_config_cache_state WHERE namespace='payment.config' AND scope_key='global'),
	 (SELECT count(*) FROM system_config_cache_outbox WHERE namespace='payment.config' AND scope_key='global'),
	 md5(coalesce(jsonb_agg(to_jsonb(v)-'notify_url' ORDER BY config_id,version)::text,'')),
	 (SELECT md5(coalesce(jsonb_agg(to_jsonb(c) ORDER BY id)::text,'')) FROM payment_config c),
	 EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='payment_config_version'::regclass AND tgname='payment_config_version_immutable' AND tgenabled='O' AND NOT tgisinternal)
	 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='payment_config_version'::regclass AND conname='ck_payment_config_version_notify_url' AND convalidated AND position('/api/admin/v1/' in pg_get_constraintdef(oid))>0)
	 FROM payment_config_version v`).Scan(&f.Versions, &f.Legacy, &f.PendingLegacyOrders, &f.Generation, &f.Events, &f.ProtectedDigest, &f.ConfigDigest, &f.ProtectionReady)
	return f, err
}

func verifyFacts(before, after facts) error {
	delta := int64(0)
	if before.Legacy > 0 {
		delta = 1
	}
	if after.Versions != before.Versions || after.Legacy != 0 || after.PendingLegacyOrders != 0 || !after.ProtectionReady || after.ProtectedDigest != before.ProtectedDigest || after.ConfigDigest != before.ConfigDigest || after.Generation != before.Generation+delta || after.Events != before.Events+delta {
		return errors.New("PostgreSQL 回调、不可变事实或 generation/outbox 核验失败；禁止启动服务")
	}
	return nil
}
