package database_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	state "admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestPaymentRedeemCodeMenuRunnerHelper(t *testing.T) {
	root := repoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "docs", "database", "2026-10-17-payment-redeem-code-menu.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	_, helper, found := strings.Cut(normalized, "$helper = @'\n")
	if !found {
		t.Fatal("embedded maintenance helper missing")
	}
	helper, _, found = strings.Cut(helper, "\n'@")
	if !found || !strings.HasPrefix(helper, "package main\n") {
		t.Fatal("embedded maintenance helper is malformed")
	}
	temp, err := os.MkdirTemp(filepath.Join(root, "server", "tmp"), "paymentRedeemMenuRunner-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(temp) })
	if err := os.WriteFile(filepath.Join(temp, "main.go"), []byte(helper), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temp, "main_test.go"), []byte(paymentRedeemCodeMenuRunnerBehaviorTests), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"test", "-v", "-count=1"}
	if testing.Short() {
		args = append(args, "-short")
	}
	command := exec.Command("go", append(args, "./tmp/"+filepath.Base(temp))...)
	command.Dir = filepath.Join(root, "server")
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatalf("maintenance helper regression failed: %v\n%s", err, output)
	}
}

func TestPaymentRedeemCodeMenuRunner(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL/Redis maintenance integration")
	}
	root := repoRoot(t)
	dsn := mustPostgresDSN(t)
	rawRedis := os.Getenv("REDIS_URL")
	if rawRedis == "" {
		t.Skip("REDIS_URL is not configured")
	}
	parsed, err := url.Parse(rawRedis)
	if err != nil {
		t.Fatal(err)
	}
	redisURL := parsed.String()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	rc, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	platformID, userID := time.Now().UnixNano(), time.Now().UnixNano()+1
	unrelatedKey := fmt.Sprintf("test:payment-menu-unrelated:%d", platformID)
	ownedKeys := []string{state.MenuStateKey(platformID), state.StateKey(userID), unrelatedKey}
	for _, key := range ownedKeys {
		_, found, readErr := rc.GetString(ctx, key)
		if readErr != nil || found {
			t.Fatalf("Redis test key must be unoccupied: key=%s found=%v err=%v", key, found, readErr)
		}
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := rc.DeleteMany(cleanup, ownedKeys); err != nil {
			t.Error(err)
		}
	})
	content, err := os.ReadFile(filepath.Join(root, "docs/database/2026-10-17-payment-redeem-code-menu.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	_, helper, ok := strings.Cut(strings.ReplaceAll(string(content), "\r\n", "\n"), "$helper = @'\n")
	if !ok {
		t.Fatal("embedded helper missing")
	}
	helper, _, ok = strings.Cut(helper, "\n'@")
	if !ok {
		t.Fatal("embedded helper terminator missing")
	}
	temp, err := os.MkdirTemp(filepath.Join(root, "server/tmp"), "paymentMenuRunnerCLI-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(temp) })
	if err := os.WriteFile(filepath.Join(temp, "main.go"), []byte(helper), 0600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(temp, "runner.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", executable, "./tmp/"+filepath.Base(temp))
	build.Dir = filepath.Join(root, "server")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	sqlPath := filepath.Join(root, "docs/database/2026-10-17-payment-redeem-code-menu.sql")
	fixture := func(t *testing.T) (*gorm.DB, context.Context, string) {
		t.Helper()
		for _, key := range ownedKeys {
			if err := rc.Delete(ctx, key); err != nil {
				t.Fatal(err)
			}
		}
		if err := rc.SetString(ctx, unrelatedKey, "keep", 0); err != nil {
			t.Fatal(err)
		}
		db, work := openPaymentRedeemMenuSchema(t, "test_payment_menu_runner_cli")
		for _, statement := range []string{
			fmt.Sprintf("UPDATE permission_auth_platform SET id=%d WHERE id=1", platformID),
			fmt.Sprintf("UPDATE permission_menu SET platform_id=%d WHERE platform_id=1", platformID),
			fmt.Sprintf("UPDATE user_account SET id=%d WHERE id=7", userID),
			fmt.Sprintf("UPDATE permission_user_role SET user_id=%d WHERE user_id=7", userID),
			fmt.Sprintf("UPDATE permission_access_version SET user_id=%d WHERE user_id=7", userID),
		} {
			if err := db.WithContext(work).Exec(statement).Error; err != nil {
				t.Fatal(err)
			}
		}
		var schema string
		if err := db.WithContext(work).Raw("SELECT current_schema()").Scan(&schema).Error; err != nil {
			t.Fatal(err)
		}
		if err := rc.SetString(ctx, state.MenuStateKey(platformID), `{"schemaVersion":2,"state":"ready","version":19}`, 0); err != nil {
			t.Fatal(err)
		}
		if err := rc.SetString(ctx, state.StateKey(userID), `{"schemaVersion":2,"state":"ready","version":4}`, 0); err != nil {
			t.Fatal(err)
		}
		return db, work, paymentMenuSchemaDSN(t, dsn, schema)
	}
	run := func(t *testing.T, targetDSN, targetRedis, auditDir, backup string, args ...string) ([]byte, error) {
		t.Helper()
		params := []string{"-sql", sqlPath, "-audit", auditDir}
		if backup != "" {
			params = append(params, "-backup", backup)
		}
		params = append(params, args...)
		command := exec.CommandContext(ctx, executable, params...)
		command.Dir = filepath.Join(root, "server")
		command.Env = append(os.Environ(), "PAYMENT_MENU_MAINTENANCE_DSN="+targetDSN, "PAYMENT_MENU_MAINTENANCE_REDIS_URL="+targetRedis)
		return command.CombinedOutput()
	}
	backup := filepath.Join(temp, "schema-before.dump")
	if err := os.WriteFile(backup, []byte("isolated helper backup fixture; outer runner validates pg_restore"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Run("check-only refuses incomplete menus without writes", func(t *testing.T) {
		db, work, targetDSN := fixture(t)
		before := paymentRedeemMenuDigest(t, db, work)
		if out, err := run(t, targetDSN, redisURL, t.TempDir(), "", "-check"); err == nil {
			t.Fatalf("incomplete deployment reported verified: %s", out)
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("check-only changed database")
		}
		assertPaymentMenuRedisVersion(t, rc, platformID, 19, userID, 4)
	})
	t.Run("missing backup aborts before database changes", func(t *testing.T) {
		db, work, targetDSN := fixture(t)
		before := paymentRedeemMenuDigest(t, db, work)
		if out, err := run(t, targetDSN, redisURL, t.TempDir(), filepath.Join(temp, "missing.dump")); err == nil {
			t.Fatalf("missing backup accepted: %s", out)
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("unreadable backup allowed migration")
		}
		assertPaymentMenuRedisVersion(t, rc, platformID, 19, userID, 4)
	})
	t.Run("audit failure aborts before database changes", func(t *testing.T) {
		db, work, targetDSN := fixture(t)
		before := paymentRedeemMenuDigest(t, db, work)
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "result.json"), 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := run(t, targetDSN, redisURL, dir, backup); err == nil {
			t.Fatal("audit failure accepted")
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("migration committed without a durable pre-commit audit")
		}
		assertPaymentMenuRedisVersion(t, rc, platformID, 19, userID, 4)
	})
	t.Run("connection failure does not disclose credentials", func(t *testing.T) {
		const secret = "PAYMENT_SECRET_SENTINEL"
		dir := t.TempDir()
		out, err := run(t, "host=127.0.0.1 password='"+secret, redisURL, dir, backup)
		if err == nil {
			t.Fatal("malformed connection accepted")
		}
		if bytes.Contains(out, []byte(secret)) {
			t.Fatalf("credential leaked to output")
		}
		if data, readErr := os.ReadFile(filepath.Join(dir, "result.json")); readErr == nil && bytes.Contains(data, []byte(secret)) {
			t.Fatal("credential leaked to audit")
		}
	})
	t.Run("committed Redis failure recovers without another version increment", func(t *testing.T) {
		db, work, targetDSN := fixture(t)
		blockedRedis, allowWrites := paymentMenuReadOnlyRedisProxy(t, redisURL)
		dir := t.TempDir()
		out, err := run(t, targetDSN, blockedRedis, dir, backup)
		if err == nil {
			t.Fatal("Redis publication failure reported success")
		}
		var menuVersion, accessVersion int64
		if err := db.WithContext(work).Raw("SELECT menu_version FROM permission_auth_platform WHERE id=?", platformID).Scan(&menuVersion).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.WithContext(work).Raw("SELECT version FROM permission_access_version WHERE user_id=?", userID).Scan(&accessVersion).Error; err != nil {
			t.Fatal(err)
		}
		if menuVersion != 20 || accessVersion != 5 {
			t.Fatalf("PostgreSQL must commit before Redis publication, versions=%d/%d output=%s", menuVersion, accessVersion, out)
		}
		var report struct {
			PostgresCommitted bool `json:"postgresCommitted"`
			RedisSynchronized bool `json:"redisSynchronized"`
		}
		data, err := os.ReadFile(filepath.Join(dir, "result.json"))
		if err != nil {
			t.Fatalf("failure audit missing: %v\n%s", err, out)
		}
		if err = json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if !report.PostgresCommitted || report.RedisSynchronized {
			t.Fatalf("wrong failure commit audit: %s", data)
		}
		before := paymentRedeemMenuDigest(t, db, work)
		// Recovery is bound to the same endpoint identity. The proxy can forward
		// writes after a simulated dependency repair, without changing the URL.
		allowWrites()
		if out, err = run(t, targetDSN, blockedRedis, t.TempDir(), "", "-recover", filepath.Join(dir, "result.json")); err != nil {
			t.Fatalf("recovery failed: %v\n%s", err, out)
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("recovery changed PostgreSQL facts or versions")
		}
		assertPaymentMenuRedisVersion(t, rc, platformID, 20, userID, 5)
		// A fresh repeat must be a no-op in both PostgreSQL and Redis.
		if out, err = run(t, targetDSN, blockedRedis, t.TempDir(), backup); err != nil {
			t.Fatalf("idempotent repeat: %v\n%s", err, out)
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("repeat changed PostgreSQL facts")
		}
		if value, found, err := rc.GetString(work, unrelatedKey); err != nil || !found || value != "keep" {
			t.Fatal("unrelated Redis key changed")
		}
		// A prepared audit with a lost COMMIT reply is resolved by exact facts,
		// not by blindly running SQL or trusting a boolean flag.
		var unknown map[string]json.RawMessage
		if err = json.Unmarshal(data, &unknown); err != nil {
			t.Fatal(err)
		}
		unknown["postgresCommitState"] = json.RawMessage(`"unknown"`)
		unknown["postgresCommitted"] = json.RawMessage(`false`)
		unknownBytes, err := json.Marshal(unknown)
		if err != nil {
			t.Fatal(err)
		}
		unknownPath := filepath.Join(dir, "unknown-commit.json")
		if err = os.WriteFile(unknownPath, unknownBytes, 0600); err != nil {
			t.Fatal(err)
		}
		if out, err = run(t, targetDSN, blockedRedis, t.TempDir(), "", "-recover", unknownPath); err != nil {
			t.Fatalf("unknown commit recovery: %v\n%s", err, out)
		}
		for _, field := range []string{"databaseIdentity", "redisIdentity", "sqlSHA256"} {
			original := unknown[field]
			unknown[field] = json.RawMessage(`"different-target"`)
			bad, err := json.Marshal(unknown)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "wrong-target.json")
			if err = os.WriteFile(path, bad, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = run(t, targetDSN, blockedRedis, t.TempDir(), "", "-recover", path); err == nil {
				t.Fatalf("recovery accepted different %s", field)
			}
			unknown[field] = original
		}
		for _, raw := range []string{`null`, `{"schemaVersion":2,"state":"ready","version":9007199254740993}`, `{"schemaVersion":2,"state":"invalidating","version":0,"mutationToken":"other","baseVersion":5}`} {
			if err = rc.SetString(work, state.StateKey(userID), raw, 0); err != nil {
				t.Fatal(err)
			}
			if _, err = run(t, targetDSN, blockedRedis, t.TempDir(), "", "-recover", unknownPath); err == nil {
				t.Fatalf("unsafe Redis state accepted")
			}
			if value, found, err := rc.GetString(work, state.StateKey(userID)); err != nil || !found || value != raw {
				t.Fatal("recovery overwrote unsafe Redis state")
			}
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("recovery rejection changed database")
		}
	})
	t.Run("zero affected users can recover", func(t *testing.T) {
		db, work, targetDSN := fixture(t)
		if err := db.WithContext(work).Exec("UPDATE user_account SET is_enabled=0").Error; err != nil {
			t.Fatal(err)
		}
		proxy, allowWrites := paymentMenuReadOnlyRedisProxy(t, redisURL)
		dir := t.TempDir()
		if _, err := run(t, targetDSN, proxy, dir, backup); err == nil {
			t.Fatal("Redis failure reported success")
		}
		before := paymentRedeemMenuDigest(t, db, work)
		allowWrites()
		if out, err := run(t, targetDSN, proxy, t.TempDir(), "", "-recover", filepath.Join(dir, "result.json")); err != nil {
			t.Fatalf("empty manifest recovery: %v\n%s", err, out)
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("empty recovery changed database")
		}
		assertPaymentMenuRedisVersion(t, rc, platformID, 20, userID, 4)
	})
	t.Run("failed replay preserves the original prepared user manifest", func(t *testing.T) {
		db, work, targetDSN := fixture(t)
		if err := db.WithContext(work).Exec("CREATE TABLE runner_iteration(value integer); INSERT INTO runner_iteration VALUES(0)").Error; err != nil {
			t.Fatal(err)
		}
		originalSQL, err := os.ReadFile(sqlPath)
		if err != nil {
			t.Fatal(err)
		}
		customSQL := append(originalSQL, []byte("\nUPDATE runner_iteration SET value=value+1; DO $$ BEGIN IF (SELECT value FROM runner_iteration)=2 THEN UPDATE permission_access_version SET version=version+1; END IF; END $$;")...)
		customPath := filepath.Join(t.TempDir(), "replay-conflict.sql")
		if err = os.WriteFile(customPath, customSQL, 0600); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		if _, err = run(t, targetDSN, redisURL, dir, backup, "-sql", customPath); err == nil {
			t.Fatal("non-idempotent replay accepted")
		}
		before := paymentRedeemMenuDigest(t, db, work)
		if out, err := run(t, targetDSN, redisURL, t.TempDir(), "", "-sql", customPath, "-recover", filepath.Join(dir, "result.json")); err != nil {
			t.Fatalf("original prepared manifest recovery: %v\n%s", err, out)
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("recovery re-executed non-idempotent SQL")
		}
		assertPaymentMenuRedisVersion(t, rc, platformID, 20, userID, 5)
	})
	t.Run("more than one Redis batch uses a bounded audit and complete manifests", func(t *testing.T) {
		db, work, targetDSN := fixture(t)
		for _, statement := range []string{
			"INSERT INTO user_account(id,is_enabled) SELECT ?::bigint+n,1 FROM generate_series(1,1000)n",
			"INSERT INTO permission_user_role(user_id,role_id) SELECT ?::bigint+n,5 FROM generate_series(1,1000)n",
			"INSERT INTO permission_access_version(user_id,version) SELECT ?::bigint+n,4 FROM generate_series(1,1000)n",
		} {
			if err := db.WithContext(work).Exec(statement, userID).Error; err != nil {
				t.Fatal(err)
			}
		}
		for i := int64(1); i <= 1000; i++ {
			key := state.StateKey(userID + i)
			if _, found, err := rc.GetString(work, key); err != nil || found {
				t.Fatal("Redis batch fixture key is occupied")
			}
			ownedKeys = append(ownedKeys, key)
		}
		dir := t.TempDir()
		if out, err := run(t, targetDSN, redisURL, dir, backup); err != nil {
			t.Fatalf("multi-batch maintenance: %v\n%s", err, out)
		}
		data, err := os.ReadFile(filepath.Join(dir, "result.json"))
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			PublishedUserCount int64 `json:"publishedUserCount"`
			Verified           bool  `json:"verified"`
		}
		if err = json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if !report.Verified || report.PublishedUserCount != 1001 || len(data) > 32<<10 {
			t.Fatalf("unbounded or incomplete audit: users=%d bytes=%d", report.PublishedUserCount, len(data))
		}
		for _, name := range []string{"before-users.jsonl", "after-users.jsonl"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Count(data, []byte("\n")) != 1001 {
				t.Fatal("incomplete user manifest")
			}
		}
		assertPaymentMenuRedisVersion(t, rc, platformID, 20, userID+1000, 5)
	})
	t.Run("check-only rejects malformed menus without changing facts", func(t *testing.T) {
		db, _, targetDSN := fixture(t)
		if out, err := run(t, targetDSN, redisURL, t.TempDir(), backup); err != nil {
			t.Fatalf("prepare deployment: %v\n%s", err, out)
		}
		for _, test := range []struct{ name, mutate, restore string }{
			{"disabled action", "UPDATE permission_menu SET is_enabled=0 WHERE code='payment:redeemCode:list'", "UPDATE permission_menu SET is_enabled=1 WHERE code='payment:redeemCode:list'"},
			{"wrong action parent", "UPDATE permission_menu SET parent_id=11 WHERE code='payment:redeemCode:generate'", "UPDATE permission_menu SET parent_id=(SELECT id FROM permission_menu WHERE code='payment:redeemCode:view') WHERE code='payment:redeemCode:generate'"},
			{"wrong page route", "UPDATE permission_menu SET component_path='payment/wrong' WHERE code='payment:redeemCode:view'", "UPDATE permission_menu SET component_path='payment/redeemCode' WHERE code='payment:redeemCode:view'"},
			{"wrong page type", "UPDATE permission_menu SET menu_type='directory' WHERE code='payment:redeemCode:view'", "UPDATE permission_menu SET menu_type='page' WHERE code='payment:redeemCode:view'"},
			{"duplicate code", "UPDATE permission_menu SET code='payment:redeemCode:detail' WHERE code='payment:redeemCode:list'", "UPDATE permission_menu SET code='payment:redeemCode:list' WHERE name='查询兑换码'"},
			{"route conflict", "INSERT INTO permission_menu(id,platform_id,menu_type,name,code,path,is_enabled,is_hidden) SELECT 900001,platform_id,'page','Conflict','conflict:view','/payment/redeemCode',1,0 FROM permission_menu WHERE code='payment:redeemCode:view'", "DELETE FROM permission_menu WHERE id=900001"},
		} {
			t.Run(test.name, func(t *testing.T) {
				if err := db.WithContext(ctx).Exec(test.mutate).Error; err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := db.WithContext(ctx).Exec(test.restore).Error; err != nil {
						t.Error(err)
					}
				})
				before := paymentRedeemMenuDigest(t, db, ctx)
				if out, err := run(t, targetDSN, redisURL, t.TempDir(), "", "-check"); err == nil {
					t.Fatalf("malformed menus reported verified: %s", out)
				}
				if paymentRedeemMenuDigest(t, db, ctx) != before {
					t.Fatal("check-only changed malformed database facts")
				}
				assertPaymentMenuRedisVersion(t, rc, platformID, 20, userID, 5)
			})
		}
	})
	t.Run("PowerShell runner preserves failure facts and recovers the archived schema", func(t *testing.T) {
		pwsh, err := exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell 7 is not installed")
		}
		db, _, _ := fixture(t)
		work := ctx
		var schema string
		if err = db.WithContext(work).Raw("SELECT current_schema()").Scan(&schema).Error; err != nil {
			t.Fatal(err)
		}
		proxy, allowWrites := paymentMenuReadOnlyRedisProxy(t, redisURL)
		runnerPath := filepath.Join(root, "docs/database/2026-10-17-payment-redeem-code-menu.ps1")
		invoke := func(dir string, args ...string) ([]byte, error) {
			params := []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-File", runnerPath, "-Schema", schema, "-AuditDirectory", dir}
			params = append(params, args...)
			command := exec.CommandContext(ctx, pwsh, params...)
			// The wrapper receives the normal libpq DSN. Its explicit -Schema
			// must bind both pg_dump and the pgx helper to the isolated fixture.
			command.Dir = root
			command.Env = append(os.Environ(), "POSTGRES_DSN="+dsn, "REDIS_URL="+proxy)
			return command.CombinedOutput()
		}
		dir := t.TempDir()
		out, err := invoke(dir, "-ServicesStopped")
		if err == nil {
			t.Fatal("publication failure reported success")
		}
		data, err := os.ReadFile(filepath.Join(dir, "audit.json"))
		if err != nil {
			t.Fatalf("outer failure audit missing: %v", err)
		}
		var report struct {
			PostgresCommitted   bool   `json:"postgresCommitted"`
			PostgresCommitState string `json:"postgresCommitState"`
			RedisSynchronized   bool   `json:"redisSynchronized"`
			Verified            bool   `json:"verified"`
			Recovered           bool   `json:"recovered"`
			FailureStage        string `json:"failureStage"`
			BackupSHA256        string `json:"backupSHA256"`
			Schema              string `json:"schema"`
		}
		if err = json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if !report.PostgresCommitted || report.PostgresCommitState != "committed" || report.RedisSynchronized || report.Verified || report.FailureStage != "redis-after" || report.Schema != schema {
			t.Fatalf("outer audit lost commit/failure facts: %s\n%s", data, out)
		}
		originalBackupHash := report.BackupSHA256
		before := paymentRedeemMenuDigest(t, db, work)
		allowWrites()
		recovered := t.TempDir()
		if out, err := invoke(recovered, "-ServicesStopped", "-RecoverFrom", filepath.Join(dir, "result.json")); err != nil {
			t.Fatalf("PowerShell recovery: %v\n%s", err, out)
		}
		data, err = os.ReadFile(filepath.Join(recovered, "audit.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if !report.PostgresCommitted || !report.RedisSynchronized || !report.Verified || !report.Recovered || report.BackupSHA256 != originalBackupHash || report.Schema != schema {
			t.Fatalf("outer recovery audit lost verified facts or original backup: %s", data)
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("PowerShell recovery changed PostgreSQL facts")
		}
		checkDir := t.TempDir()
		if out, err := invoke(checkDir, "-CheckOnly"); err != nil {
			t.Fatalf("PowerShell check-only: %v\n%s", err, out)
		}
		if paymentRedeemMenuDigest(t, db, work) != before {
			t.Fatal("PowerShell check-only changed PostgreSQL facts")
		}
		assertPaymentMenuRedisVersion(t, rc, platformID, 20, userID, 5)
	})
}

func paymentMenuSchemaDSN(t *testing.T, dsn, schema string) string {
	t.Helper()
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return dsn + " search_path=" + schema
}

func assertPaymentMenuRedisVersion(t *testing.T, rc *projectredis.Client, platformID, menuVersion, userID, accessVersion int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	menu, found, err := state.NewMenuStore(rc).Read(ctx, platformID)
	if err != nil || !found || menu.State != state.StateReady || menu.Version != menuVersion {
		t.Fatalf("menu state=%+v found=%v err=%v", menu, found, err)
	}
	user, found, err := state.NewStore(rc).Read(ctx, userID)
	if err != nil || !found || user.State != state.StateReady || user.Version != accessVersion {
		t.Fatalf("user state=%+v found=%v err=%v", user, found, err)
	}
}

// A TCP proxy forwards to real Redis while rejecting only writes made by the
// runner. It leaves the real Redis service and its other clients untouched.
func paymentMenuReadOnlyRedisProxy(t *testing.T, rawURL string) (string, func()) {
	t.Helper()
	options, err := goredis.ParseURL(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var allow atomic.Bool
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Host = listener.Addr().String()
	proxyURL := parsed.String()
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer client.Close()
				server, err := net.DialTimeout("tcp", options.Addr, 3*time.Second)
				if err != nil {
					return
				}
				defer server.Close()
				_ = client.SetDeadline(time.Now().Add(60 * time.Second))
				_ = server.SetDeadline(time.Now().Add(60 * time.Second))
				requests, responses := bufio.NewReader(client), bufio.NewReader(server)
				for {
					raw, err := readPaymentMenuRESP(requests)
					if err != nil {
						return
					}
					upper := strings.ToUpper(string(raw))
					blocked := strings.Contains(upper, "\r\nEVAL\r\n") || strings.Contains(upper, "\r\nSET\r\n")
					if blocked && !allow.Load() {
						if _, err = client.Write([]byte("-ERR controlled publication failure\r\n")); err != nil {
							return
						}
						continue
					}
					if _, err = server.Write(raw); err != nil {
						return
					}
					reply, err := readPaymentMenuRESP(responses)
					if err != nil {
						return
					}
					if _, err = client.Write(reply); err != nil {
						return
					}
				}
			}()
		}
	}()
	return proxyURL, func() { allow.Store(true) }
}

