package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"admin/server/internal/database/testschema"
	permissionstate "admin/server/internal/module/permission/state"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type testMenuState struct {
	state permissionstate.State
	found bool
	fail  bool
	calls int
}

func (s *testMenuState) Read(context.Context, int64) (permissionstate.State, bool, error) {
	return s.state, s.found, nil
}
func (s *testMenuState) RebuildReadyState(_ context.Context, _ int64, version int64) error {
	s.calls++
	if s.fail {
		return errors.New("Redis unavailable")
	}
	s.state, s.found = permissionstate.State{State: permissionstate.StateReady, Version: version}, true
	return nil
}

func TestMigrationCreatesOptionalPageWithoutActionAndIsIdempotent(t *testing.T) {
	db, ctx, sql := fixture(t)
	store := &testMenuState{}
	path := filepath.Join(t.TempDir(), "audit.json")
	if err := execute(ctx, db, store, sql, path); err != nil {
		t.Fatal(err)
	}
	first, err := readFacts(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 25 || first.Pages != 1 || first.Actions != 0 || store.state.Version != 25 {
		t.Fatalf("facts=%+v state=%+v", first, store.state)
	}
	if err := execute(ctx, db, store, sql, path); err != nil {
		t.Fatal(err)
	}
	next, _ := readFacts(ctx, db)
	if next != first {
		t.Fatalf("repeat changed facts: %+v -> %+v", first, next)
	}
	var canvasVersion, grants int
	if err := db.QueryRow(ctx, `SELECT menu_version FROM permission_auth_platform WHERE code='canvas'`).Scan(&canvasVersion); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM permission_role_menu`).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if canvasVersion != 5 || grants != 0 {
		t.Fatal("changed another platform or implicitly granted roles")
	}
}

func TestMigrationConflictRollsBackPageAndVersion(t *testing.T) {
	db, ctx, sql := fixture(t)
	// A route already owned by a different page must not be overwritten.
	_, err := db.Exec(ctx, `INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,path,component_path) VALUES(1,1,'page','冲突','system:other:view','/system/server','system/other')`)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := readFacts(ctx, db)
	if err := execute(ctx, db, &testMenuState{}, sql, filepath.Join(t.TempDir(), "audit.json")); err == nil {
		t.Fatal("conflict accepted")
	}
	after, _ := readFacts(ctx, db)
	if before != after {
		t.Fatalf("partial commit: %+v -> %+v", before, after)
	}
}

func TestMigrationRedisFailureIsAuditedAndRepeatRepairsWithoutVersionBump(t *testing.T) {
	db, ctx, sql := fixture(t)
	store := &testMenuState{fail: true}
	path := filepath.Join(t.TempDir(), "audit.json")
	if err := execute(ctx, db, store, sql, path); err == nil {
		t.Fatal("sync failure hidden")
	}
	before, _ := readFacts(ctx, db)
	if before.Version != 25 {
		t.Fatal("database was not committed before sync")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var a audit
	if err = json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if !a.PostgresCommitted || a.RedisSynchronized || a.Error == "" {
		t.Fatalf("invalid failure audit: %s", raw)
	}
	store.fail = false
	if err := execute(ctx, db, store, sql, path); err != nil {
		t.Fatal(err)
	}
	after, _ := readFacts(ctx, db)
	if before != after || store.state.Version != 25 {
		t.Fatal("repair changed database or failed to sync")
	}
}

func TestMigrationRejectsActiveOrAheadMenuStateBeforeChangingFacts(t *testing.T) {
	for _, state := range []permissionstate.State{{State: permissionstate.StateInvalidating, Version: 24}, {State: permissionstate.StateReady, Version: 26}} {
		t.Run(state.State+fmt.Sprint(state.Version), func(t *testing.T) {
			db, ctx, sql := fixture(t)
			store := &testMenuState{state: state, found: true}
			before, _ := readFacts(ctx, db)
			if err := execute(ctx, db, store, sql, filepath.Join(t.TempDir(), "audit.json")); err == nil {
				t.Fatal("invalid state accepted")
			}
			after, _ := readFacts(ctx, db)
			if before != after || store.calls != 0 {
				t.Fatal("rejected preflight mutated state")
			}
		})
	}
}

func fixture(t *testing.T) (*pgx.Conn, context.Context, []byte) {
	t.Helper()
	values, err := godotenv.Read("../../.env")
	if err != nil {
		t.Fatal("read test database configuration")
	}
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = values["POSTGRES_DSN"]
	}
	gormDB, ctx := testschema.Open(t, dsn, "test_system_server")
	var schema string
	if err := gormDB.WithContext(ctx).Raw(`SELECT current_schema()`).Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("parse test database configuration")
	}
	cfg.RuntimeParams["search_path"] = schema
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	db, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal("connect isolated schema")
	}
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	_, err = db.Exec(ctx, `
CREATE TABLE permission_auth_platform(id bigint PRIMARY KEY,code text UNIQUE NOT NULL,status smallint NOT NULL DEFAULT 1,menu_version bigint NOT NULL,updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at timestamptz);
CREATE TABLE permission_menu(id bigserial PRIMARY KEY,platform_id bigint NOT NULL REFERENCES permission_auth_platform(id),parent_id bigint REFERENCES permission_menu(id),menu_type text NOT NULL,name text NOT NULL,code text NOT NULL,i18n_key text,path text,component_path text,icon text,sort_order integer NOT NULL DEFAULT 0,status smallint NOT NULL DEFAULT 1,is_hidden smallint NOT NULL DEFAULT 0,remark text,created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at timestamptz);
CREATE UNIQUE INDEX permission_menu_code ON permission_menu(platform_id,code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX permission_menu_path ON permission_menu(platform_id,path) WHERE deleted_at IS NULL AND path IS NOT NULL;
CREATE TABLE permission_role_menu(id bigserial PRIMARY KEY,role_id bigint,menu_id bigint REFERENCES permission_menu(id));
INSERT INTO permission_auth_platform(id,code,menu_version) VALUES(1,'admin',24),(2,'canvas',5);
INSERT INTO permission_menu(platform_id,menu_type,name,code,i18n_key) VALUES(1,'directory','系统','system','navigation.system');`)
	if err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile("../../../docs/database/2026-11-06-system-server.sql")
	if err != nil {
		t.Fatal(err)
	}
	return db, ctx, sql
}
