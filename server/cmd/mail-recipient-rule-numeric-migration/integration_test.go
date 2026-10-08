package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	projectredis "admin/server/internal/redis"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func fixtureSettings(t *testing.T) config.Worker {
	t.Helper()
	if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal("load test environment")
	}
	s, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal("test PostgreSQL/Redis configuration unavailable")
	}
	return s
}

func TestSyncMailStateMonotonicIdempotentAndMutationGuard(t *testing.T) {
	s := fixtureSettings(t)
	u, err := url.Parse(s.RedisURL)
	if err != nil {
		t.Fatal("parse test Redis URL")
	}
	u.Path = "/13"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := projectredis.Open(ctx, u.String())
	if err != nil {
		t.Fatal("test Redis connection unavailable")
	}
	defer client.Close()
	scope := cachegeneration.Scope{Namespace: "test.mailnumeric", ScopeKey: fmt.Sprintf("%d", time.Now().UnixNano())}
	key := cachegeneration.StateKey(scope)
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
	store := cachegeneration.NewStore(client)
	if err := syncMailState(ctx, store, scope, 3); err != nil {
		t.Fatal(err)
	}
	if err := syncMailState(ctx, store, scope, 4); err != nil {
		t.Fatal(err)
	}
	before, _, _ := client.GetString(ctx, key)
	if err := syncMailState(ctx, store, scope, 4); err != nil {
		t.Fatal(err)
	}
	after, _, _ := client.GetString(ctx, key)
	if before != after {
		t.Fatal("idempotent sync altered state")
	}
	if err := syncMailState(ctx, store, scope, 3); err == nil {
		t.Fatal("downgrade accepted")
	}
	lease, err := store.Acquire(ctx, scope, 4)
	if err != nil {
		t.Fatal(err)
	}
	before, _, _ = client.GetString(ctx, key)
	if err := syncMailState(ctx, store, scope, 5); err == nil {
		t.Fatal("active mutation overridden")
	}
	after, _, _ = client.GetString(ctx, key)
	if before != after {
		t.Fatal("active mutation changed")
	}
	if err := lease.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.SetString(ctx, key, "corrupt", 0); err != nil {
		t.Fatal(err)
	}
	if err := syncMailState(ctx, store, scope, 5); err == nil {
		t.Fatal("corrupt state repaired")
	}
	after, _, _ = client.GetString(ctx, key)
	if after != "corrupt" {
		t.Fatal("corrupt state overwritten")
	}
	if err := syncMailState(ctx, store, scope, 0); err == nil {
		t.Fatal("invalid authoritative generation accepted")
	}
}

func postgresFixture(t *testing.T) (*pgx.Conn, context.Context) {
	t.Helper()
	s := fixtureSettings(t)
	db, ctx := testschema.Open(t, s.PostgresDSN, "test_mail_numeric_helper")
	var schema string
	if err := db.WithContext(ctx).Raw(`SELECT current_schema()`).Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	if schema == "public" {
		t.Fatal("fixture is not isolated")
	}
	cfg, err := pgx.ParseConfig(s.PostgresDSN)
	if err != nil {
		t.Fatal("parse test PostgreSQL DSN")
	}
	cfg.RuntimeParams["search_path"] = schema
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal("connect isolated PostgreSQL")
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	_, err = conn.Exec(ctx, `CREATE TABLE message_mail_recipient_rule(id bigint PRIMARY KEY,scope varchar(16) NOT NULL,pattern text NOT NULL,action varchar(16) NOT NULL,name text NOT NULL,remark text NOT NULL,is_enabled smallint NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,deleted_at timestamptz);
CREATE TABLE system_config_cache_generation(namespace text,scope_key text,generation bigint,created_at timestamptz,updated_at timestamptz);
CREATE TABLE permission_auth_platform(id bigint,menu_version bigint);
CREATE TABLE system_config_cache_outbox(id bigint,namespace text,scope_key text,generation bigint,attempts int,available_at timestamptz,created_at timestamptz,updated_at timestamptz,published_at timestamptz,lock_token text);
INSERT INTO message_mail_recipient_rule VALUES (2,'domain','example.test','deny','deleted','keep',0,'2026-01-01','2026-01-02','2026-01-03'),(1,'email','a@example.test','allow','active','keep',1,'2026-01-01','2026-01-02',NULL);
INSERT INTO system_config_cache_generation VALUES ('message.mail','global',3,'2026-01-01','2026-01-02'),('system.setting','global',8,'2026-01-01','2026-01-02'),('message.mail','other',9,'2026-01-01','2026-01-02');
INSERT INTO permission_auth_platform VALUES (1,7),(2,5);
INSERT INTO system_config_cache_outbox VALUES (1,'message.mail','global',3,1,'2026-01-01','2026-01-01','2026-01-02',NULL,'secret-test-token');`)
	if err != nil {
		t.Fatal(err)
	}
	return conn, ctx
}

