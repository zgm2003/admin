package database_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"admin/server/internal/database/testschema"
	"gorm.io/gorm"
)

var realtimeNotificationTables = []string{
	"message_notification_task",
	"message_notification_task_target",
	"message_notification",
	"message_notification_recipient",
	"message_notification_broadcast_state",
	"message_notification_mailbox_state",
	"message_notification_dispatch_outbox",
	"realtime_event",
	"realtime_event_outbox",
	"realtime_retention_state",
}

type realtimeNotificationFacts struct {
	TableCount        int64
	Settings          string
	Generation        int64
	OutboxCount       int64
	AdminMenuVersion  int64
	CanvasMenuVersion int64
	AdminMenus        int64
	CanvasMenus       int64
	RoleGrants        int64
	AccessVersions    string
}

func TestRealtimeNotificationMigrationIsIdempotent(t *testing.T) {
	db, ctx := openRealtimeNotificationMigrationSchema(t)
	seedRealtimeNotificationPrerequisites(t, db, ctx)
	before := readRealtimeNotificationFacts(t, db, ctx)
	script := readRealtimeNotificationMigration(t)

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute migration: %v", err)
	}
	first := readRealtimeNotificationFacts(t, db, ctx)
	assertRealtimeNotificationStructure(t, db, ctx)
	assertRealtimeNotificationSeed(t, db, ctx)
	if first.TableCount != 10 || first.Generation != before.Generation+1 || first.OutboxCount != before.OutboxCount+1 {
		t.Fatalf("unexpected first migration facts: before=%+v after=%+v", before, first)
	}
	if first.AdminMenuVersion != before.AdminMenuVersion+1 || first.CanvasMenuVersion != before.CanvasMenuVersion {
		t.Fatalf("menu versions changed incorrectly: before=%+v after=%+v", before, first)
	}
	if first.CanvasMenus != before.CanvasMenus || first.AdminMenus != before.AdminMenus+13 {
		t.Fatalf("menu rows changed incorrectly: before=%+v after=%+v", before, first)
	}
	if first.RoleGrants != before.RoleGrants+8 {
		t.Fatalf("notification grants=%d want=%d", first.RoleGrants, before.RoleGrants+8)
	}
	if first.AccessVersions != "100:8,101:10" {
		t.Fatalf("access versions=%q", first.AccessVersions)
	}

	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("re-execute migration: %v", err)
	}
	second := readRealtimeNotificationFacts(t, db, ctx)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("migration is not idempotent:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestRealtimeNotificationMigrationPreservesValidRetentionValues(t *testing.T) {
	db, ctx := openRealtimeNotificationMigrationSchema(t)
	seedRealtimeNotificationPrerequisites(t, db, ctx)
	createdAt := time.Date(2026, 9, 1, 1, 2, 3, 0, time.UTC)
	for _, statement := range []string{
		`INSERT INTO system_setting (id,setting_key,value,value_type,description,is_enabled,is_builtin,created_at,updated_at) VALUES (10,'message.notification.retention_days','365',2,'old',1,1,?,?)`,
		`INSERT INTO system_setting (id,setting_key,value,value_type,description,is_enabled,is_builtin,created_at,updated_at) VALUES (11,'realtime.event.retention_days','14',2,'old',1,1,?,?)`,
	} {
		if err := db.WithContext(ctx).Exec(statement, createdAt, createdAt).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.WithContext(ctx).Exec(readRealtimeNotificationMigration(t)).Error; err != nil {
		t.Fatal(err)
	}
	var values string
	if err := db.WithContext(ctx).Raw(`SELECT string_agg(setting_key || '=' || value,',' ORDER BY setting_key) FROM system_setting WHERE deleted_at IS NULL AND setting_key IN ('message.notification.retention_days','realtime.event.retention_days')`).Scan(&values).Error; err != nil {
		t.Fatal(err)
	}
	if values != "message.notification.retention_days=365,realtime.event.retention_days=14" {
		t.Fatalf("settings=%q", values)
	}
}

func TestRealtimeNotificationMigrationRollsBackOnInvalidPrerequisites(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, *gorm.DB, context.Context)
		want    string
	}{
		{
			name: "missing admin message directory",
			prepare: func(t *testing.T, db *gorm.DB, ctx context.Context) {
				t.Helper()
				if err := db.WithContext(ctx).Exec(`DELETE FROM permission_menu WHERE platform_id=1 AND code='message'`).Error; err != nil {
					t.Fatal(err)
				}
			},
			want: "Admin message directory",
		},
		{
			name: "conflicting notification code",
			prepare: func(t *testing.T, db *gorm.DB, ctx context.Context) {
				t.Helper()
				if err := db.WithContext(ctx).Exec(`INSERT INTO permission_menu (id,platform_id,parent_id,menu_type,name,code,is_enabled,is_hidden,created_at,updated_at) VALUES (99,1,10,'action','conflict','message:notification:view',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error; err != nil {
					t.Fatal(err)
				}
			},
			want: "message:notification:view",
		},
		{
			name: "invalid existing notification retention",
			prepare: func(t *testing.T, db *gorm.DB, ctx context.Context) {
				t.Helper()
				if err := db.WithContext(ctx).Exec(`INSERT INTO system_setting (id,setting_key,value,value_type,description,is_enabled,is_builtin,created_at,updated_at) VALUES (10,'message.notification.retention_days','29',2,'bad',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error; err != nil {
					t.Fatal(err)
				}
			},
			want: "message.notification.retention_days",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, ctx := openRealtimeNotificationMigrationSchema(t)
			seedRealtimeNotificationPrerequisites(t, db, ctx)
			test.prepare(t, db, ctx)
			before := readRealtimeNotificationFacts(t, db, ctx)
			err := db.WithContext(ctx).Exec(readRealtimeNotificationMigration(t)).Error
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want contains %q", err, test.want)
			}
			after := readRealtimeNotificationFacts(t, db, ctx)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("migration did not roll back: before=%+v after=%+v", before, after)
			}
			for _, table := range realtimeNotificationTables {
				assertTableMissing(t, db, ctx, table)
			}
		})
	}
}

