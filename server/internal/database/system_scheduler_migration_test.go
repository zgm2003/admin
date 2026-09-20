package database_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gorm.io/gorm"
)

type schedulerMigrationFacts struct {
	Tables           int64
	Schedules        int64
	Setting          string
	Generation       int64
	Outbox           int64
	Menus            int64
	Grants           int64
	AdminMenuVersion int64
	AccessVersions   string
	OldOutbox        int64
}

func TestSystemSchedulerMigrationIsIdempotent(t *testing.T) {
	db, ctx := openRealtimeNotificationMigrationSchema(t)
	seedRealtimeNotificationPrerequisites(t, db, ctx)
	if err := db.WithContext(ctx).Exec(readRealtimeNotificationMigration(t)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,is_enabled,is_hidden,created_at,updated_at) VALUES(1,NULL,'directory','系统管理','system','navigation.system','/system',NULL,'lucide:settings',500,1,0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	before := readSchedulerMigrationFacts(t, db)
	script := readSchedulerMigration(t)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("execute scheduler migration: %v", err)
	}
	first := readSchedulerMigrationFacts(t, db)
	if first.Tables != 3 || first.Schedules != 3 || first.Setting != "system.scheduler.history_retention_days=30:2:1:1" || first.OldOutbox != 0 {
		t.Fatalf("first facts=%+v", first)
	}
	if first.Generation != before.Generation+1 || first.Outbox != before.Outbox+1 || first.Menus != before.Menus+9 || first.Grants != before.Grants+18 || first.AdminMenuVersion != before.AdminMenuVersion+1 {
		t.Fatalf("unexpected scheduler deltas: before=%+v after=%+v", before, first)
	}
	assertSchedulerStructure(t, db)
	if err := db.WithContext(ctx).Exec(script).Error; err != nil {
		t.Fatalf("rerun scheduler migration: %v", err)
	}
	second := readSchedulerMigrationFacts(t, db)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("migration is not idempotent:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestSystemSchedulerMigrationRejectsPartialSchema(t *testing.T) {
	db, ctx := openRealtimeNotificationMigrationSchema(t)
	seedRealtimeNotificationPrerequisites(t, db, ctx)
	if err := db.WithContext(ctx).Exec(readRealtimeNotificationMigration(t)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`CREATE TABLE system_scheduler_schedule(id BIGINT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	err := db.WithContext(ctx).Exec(readSchedulerMigration(t)).Error
	if err == nil || !strings.Contains(err.Error(), "entirely absent or complete") {
		t.Fatalf("error=%v", err)
	}
}

func assertSchedulerStructure(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, name := range []string{"ux_system_scheduler_schedule_name_active", "ux_system_scheduler_schedule_builtin_active", "ux_system_scheduler_job_active_schedule", "ux_system_scheduler_job_source_key", "ux_system_scheduler_run_job_attempt", "ix_system_scheduler_schedule_due", "ix_system_scheduler_job_publish", "ix_system_scheduler_job_run_lease", "ix_system_scheduler_job_terminal_cleanup"} {
		var exists bool
		if err := db.Raw(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname=?)`, name).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("index %s is missing", name)
		}
	}
}
func readSchedulerMigration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "database", "2026-09-19-system-scheduler.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func readSchedulerMigrationFacts(t *testing.T, db *gorm.DB) schedulerMigrationFacts {
	t.Helper()
	var facts schedulerMigrationFacts
	queries := []struct {
		sql    string
		target any
	}{
		{`SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ('system_scheduler_schedule','system_scheduler_job','system_scheduler_run')`, &facts.Tables},
		{`SELECT COALESCE(string_agg(setting_key||'='||value||':'||value_type||':'||is_enabled||':'||is_builtin,','),'') FROM system_setting WHERE deleted_at IS NULL AND setting_key='system.scheduler.history_retention_days'`, &facts.Setting},
		{`SELECT generation FROM system_config_cache_generation WHERE namespace='system.setting' AND scope_key='global'`, &facts.Generation},
		{`SELECT count(*) FROM system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global'`, &facts.Outbox},
		{`SELECT count(*) FROM permission_menu WHERE platform_id=1 AND deleted_at IS NULL AND code LIKE 'system:scheduler:%'`, &facts.Menus},
		{`SELECT count(*) FROM permission_role_menu rm JOIN permission_menu m ON m.id=rm.menu_id WHERE rm.deleted_at IS NULL AND m.code LIKE 'system:scheduler:%'`, &facts.Grants},
		{`SELECT menu_version FROM permission_auth_platform WHERE id=1`, &facts.AdminMenuVersion},
		{`SELECT COALESCE(string_agg(user_id||':'||version,',' ORDER BY user_id),'') FROM permission_access_version`, &facts.AccessVersions},
		{`SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='message_notification_dispatch_outbox'`, &facts.OldOutbox},
	}
	for _, query := range queries {
		if err := db.Raw(query.sql).Scan(query.target).Error; err != nil {
			t.Fatal(err)
		}
	}
	if facts.Tables == 3 {
		if err := db.Raw(`SELECT count(*) FROM system_scheduler_schedule WHERE deleted_at IS NULL`).Scan(&facts.Schedules).Error; err != nil {
			t.Fatal(err)
		}
	}
	return facts
}
