-- Forward-only migration. Review and execute with the API/Worker stopped.
BEGIN;

ALTER TABLE system_scheduler_job
  DROP CONSTRAINT IF EXISTS ck_system_scheduler_job_completed_at,
  DROP CONSTRAINT IF EXISTS ck_system_scheduler_job_publish_lease,
  DROP CONSTRAINT IF EXISTS ck_system_scheduler_job_status;
DROP INDEX IF EXISTS ix_system_scheduler_job_publish;
DROP INDEX IF EXISTS ix_system_scheduler_job_publish_lease;
DROP INDEX IF EXISTS ix_system_scheduler_job_run_lease;
DROP INDEX IF EXISTS ix_system_scheduler_job_terminal_cleanup;
DROP INDEX IF EXISTS ux_system_scheduler_job_active_schedule;

DO $$
BEGIN
  ALTER TABLE system_scheduler_job ALTER COLUMN status DROP DEFAULT;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='system_scheduler_job' AND column_name='status' AND data_type IN ('character varying','text')) THEN
    IF EXISTS (
      SELECT 1 FROM system_scheduler_job
      WHERE status NOT IN ('scheduled', 'queued', 'running', 'completed', 'failed', 'canceled')
    ) THEN
      RAISE EXCEPTION 'system_scheduler_job contains an unknown status';
    END IF;
    ALTER TABLE system_scheduler_job
      ALTER COLUMN status TYPE SMALLINT USING CASE status
        WHEN 'scheduled' THEN 1
        WHEN 'queued' THEN 2
        WHEN 'running' THEN 3
        WHEN 'completed' THEN 4
        WHEN 'failed' THEN 5
        WHEN 'canceled' THEN 6
        ELSE NULL
      END;
  END IF;
END $$;
ALTER TABLE system_scheduler_job ALTER COLUMN status SET DEFAULT 1;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_scheduler_job'::regclass AND conname = 'ck_system_scheduler_job_status') THEN
    ALTER TABLE system_scheduler_job ADD CONSTRAINT ck_system_scheduler_job_status CHECK (status BETWEEN 1 AND 6);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_scheduler_job'::regclass AND conname = 'ck_system_scheduler_job_completed_at') THEN
    ALTER TABLE system_scheduler_job ADD CONSTRAINT ck_system_scheduler_job_completed_at CHECK ((status IN (4,5,6)) = (completed_at IS NOT NULL));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_scheduler_job'::regclass AND conname = 'ck_system_scheduler_job_publish_lease') THEN
    ALTER TABLE system_scheduler_job ADD CONSTRAINT ck_system_scheduler_job_publish_lease CHECK (
      (status = 1 AND ((publish_lease_until IS NULL AND publish_token IS NULL) OR (publish_lease_until IS NOT NULL AND publish_token IS NOT NULL)))
      OR (status = 2 AND publish_lease_until IS NOT NULL AND publish_token IS NOT NULL)
      OR (status NOT IN (1,2) AND publish_lease_until IS NULL AND publish_token IS NULL)
    );
  END IF;
END $$;
CREATE INDEX IF NOT EXISTS ix_system_scheduler_job_publish ON system_scheduler_job (available_at, id) WHERE status = 1;
CREATE INDEX IF NOT EXISTS ix_system_scheduler_job_publish_lease ON system_scheduler_job (publish_lease_until, id) WHERE status = 1 AND publish_lease_until IS NOT NULL;
CREATE INDEX IF NOT EXISTS ix_system_scheduler_job_run_lease ON system_scheduler_job (run_lease_until, id) WHERE status = 3 AND run_lease_until IS NOT NULL;
CREATE INDEX IF NOT EXISTS ix_system_scheduler_job_terminal_cleanup ON system_scheduler_job (completed_at, id) WHERE status IN (4,5,6);
CREATE UNIQUE INDEX IF NOT EXISTS ux_system_scheduler_job_active_schedule ON system_scheduler_job (schedule_id) WHERE schedule_id IS NOT NULL AND status IN (1,2,3);

ALTER TABLE system_scheduler_run
  DROP CONSTRAINT IF EXISTS ck_system_scheduler_run_finished,
  DROP CONSTRAINT IF EXISTS ck_system_scheduler_run_status;
DROP INDEX IF EXISTS ix_system_scheduler_run_cleanup;

DO $$
BEGIN
  ALTER TABLE system_scheduler_run ALTER COLUMN status DROP DEFAULT;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='system_scheduler_run' AND column_name='status' AND data_type IN ('character varying','text')) THEN
    IF EXISTS (SELECT 1 FROM system_scheduler_run WHERE status NOT IN ('running', 'succeeded', 'failed')) THEN
      RAISE EXCEPTION 'system_scheduler_run contains an unknown status';
    END IF;
    ALTER TABLE system_scheduler_run
      ALTER COLUMN status TYPE SMALLINT USING CASE status
        WHEN 'running' THEN 1
        WHEN 'succeeded' THEN 2
        WHEN 'failed' THEN 3
        ELSE NULL
      END;
  END IF;
END $$;
ALTER TABLE system_scheduler_run ALTER COLUMN status SET DEFAULT 1;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_scheduler_run'::regclass AND conname = 'ck_system_scheduler_run_status') THEN
    ALTER TABLE system_scheduler_run ADD CONSTRAINT ck_system_scheduler_run_status CHECK (status BETWEEN 1 AND 3);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_scheduler_run'::regclass AND conname = 'ck_system_scheduler_run_finished') THEN
    ALTER TABLE system_scheduler_run ADD CONSTRAINT ck_system_scheduler_run_finished CHECK ((status = 1 AND finished_at IS NULL) OR (status <> 1 AND finished_at IS NOT NULL));
  END IF;
END $$;
CREATE INDEX IF NOT EXISTS ix_system_scheduler_run_cleanup ON system_scheduler_run (finished_at, id) WHERE status IN (2,3);

COMMIT;
