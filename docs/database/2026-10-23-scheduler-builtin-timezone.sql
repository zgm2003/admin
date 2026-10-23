-- Run after the file-scan and payment-reconcile seeds, using:
-- psql -X --set ON_ERROR_STOP=1 --single-transaction --file <this file>
-- These legacy seeds explicitly used UTC although the schedule default is
-- Asia/Shanghai. Every-minute schedules have identical future firing instants
-- in these two zones. Keep next_run_at, job/run history and user choices intact.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $migration$
BEGIN
    PERFORM pg_advisory_xact_lock(
        hashtextextended('admin.scheduler.builtin-timezone.2026-10-23', 0)
    );

    PERFORM id
    FROM system_scheduler_schedule
    WHERE deleted_at IS NULL
      AND builtin_key IN ('storage.file.scan', 'payment.order.reconcile')
    ORDER BY id
    FOR UPDATE;

    IF EXISTS (
        SELECT 1 FROM system_scheduler_schedule
        WHERE deleted_at IS NULL
          AND builtin_key IN ('storage.file.scan', 'payment.order.reconcile')
          AND task_type <> builtin_key
    ) THEN
        RAISE EXCEPTION 'Built-in schedule task type conflict; no timezone changes applied';
    END IF;

    IF EXISTS (
        SELECT 1 FROM system_scheduler_schedule
        WHERE deleted_at IS NULL
          AND builtin_key IN ('storage.file.scan', 'payment.order.reconcile')
          AND timezone = 'UTC'
          AND cron_expression <> '* * * * *'
    ) THEN
        RAISE EXCEPTION 'UTC built-in schedule has a custom cron; review firing times before migration';
    END IF;

    UPDATE system_scheduler_schedule
    SET timezone = 'Asia/Shanghai', updated_at = CURRENT_TIMESTAMP
    WHERE deleted_at IS NULL
      AND builtin_key IN ('storage.file.scan', 'payment.order.reconcile')
      AND task_type = builtin_key
      AND cron_expression = '* * * * *'
      AND timezone = 'UTC';
END
$migration$;