func readPaymentMenuRESP(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		return nil, io.ErrUnexpectedEOF
	}
	raw := append([]byte(nil), line...)
	switch line[0] {
	case '$', '!', '=':
		n, err := strconv.Atoi(strings.TrimSpace(string(line[1:])))
		if err != nil {
			return nil, err
		}
		if n >= 0 {
			body := make([]byte, n+2)
			if _, err = io.ReadFull(reader, body); err != nil {
				return nil, err
			}
			raw = append(raw, body...)
		}
	case '*', '%', '~', '>', '|':
		n, err := strconv.Atoi(strings.TrimSpace(string(line[1:])))
		if err != nil {
			return nil, err
		}
		if line[0] == '%' || line[0] == '|' {
			n *= 2
		}
		for i := 0; i < n; i++ {
			child, err := readPaymentMenuRESP(reader)
			if err != nil {
				return nil, err
			}
			raw = append(raw, child...)
		}
	}
	return raw, nil
}

const paymentRedeemCodeMenuRunnerBehaviorTests = `package main

import (
 "fmt"
 "os"
 "testing"

 "admin/server/internal/database/testschema"
 "github.com/joho/godotenv"
)

func TestApplyMigrationUsesConnectionSchema(t *testing.T) {
 if testing.Short() { t.Skip("PostgreSQL integration") }
 _ = godotenv.Load("../../.env")
 dsn := os.Getenv("POSTGRES_DSN")
 if dsn == "" { t.Skip("POSTGRES_DSN is not configured") }
 db, ctx := testschema.Open(t, dsn, "test_payment_menu_runner")
 var schema string
 if err := db.WithContext(ctx).Raw("SELECT current_schema()").Scan(&schema).Error; err != nil { t.Fatal(err) }
 // Raise before any writes if the helper switches away from the isolated schema.
 sql := fmt.Sprintf("DO $$ BEGIN IF current_schema() <> '%s' THEN RAISE EXCEPTION 'connection schema was overridden'; END IF; END $$", schema)
 if err := applyMigration(ctx, db, []byte(sql)); err != nil { t.Fatalf("migration did not stay in the connection schema: %v", err) }
}
`
