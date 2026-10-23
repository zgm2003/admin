package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"gorm.io/gorm"
)

func timezoneMigrationFixture(t *testing.T) (*gorm.DB, context.Context, string) {
	t.Helper()
	db, ctx := openSchedulerRepositoryDatabase(t)
	err := db.WithContext(ctx).Exec(`
CREATE TABLE system_scheduler_schedule (
 id BIGINT PRIMARY KEY, name TEXT NOT NULL, task_type TEXT NOT NULL,
 cron_expression TEXT NOT NULL, timezone TEXT NOT NULL DEFAULT 'Asia/Shanghai',
 params JSONB NOT NULL DEFAULT '{}', status SMALLINT NOT NULL DEFAULT 1,
 builtin_key TEXT, next_run_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT '2026-10-01T00:00:00Z',
 updated_at TIMESTAMPTZ NOT NULL DEFAULT '2026-10-01T00:00:00Z', deleted_at TIMESTAMPTZ
);
INSERT INTO system_scheduler_schedule(id,name,task_type,cron_expression,timezone,status,builtin_key,next_run_at) VALUES
 (1,'File scan','storage.file.scan','* * * * *','UTC',1,'storage.file.scan','2026-10-23T04:01:00Z'),
 (2,'Payment scan','payment.order.reconcile','* * * * *','UTC',0,'payment.order.reconcile',NULL),
 (3,'History','system.scheduler.history.cleanup','30 3 * * *','Asia/Shanghai',1,'system.scheduler.history.cleanup','2026-10-23T19:30:00Z'),
 (4,'Custom UTC','custom.task','0 0 * * *','UTC',1,NULL,'2026-10-24T00:00:00Z'),
 (5,'Deleted file scan','storage.file.scan','0 0 * * *','UTC',0,'storage.file.scan',NULL),
 (6,'Other builtin','other.task','* * * * *','UTC',1,'other.task','2026-10-23T04:01:00Z');
UPDATE system_scheduler_schedule SET deleted_at='2026-10-02T00:00:00Z' WHERE id=5;
`).Error
	if err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../docs/database/2026-10-23-scheduler-builtin-timezone.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return db, ctx, string(raw)
}

func timezoneMigrationSnapshot(t *testing.T, db *gorm.DB, ctx context.Context, normalize bool) string {
	t.Helper()
	query := `SELECT jsonb_agg(to_jsonb(s) ORDER BY id)::text FROM system_scheduler_schedule s`
	if normalize {
		query = `SELECT jsonb_agg(CASE WHEN id IN (1,2) THEN to_jsonb(s)-'timezone'-'updated_at' ELSE to_jsonb(s) END ORDER BY id)::text FROM system_scheduler_schedule s`
	}
	var result string
	if err := db.WithContext(ctx).Raw(query).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSchedulerTimezoneMigrationPreservesTimingAndReplays(t *testing.T) {
	db, ctx, sql := timezoneMigrationFixture(t)
	before := timezoneMigrationSnapshot(t, db, ctx, true)
	apply := func() error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error })
	}
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	var changed int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_scheduler_schedule WHERE id IN (1,2) AND timezone='Asia/Shanghai' AND updated_at > created_at`).Scan(&changed).Error; err != nil || changed != 2 {
		t.Fatalf("built-in timezone correction count=%d err=%v", changed, err)
	}
	if got := timezoneMigrationSnapshot(t, db, ctx, true); got != before {
		t.Fatal("migration changed timing, status, payload, or unrelated schedules")
	}
	after := timezoneMigrationSnapshot(t, db, ctx, false)
	if err := apply(); err != nil {
		t.Fatal("replay", err)
	}
	if got := timezoneMigrationSnapshot(t, db, ctx, false); got != after {
		t.Fatal("replay rewrote rows")
	}
}

func TestSchedulerTimezoneMigrationRejectsUnsafeChangesAtomically(t *testing.T) {
	for _, change := range []string{
		`UPDATE system_scheduler_schedule SET cron_expression='0 0 * * *' WHERE id=2`,
		`UPDATE system_scheduler_schedule SET task_type='other.task' WHERE id=2`,
	} {
		t.Run(change, func(t *testing.T) {
			db, ctx, sql := timezoneMigrationFixture(t)
			if err := db.WithContext(ctx).Exec(change).Error; err != nil {
				t.Fatal(err)
			}
			before := timezoneMigrationSnapshot(t, db, ctx, false)
			err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error })
			if err == nil {
				t.Fatal("unsafe timezone change accepted")
			}
			if got := timezoneMigrationSnapshot(t, db, ctx, false); got != before {
				t.Fatal("rejection left partial changes")
			}
		})
	}
}

func TestSchedulerTimezoneMigrationPreservesExplicitTimezone(t *testing.T) {
	db, ctx, sql := timezoneMigrationFixture(t)
	if err := db.WithContext(ctx).Exec(`UPDATE system_scheduler_schedule SET timezone='Asia/Tokyo',cron_expression='0 0 * * *' WHERE id=2`).Error; err != nil {
		t.Fatal(err)
	}
	var before string
	query := `SELECT row_to_json(s)::text FROM system_scheduler_schedule s WHERE id=2`
	if err := db.WithContext(ctx).Raw(query).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(sql).Error }); err != nil {
		t.Fatal(err)
	}
	var after string
	if err := db.WithContext(ctx).Raw(query).Scan(&after).Error; err != nil || before != after {
		t.Fatal("explicit administrator timezone changed", err)
	}
}

func TestSchedulerMinuteTimezoneCorrectionPreservesNextRun(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 10, 23, 15, 59, 59, 0, time.UTC),
		time.Date(2026, 12, 31, 23, 59, 0, 0, time.UTC),
	} {
		for _, timezone := range []string{"UTC", "Asia/Shanghai"} {
			got, err := nextRun("* * * * *", timezone, now)
			want := now.Truncate(time.Minute).Add(time.Minute)
			if err != nil || !got.Equal(want) {
				t.Fatalf("nextRun(%s,%s)=%s,%v want %s", timezone, now, got, err, want)
			}
		}
	}
}
