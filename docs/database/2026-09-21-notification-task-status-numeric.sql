-- Forward-only migration: notification task status strings -> SMALLINT enum.
-- 1=draft, 2=scheduled, 3=queued, 4=processing, 5=completed, 6=failed, 7=canceled.
BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
BEGIN
  IF to_regclass(current_schema() || '.message_notification_task') IS NULL THEN
    RAISE EXCEPTION 'message_notification_task table is required';
  END IF;
END $$;

LOCK TABLE message_notification_task IN SHARE ROW EXCLUSIVE MODE;

ALTER TABLE message_notification_task
  DROP CONSTRAINT IF EXISTS ck_message_notification_task_state,
  DROP CONSTRAINT IF EXISTS ck_message_notification_task_status;

DO $$
DECLARE
  status_type text;
BEGIN
  SELECT data_type INTO status_type
  FROM information_schema.columns
  WHERE table_schema = current_schema()
    AND table_name = 'message_notification_task'
    AND column_name = 'status';

  IF status_type IS NULL THEN
    RAISE EXCEPTION 'message_notification_task.status column is required';
  END IF;

  ALTER TABLE message_notification_task ALTER COLUMN status DROP DEFAULT;

  IF status_type IN ('character varying', 'text') THEN
    IF EXISTS (
      SELECT 1 FROM message_notification_task
      WHERE status NOT IN ('draft', 'scheduled', 'queued', 'processing', 'completed', 'failed', 'canceled')
    ) THEN
      RAISE EXCEPTION 'message_notification_task contains an unknown status';
    END IF;
    ALTER TABLE message_notification_task
      ALTER COLUMN status TYPE SMALLINT USING CASE status
        WHEN 'draft' THEN 1
        WHEN 'scheduled' THEN 2
        WHEN 'queued' THEN 3
        WHEN 'processing' THEN 4
        WHEN 'completed' THEN 5
        WHEN 'failed' THEN 6
        WHEN 'canceled' THEN 7
      END;
  ELSIF status_type = 'smallint' THEN
    IF EXISTS (SELECT 1 FROM message_notification_task WHERE status NOT BETWEEN 1 AND 7) THEN
      RAISE EXCEPTION 'message_notification_task contains an unknown numeric status';
    END IF;
  ELSE
    RAISE EXCEPTION 'message_notification_task.status must be varchar/text or smallint, got %', status_type;
  END IF;
END $$;

ALTER TABLE message_notification_task
  ALTER COLUMN status SET DEFAULT 1;

ALTER TABLE message_notification_task
  ADD CONSTRAINT ck_message_notification_task_status CHECK (status BETWEEN 1 AND 7),
  ADD CONSTRAINT ck_message_notification_task_state CHECK (
    (status = 1
      AND submitted_at IS NULL
      AND audience_max_user_id IS NULL
      AND published_at IS NULL
      AND completed_at IS NULL
      AND canceled_at IS NULL
      AND failed_at IS NULL
      AND failure_message IS NULL)
    OR
    (status <> 1
      AND submitted_at IS NOT NULL
      AND audience_max_user_id IS NOT NULL
      AND audience_max_user_id >= 0
      AND (status <> 2 OR scheduled_at IS NOT NULL)
      AND (status <> 5 OR completed_at IS NOT NULL)
      AND (status <> 7 OR canceled_at IS NOT NULL)
      AND (status <> 6 OR (failed_at IS NOT NULL AND failure_message IS NOT NULL AND btrim(failure_message) <> '')))
  );

CREATE INDEX IF NOT EXISTS ix_message_notification_task_due
  ON message_notification_task (status, scheduled_at, id)
  WHERE deleted_at IS NULL;

COMMIT;
