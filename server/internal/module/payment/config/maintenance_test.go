package config

import (
	projectredis "admin/server/internal/redis"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigMaintenanceRunnerReplayAndPartialFailure(t *testing.T) {
	db, ctx := configFixture(t, false)
	runConfigSQL(t, db, ctx)
	seedLegacyConfig(t, db, ctx)
	// Runner includes compilation, backup and several recovery invocations; keep
	// it bounded independently of the short single-query fixture setup budget.
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	var schema string
	if err := db.WithContext(ctx).Raw(`SELECT current_schema()`).Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(os.Getenv("REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	var r *projectredis.Client
	for _, number := range []int{10, 9, 8, 7, 6, 5, 4, 3, 14} {
		u.Path = fmt.Sprintf("/%d", number)
		candidate, err := projectredis.Open(ctx, u.String())
		if err != nil {
			t.Fatal(err)
		}
		n, err := candidate.UniversalClient().DBSize(ctx).Result()
		if err != nil {
			candidate.Close()
			t.Fatal(err)
		}
		if n == 0 {
			r = candidate
			break
		}
		candidate.Close()
	}
	if r == nil {
		t.Fatal("no empty isolated Redis database; no data changed")
	}
	marker := "test:payment-config-runner:owner"
	ok, err := r.UniversalClient().SetNX(ctx, marker, fmt.Sprint(os.Getpid()), 5*time.Minute).Result()
	if err != nil || !ok {
		t.Fatal("test Redis ownership unavailable")
	}
	t.Cleanup(func() {
		work, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r.DeleteMany(work, []string{marker, "config-cache:state:v1:payment.config:global", "authz:menu-state:v1:1"})
		r.Close()
	})
	content, err := os.ReadFile("../../../../../docs/database/2026-10-14-payment-config-remove-code.ps1")
	if err != nil {
		t.Fatal(err)
	}
	_, body, ok := strings.Cut(strings.ReplaceAll(string(content), "\r\n", "\n"), "$helper = @'\n")
	if !ok {
		t.Fatal("embedded helper missing")
	}
	body, _, ok = strings.Cut(body, "\n'@")
	if !ok {
		t.Fatal("helper terminator missing")
	}
	directory, err := os.MkdirTemp("../../../../", "payment-config-maintenance-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })
	source := filepath.Join(directory, "main.go")
	if err = os.WriteFile(source, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "maintenance.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", exe, source)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %s %v", out, err)
	}
	auditRoot := t.TempDir()
	sqlPath, err := filepath.Abs("../../../../../docs/database/2026-10-14-payment-config-remove-code.sql")
	if err != nil {
		t.Fatal(err)
	}
	dsn := os.Getenv("POSTGRES_DSN") + " search_path=" + schema
	run := func(redisURL, auditDir, recover string) (string, error) {
		cmd := exec.CommandContext(ctx, exe)
		cmd.Env = append(os.Environ(), "POSTGRES_DSN="+dsn, "REDIS_URL="+redisURL, "PAYMENT_CONFIG_SQL="+sqlPath, "PAYMENT_CONFIG_AUDIT="+auditDir, "PAYMENT_CONFIG_RECOVER="+recover)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	first := filepath.Join(auditRoot, "first")
	if out, err := run(u.String(), first, ""); err != nil {
		t.Fatalf("first: %s %v", out, err)
	}
	var audit struct {
		PostgresCommitted, RedisSynchronized, Verified bool
		BackupSHA256                                   string
	}
	raw, err := os.ReadFile(filepath.Join(first, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &audit); err != nil || !audit.PostgresCommitted || !audit.RedisSynchronized || !audit.Verified || audit.BackupSHA256 == "" {
		t.Fatalf("audit %s %v", raw, err)
	}
	var before string
	digest := `SELECT md5(jsonb_build_object('menus',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM permission_menu m),'platforms',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permission_auth_platform p),'state',(SELECT jsonb_agg(to_jsonb(s)) FROM system_config_cache_state s),'outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM system_config_cache_outbox o))::text)`
	if err = db.WithContext(ctx).Raw(digest).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	if out, err := run(u.String(), filepath.Join(auditRoot, "repeat"), ""); err != nil {
		t.Fatalf("repeat: %s %v", out, err)
	}
	var after string
	db.WithContext(ctx).Raw(digest).Scan(&after)
	if before != after {
		t.Fatal("repeat changed generation, outbox or menus")
	}
	user := fmt.Sprintf("test_payment_%d", time.Now().UnixNano())
	password := "local-test-only"
	if err := r.UniversalClient().Do(ctx, "ACL", "SETUSER", user, "on", ">"+password, "~*", "+ping", "+get", "+info", "+select", "+client").Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		work, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r.UniversalClient().Do(work, "ACL", "DELUSER", user)
	})
	restricted := *u
	restricted.User = url.UserPassword(user, password)
	partial := filepath.Join(auditRoot, "partial")
	if _, err := run(restricted.String(), partial, ""); err == nil {
		t.Fatal("Redis sync failure returned success")
	}
	raw, err = os.ReadFile(filepath.Join(partial, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	audit.PostgresCommitted = false
	audit.RedisSynchronized = true
	if err = json.Unmarshal(raw, &audit); err != nil || !audit.PostgresCommitted || audit.RedisSynchronized {
		t.Fatalf("partial audit %s %v", raw, err)
	}
	if out, err := run(u.String(), filepath.Join(auditRoot, "recover"), filepath.Join(partial, "audit.json")); err != nil {
		t.Fatalf("recover: %s %v", out, err)
	}
	// Model a lost COMMIT acknowledgement / crash before the committed audit write.
	// Only facts recorded before COMMIT may authorize recovery.
	var prior map[string]any
	if err := json.Unmarshal(raw, &prior); err != nil {
		t.Fatal(err)
	}
	prior["PostgresCommitted"] = false
	prior["PostgresAttempted"] = true
	prior["PostgresOutcome"] = "commit_unknown"
	prior["After"] = map[string]any{}
	unknown := filepath.Join(auditRoot, "unknown.json")
	changed, _ := json.Marshal(prior)
	if err := os.WriteFile(unknown, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := run(u.String(), filepath.Join(auditRoot, "recover-unknown"), unknown); err != nil {
		t.Fatalf("uncertain COMMIT recovery: %s %v", out, err)
	}
	// A restart changes run_id and can lose only these two state keys. Do not
	// actually restart the shared Redis process; exercise equivalent loss facts.
	prior["RedisRunID"] = "previous-redis-process"
	restart := filepath.Join(auditRoot, "restart.json")
	changed, _ = json.Marshal(prior)
	os.WriteFile(restart, changed, 0600)
	if err := r.DeleteMany(ctx, []string{"authz:menu-state:v1:1", "config-cache:state:v1:payment.config:global"}); err != nil {
		t.Fatal(err)
	}
	if out, err := run(u.String(), filepath.Join(auditRoot, "recover-state-loss"), restart); err != nil {
		t.Fatalf("Redis run/state-loss recovery: %s %v", out, err)
	}
	expected, ok := prior["Expected"].(map[string]any)
	if !ok {
		t.Fatal("pre-COMMIT expected facts not persisted")
	}
	expected["Digest"] = "different-facts"
	drift := filepath.Join(auditRoot, "drift.json")
	changed, _ = json.Marshal(prior)
	os.WriteFile(drift, changed, 0600)
	if _, err := run(u.String(), filepath.Join(auditRoot, "reject-drift"), drift); err == nil {
		t.Fatal("recovery accepted different expected facts")
	}
	// Keep menu/generation facts intact but remove payment schema in this isolated
	// test schema. A rolled-back CREATE branch is not proof of committed objects.
	if err := db.WithContext(ctx).Exec(`DROP TABLE payment_config,payment_config_version CASCADE; DROP FUNCTION payment_config_guard(); DROP FUNCTION payment_config_version_guard()`).Error; err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteMany(ctx, []string{"authz:menu-state:v1:1", "config-cache:state:v1:payment.config:global"}); err != nil {
		t.Fatal(err)
	}
	rejected := filepath.Join(auditRoot, "reject-missing-schema")
	if out, err := run(u.String(), rejected, filepath.Join(partial, "audit.json")); err == nil {
		t.Fatalf("recovery rebuilt missing tables only inside rollback and claimed success: %s", out)
	}
	rejectedRaw, err := os.ReadFile(filepath.Join(rejected, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rejectedAudit struct{ Verified, RedisSynchronized bool }
	if err := json.Unmarshal(rejectedRaw, &rejectedAudit); err != nil || rejectedAudit.Verified || rejectedAudit.RedisSynchronized {
		t.Fatal("missing schema recovery claimed verified", err)
	}
	for _, key := range []string{"authz:menu-state:v1:1", "config-cache:state:v1:payment.config:global"} {
		if _, found, err := r.GetString(ctx, key); found || err != nil {
			t.Fatal("rejected recovery published Redis", err)
		}
	}
}
