-- Maintenance-only forward migration. Stop API/Worker and back up first.
-- Execute with psql --single-transaction -v ON_ERROR_STOP=1.
-- No schema qualification: maintenance runner uses public; tests use an isolated schema.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';
LOCK TABLE system_scheduler_schedule, system_scheduler_job, system_scheduler_run IN ACCESS EXCLUSIVE MODE;

DO $preflight$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='system_scheduler_run' AND column_name='result_summary') THEN
        IF EXISTS (SELECT 1 FROM system_scheduler_run WHERE result_summary <> '') THEN
            RAISE EXCEPTION 'scheduler cleanup refuses to discard nonempty execution results';
        END IF;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_index i JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=i.indkey[0]
        WHERE i.indrelid='system_scheduler_job'::regclass AND i.indisprimary AND i.indnkeyatts=1 AND a.attname='id'
    ) THEN
        RAISE EXCEPTION 'scheduler job id primary key is required';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid
        WHERE i.indrelid='system_scheduler_run'::regclass AND c.relname='ux_system_scheduler_run_job_attempt'
        AND i.indisunique AND i.indisvalid AND i.indpred IS NULL
        AND pg_get_indexdef(i.indexrelid,1,true)='job_id' AND pg_get_indexdef(i.indexrelid,2,true)='attempt_no' AND i.indnkeyatts=2
    ) THEN
        RAISE EXCEPTION 'scheduler run unique job/attempt index is required';
    END IF;
END
$preflight$;

ALTER TABLE system_scheduler_job
    DROP CONSTRAINT IF EXISTS ck_system_scheduler_job_counters,
    DROP COLUMN IF EXISTS queue,
    DROP COLUMN IF EXISTS timeout_seconds,
    DROP COLUMN IF EXISTS worker_id;
ALTER TABLE system_scheduler_job ADD CONSTRAINT ck_system_scheduler_job_counters
    CHECK (attempt_count >= 0 AND max_attempts > 0);
ALTER TABLE system_scheduler_run DROP COLUMN IF EXISTS result_summary;

DROP INDEX IF EXISTS ux_system_scheduler_job_task_attempt;
DROP INDEX IF EXISTS ix_system_scheduler_run_job_list;
DROP INDEX IF EXISTS ix_system_scheduler_run_cleanup;
CREATE INDEX IF NOT EXISTS ix_system_scheduler_job_queued_lease
    ON system_scheduler_job (publish_lease_until, id)
    WHERE status=2 AND publish_lease_until IS NOT NULL;

DO $postflight$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND
        ((table_name='system_scheduler_job' AND column_name IN ('queue','timeout_seconds','worker_id')) OR
        (table_name='system_scheduler_run' AND column_name='result_summary'))) THEN
        RAISE EXCEPTION 'scheduler legacy columns remain';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid
        WHERE i.indrelid='system_scheduler_job'::regclass AND c.relname='ix_system_scheduler_job_queued_lease'
        AND i.indisvalid AND NOT i.indisunique AND i.indnkeyatts=2
        AND pg_get_indexdef(i.indexrelid,1,true)='publish_lease_until' AND pg_get_indexdef(i.indexrelid,2,true)='id'
        AND pg_get_expr(i.indpred,i.indrelid)='((status = 2) AND (publish_lease_until IS NOT NULL))'
    ) THEN
        RAISE EXCEPTION 'scheduler queued recovery index is incompatible';
    END IF;
END
$postflight$;