func TestPostgresSnapshotNormalizesLegacyAndPreservesEveryFact(t *testing.T) {
	db, ctx := postgresFixture(t)
	before, err := inspectPostgres(ctx, db, false)
	if err != nil {
		t.Fatal(err)
	}
	if before.ScopeType != "character varying" || before.ActionType != "character varying" || before.RowCount != 2 || before.MailGeneration != 3 {
		t.Fatalf("legacy metadata mismatch: %+v", before)
	}
	if _, err := inspectPostgres(ctx, db, true); err == nil {
		t.Fatal("sync accepted legacy columns")
	}
	if _, err := db.Exec(ctx, `ALTER TABLE message_mail_recipient_rule ALTER COLUMN scope TYPE smallint USING CASE scope WHEN 'email' THEN 0 WHEN 'domain' THEN 1 END, ALTER COLUMN action TYPE smallint USING CASE action WHEN 'deny' THEN 0 WHEN 'allow' THEN 1 END;`); err != nil {
		t.Fatal(err)
	}
	after, err := inspectPostgres(ctx, db, true)
	if err != nil {
		t.Fatal(err)
	}
	if after.ScopeType != "smallint" || after.ActionType != "smallint" || after.RuleFactsHash != before.RuleFactsHash || after.ControlHash != before.ControlHash || after.MailOutboxHash != before.MailOutboxHash {
		t.Fatalf("migration snapshot drift: before=%+v after=%+v", before, after)
	}
	for _, tc := range []struct{ sql, field string }{{`UPDATE message_mail_recipient_rule SET remark='changed' WHERE id=2`, "rule"}, {`UPDATE permission_auth_platform SET menu_version=menu_version+1 WHERE id=2`, "control"}, {`UPDATE system_config_cache_generation SET generation=generation+1 WHERE scope_key='other'`, "control"}, {`UPDATE system_config_cache_outbox SET lock_token='different'`, "outbox"}} {
		prev, err := inspectPostgres(ctx, db, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, tc.sql); err != nil {
			t.Fatal(err)
		}
		next, err := inspectPostgres(ctx, db, false)
		if err != nil {
			t.Fatal(err)
		}
		if tc.field == "rule" && next.RuleFactsHash == prev.RuleFactsHash || tc.field == "control" && next.ControlHash == prev.ControlHash || tc.field == "outbox" && next.MailOutboxHash == prev.MailOutboxHash {
			t.Fatalf("%s fact change omitted", tc.field)
		}
	}
	if _, err := db.Exec(ctx, `UPDATE message_mail_recipient_rule SET scope=2 WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectPostgres(ctx, db, true); err == nil {
		t.Fatal("invalid soft-deleted enum accepted")
	}
}

func TestPostgresSnapshotEmptyRowsAndMalformedInput(t *testing.T) {
	for _, tc := range []struct {
		name, setup string
		valid       bool
	}{
		{"empty rules", `DELETE FROM message_mail_recipient_rule`, true},
		{"invalid legacy action", `UPDATE message_mail_recipient_rule SET action='unknown' WHERE id=2`, false},
		{"numeric spelling in legacy", `UPDATE message_mail_recipient_rule SET scope='0' WHERE id=2`, false},
		{"null scope", `ALTER TABLE message_mail_recipient_rule ALTER COLUMN scope DROP NOT NULL; UPDATE message_mail_recipient_rule SET scope=NULL WHERE id=2`, false},
		{"mixed columns", `ALTER TABLE message_mail_recipient_rule ALTER COLUMN scope TYPE smallint USING CASE scope WHEN 'email' THEN 0 ELSE 1 END`, false},
		{"missing generation", `DELETE FROM system_config_cache_generation WHERE namespace='message.mail' AND scope_key='global'`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, ctx := postgresFixture(t)
			if _, err := db.Exec(ctx, tc.setup); err != nil {
				t.Fatal(err)
			}
			r, err := inspectPostgres(ctx, db, false)
			if !tc.valid && err == nil {
				t.Fatal("malformed facts accepted")
			}
			if tc.valid && (err != nil || r.RowCount != 0 || r.RuleFactsHash == "") {
				t.Fatalf("empty snapshot: %+v %v", r, err)
			}
		})
	}
}
