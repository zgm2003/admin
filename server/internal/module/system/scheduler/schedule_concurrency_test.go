package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

// A concurrent writer holds the schedule row until the administration request
// is observably blocked in PostgreSQL. This reproduces the stale-read window
// without relying on a sleep to order the two writers.
func TestScheduleMutationReadsAfterConcurrentWriter(t *testing.T) {
	for _, operation := range []string{"delete", "enable"} {
		t.Run(operation, func(t *testing.T) {
			db, ctx := openSchedulerRepositoryDatabase(t)
			if err := db.WithContext(ctx).Exec(`
CREATE TABLE system_scheduler_schedule (
 id BIGINT PRIMARY KEY, builtin_key TEXT, cron_expression TEXT NOT NULL,
 timezone TEXT NOT NULL, is_enabled SMALLINT NOT NULL, next_run_at TIMESTAMPTZ,
 updated_by BIGINT, updated_at TIMESTAMPTZ NOT NULL, deleted_at TIMESTAMPTZ
);
CREATE TABLE system_scheduler_job (id BIGINT PRIMARY KEY, schedule_id BIGINT NOT NULL, status SMALLINT NOT NULL);
INSERT INTO system_scheduler_schedule VALUES (1,NULL,'0 * * * *','UTC',0,NULL,NULL,NOW(),NULL);
`).Error; err != nil {
				t.Fatal(err)
			}
			tx := db.WithContext(ctx).Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer tx.Rollback()
			var pid int
			if err := tx.Raw("SELECT pg_backend_pid()").Scan(&pid).Error; err != nil {
				t.Fatal(err)
			}
			if err := tx.Exec("UPDATE system_scheduler_schedule SET cron_expression='*/5 * * * *' WHERE id=1").Error; err != nil {
				t.Fatal(err)
			}
			if operation == "delete" {
				if err := tx.Exec("INSERT INTO system_scheduler_job VALUES (1,1,1)").Error; err != nil {
					t.Fatal(err)
				}
			}
			now := time.Date(2026, 9, 20, 12, 1, 0, 0, time.UTC)
			done := make(chan error, 1)
			go func() {
				repo := NewRepository(db)
				if operation == "delete" {
					done <- repo.DeleteSchedule(ctx, 1, now)
				} else {
					done <- repo.SetScheduleEnabled(ctx, 1, true, nil, now)
				}
			}()
			waitScheduleWriterBlocked(t, ctx, db, pid)
			if err := tx.Commit().Error; err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if operation == "delete" {
					if !errors.Is(err, ErrActiveJobExists) {
						t.Fatalf("delete must reject newly committed active job, got %v", err)
					}
					var count int64
					if err := db.Table("system_scheduler_schedule").Where("id=1 AND deleted_at IS NULL").Count(&count).Error; err != nil {
						t.Fatal(err)
					}
					if count != 1 {
						t.Fatal("schedule with active job was deleted")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					var row struct{ NextRunAt time.Time }
					if err := db.Table("system_scheduler_schedule").Select("next_run_at").Where("id=1").Take(&row).Error; err != nil {
						t.Fatal(err)
					}
					expected := time.Date(2026, 9, 20, 12, 5, 0, 0, time.UTC)
					if !row.NextRunAt.Equal(expected) {
						t.Fatalf("next run uses stale cron: got %s, want %s", row.NextRunAt, expected)
					}
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}

func waitScheduleWriterBlocked(t *testing.T, ctx context.Context, db *gorm.DB, pid int) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var blocked bool
		if err := db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid)))", pid).Scan(&blocked).Error; err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-deadline.C:
			t.Fatal("schedule writer did not reach the row lock")
		case <-tick.C:
		}
	}
}