func openRealtimeNotificationMigrationSchema(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, ctx := testschema.Open(t, mustPostgresDSN(t), "test_realtime_notification_migration")
	statements := []string{
		`CREATE TABLE user_account (id BIGINT PRIMARY KEY,is_enabled SMALLINT NOT NULL DEFAULT 1,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_auth_platform (id BIGINT PRIMARY KEY,code VARCHAR(49) NOT NULL,is_enabled SMALLINT NOT NULL DEFAULT 1,menu_version BIGINT NOT NULL DEFAULT 1,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_menu (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,platform_id BIGINT NOT NULL REFERENCES permission_auth_platform(id),parent_id BIGINT NULL REFERENCES permission_menu(id),menu_type VARCHAR(16) NOT NULL,name VARCHAR(128) NOT NULL,code VARCHAR(128) NOT NULL,i18n_key VARCHAR(128) NULL,path VARCHAR(255) NULL,component_path VARCHAR(255) NULL,icon VARCHAR(128) NULL,sort_order INTEGER NOT NULL DEFAULT 0,is_enabled SMALLINT NOT NULL DEFAULT 1,is_hidden SMALLINT NOT NULL DEFAULT 0,remark VARCHAR(255) NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at TIMESTAMPTZ NULL)`,
		`CREATE UNIQUE INDEX ux_permission_menu_code_active ON permission_menu(platform_id,code) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX ux_permission_menu_page_path_active ON permission_menu(platform_id,path) WHERE deleted_at IS NULL AND menu_type='page'`,
		`CREATE TABLE permission_role (id BIGINT PRIMARY KEY,code VARCHAR(64) NOT NULL,name VARCHAR(128) NOT NULL,is_default SMALLINT NOT NULL DEFAULT 0,is_enabled SMALLINT NOT NULL DEFAULT 1,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at TIMESTAMPTZ NULL)`,
		`CREATE TABLE permission_role_menu (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,role_id BIGINT NOT NULL REFERENCES permission_role(id),menu_id BIGINT NOT NULL REFERENCES permission_menu(id),created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at TIMESTAMPTZ NULL)`,
		`CREATE UNIQUE INDEX ux_permission_role_menu_active ON permission_role_menu(role_id,menu_id) WHERE deleted_at IS NULL`,
		`CREATE TABLE permission_user_role (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,user_id BIGINT NOT NULL REFERENCES user_account(id),role_id BIGINT NOT NULL REFERENCES permission_role(id),created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at TIMESTAMPTZ NULL)`,
		`CREATE UNIQUE INDEX ux_permission_user_role_active ON permission_user_role(user_id,role_id) WHERE deleted_at IS NULL`,
		`CREATE TABLE permission_access_version (user_id BIGINT PRIMARY KEY REFERENCES user_account(id),version BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE system_setting (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,setting_key VARCHAR(128) NOT NULL,value TEXT NOT NULL,value_type SMALLINT NOT NULL,description VARCHAR(512) NOT NULL DEFAULT '',is_enabled SMALLINT NOT NULL DEFAULT 1,is_builtin SMALLINT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,deleted_at TIMESTAMPTZ NULL)`,
		`CREATE UNIQUE INDEX ux_system_setting_key_active ON system_setting(setting_key) WHERE deleted_at IS NULL`,
		`CREATE TABLE system_config_cache_generation (namespace VARCHAR(64) NOT NULL,scope_key VARCHAR(128) NOT NULL,generation BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(namespace,scope_key))`,
		`CREATE TABLE system_config_cache_outbox (id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,namespace VARCHAR(64) NOT NULL,scope_key VARCHAR(128) NOT NULL,generation BIGINT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,available_at TIMESTAMPTZ NOT NULL,locked_until TIMESTAMPTZ NULL,lock_token VARCHAR(64) NULL,last_error VARCHAR(512) NOT NULL DEFAULT '',published_at TIMESTAMPTZ NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,UNIQUE(namespace,scope_key,generation))`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("prepare schema: %v", err)
		}
	}
	return db, ctx
}

