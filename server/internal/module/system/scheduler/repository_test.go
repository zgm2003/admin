package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"admin/server/internal/config"
	"admin/server/internal/database/testschema"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestRepositoryGetJobAcceptsNullableLeaseFields(t *testing.T) {
	db, ctx := openSchedulerRepositoryDatabase(t)
	if err := db.WithContext(ctx).Exec(`
CREATE TABLE system_scheduler_job (
  id BIGINT PRIMARY KEY,
  schedule_id BIGINT NULL,
  task_type VARCHAR(128) NOT NULL,
  payload JSONB NOT NULL,
  trigger_source VARCHAR(16) NOT NULL,
  source_key VARCHAR(255) NULL,
  scheduled_at TIMESTAMPTZ NOT NULL,
  available_at TIMESTAMPTZ NOT NULL,
  status SMALLINT NOT NULL,
  attempt_count INTEGER NOT NULL,
  max_attempts INTEGER NOT NULL,
  queue VARCHAR(64) NOT NULL,
  timeout_seconds INTEGER NOT NULL,
  publish_token VARCHAR(64) NULL,
  publish_lease_until TIMESTAMPTZ NULL,
  run_token VARCHAR(64) NULL,
  run_lease_until TIMESTAMPTZ NULL,
  worker_id VARCHAR(128) NULL,
  retry_of_job_id BIGINT NULL,
  triggered_by BIGINT NULL,
  error_class VARCHAR(64) NOT NULL,
  last_error VARCHAR(1024) NOT NULL,
  completed_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`
INSERT INTO system_scheduler_job (
  id,task_type,payload,trigger_source,scheduled_at,available_at,status,attempt_count,max_attempts,queue,timeout_seconds,error_class,last_error,created_at,updated_at
) VALUES (1,'example','{}','manual',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,1,0,1,'default',30,'','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}

	job, err := NewRepository(db).GetJob(ctx, 1)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if job.ID != 1 || job.SourceKey != "" || job.PublishToken != "" || job.RunToken != "" || job.WorkerID != "" {
		t.Fatalf("job = %+v", job)
	}
}

func openSchedulerRepositoryDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test")
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", ".."))
	if err := godotenv.Load(filepath.Join(repoRoot, "server", ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return testschema.Open(t, settings.PostgresDSN, "test_scheduler_repository")
}