func seedRealtimeNotificationPrerequisites(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	statements := []string{
		`INSERT INTO permission_auth_platform(id,code,is_enabled,menu_version) VALUES (1,'admin',1,3),(2,'canvas',1,7)`,
		`INSERT INTO permission_menu(id,platform_id,parent_id,menu_type,name,code,i18n_key,sort_order,is_enabled,is_hidden) VALUES (10,1,NULL,'directory','消息服务','message','navigation.message',400,1,0),(20,2,NULL,'directory','Canvas','canvas','navigation.canvas',10,1,0)`,
		`SELECT setval(pg_get_serial_sequence('permission_menu','id'),(SELECT max(id) FROM permission_menu),true)`,
		`INSERT INTO permission_role(id,code,name,is_default,is_enabled) VALUES (1,'admin','Admin',0,1),(2,'viewer','Viewer',0,1)`,
		`INSERT INTO user_account(id,is_enabled) VALUES (100,1),(101,1)`,
		`INSERT INTO permission_user_role(user_id,role_id) VALUES (100,1),(101,2)`,
		`INSERT INTO permission_access_version(user_id,version) VALUES (100,7),(101,9)`,
		`INSERT INTO system_config_cache_generation(namespace,scope_key,generation) VALUES ('system.setting','global',4)`,
		`INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,available_at,published_at) VALUES ('system.setting','global',4,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			t.Fatalf("seed prerequisites: %v", err)
		}
	}
}

func readRealtimeNotificationFacts(t *testing.T, db *gorm.DB, ctx context.Context) realtimeNotificationFacts {
	t.Helper()
	facts := realtimeNotificationFacts{}
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ?`, realtimeNotificationTables).Scan(&facts.TableCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT COALESCE(string_agg(setting_key || '=' || value || ':' || value_type || ':' || is_enabled || ':' || is_builtin,',' ORDER BY setting_key),'') FROM system_setting WHERE deleted_at IS NULL AND setting_key IN ('message.notification.retention_days','realtime.event.retention_days')`).Scan(&facts.Settings).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT generation FROM system_config_cache_generation WHERE namespace='system.setting' AND scope_key='global'`).Scan(&facts.Generation).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global'`).Scan(&facts.OutboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=1`).Scan(&facts.AdminMenuVersion).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT menu_version FROM permission_auth_platform WHERE id=2`).Scan(&facts.CanvasMenuVersion).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id=1 AND deleted_at IS NULL`).Scan(&facts.AdminMenus).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id=2 AND deleted_at IS NULL`).Scan(&facts.CanvasMenus).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.deleted_at IS NULL AND m.platform_id=1 AND m.code IN ('message:notification:view','message:notification:list','message:notification:read','message:notification:delete')`).Scan(&facts.RoleGrants).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT COALESCE(string_agg(user_id || ':' || version,',' ORDER BY user_id),'') FROM permission_access_version`).Scan(&facts.AccessVersions).Error; err != nil {
		t.Fatal(err)
	}
	return facts
}

func assertRealtimeNotificationStructure(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	for _, table := range realtimeNotificationTables {
		assertTableExists(t, db, ctx, table)
	}
	for _, index := range []string{"ux_message_notification_source", "ux_message_notification_source_task", "ux_message_notification_task_target_active", "ux_message_notification_recipient", "ux_message_notification_broadcast_state", "ux_realtime_event_event_id", "ux_realtime_event_dedup_key", "ux_realtime_event_outbox_event", "ux_message_notification_dispatch_batch", "ix_permission_user_role_role_history_cursor"} {
		var exists bool
		if err := db.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname=?)`, index).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("index %s is missing", index)
		}
	}
	var taskNotificationColumn int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_notification_task' AND column_name='notification_id'`).Scan(&taskNotificationColumn).Error; err != nil {
		t.Fatal(err)
	}
	if taskNotificationColumn != 0 {
		t.Fatal("message_notification_task must not contain notification_id")
	}
}

func assertRealtimeNotificationSeed(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	facts := readRealtimeNotificationFacts(t, db, ctx)
	if facts.Settings != "message.notification.retention_days=180:2:1:1,realtime.event.retention_days=7:2:1:1" {
		t.Fatalf("settings=%q", facts.Settings)
	}
	var invalid int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM permission_menu WHERE platform_id=1 AND deleted_at IS NULL AND ((code='message:notification:view' AND NOT (menu_type='page' AND parent_id IS NULL AND is_hidden=1 AND path='/message/notification' AND component_path='message/notification')) OR (code LIKE 'message:notification:%' AND code<>'message:notification:view' AND NOT (menu_type='action' AND is_hidden=1 AND parent_id IS NOT NULL AND path IS NULL AND component_path IS NULL AND icon IS NULL AND i18n_key IS NULL)))`).Scan(&invalid).Error; err != nil {
		t.Fatal(err)
	}
	if invalid != 0 {
		t.Fatalf("invalid personal notification menu rows=%d", invalid)
	}
}

func readRealtimeNotificationMigration(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "docs", "database", "2026-09-18-realtime-notification.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatalf("migration %s is empty", path)
	}
	return string(data)
}

func debugRealtimeNotificationFacts(f realtimeNotificationFacts) string {
	return fmt.Sprintf("%+v", f)
}
