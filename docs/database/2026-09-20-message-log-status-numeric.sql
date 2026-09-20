-- Forward-only migration: message mail/SMS log status strings -> SMALLINT enums.
-- 1 = pending, 2 = sent, 3 = failed.
BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
BEGIN
  IF to_regclass(current_schema() || '.message_mail_log') IS NULL
     OR to_regclass(current_schema() || '.message_sms_log') IS NULL THEN
    RAISE EXCEPTION 'message log tables are required';
  END IF;
END $$;

LOCK TABLE message_mail_log, message_sms_log IN SHARE ROW EXCLUSIVE MODE;

ALTER TABLE message_mail_log
  DROP CONSTRAINT IF EXISTS message_mail_log_status_check;
ALTER TABLE message_sms_log
  DROP CONSTRAINT IF EXISTS ck_message_sms_log_status;
ALTER TABLE message_mail_log
  DROP CONSTRAINT IF EXISTS ck_message_mail_log_scene;

DO $$
DECLARE
  mail_type text;
  sms_type text;
BEGIN
  SELECT data_type INTO mail_type
  FROM information_schema.columns
  WHERE table_schema = current_schema()
    AND table_name = 'message_mail_log'
    AND column_name = 'status';
  SELECT data_type INTO sms_type
  FROM information_schema.columns
  WHERE table_schema = current_schema()
    AND table_name = 'message_sms_log'
    AND column_name = 'status';

  IF mail_type IN ('character varying', 'text') THEN
    IF EXISTS (
      SELECT 1 FROM message_mail_log
      WHERE status NOT IN ('pending', 'sent', 'failed')
    ) THEN
      RAISE EXCEPTION 'message_mail_log contains an unknown status';
    END IF;
    ALTER TABLE message_mail_log
      ALTER COLUMN status TYPE SMALLINT USING CASE status
        WHEN 'pending' THEN 1
        WHEN 'sent' THEN 2
        WHEN 'failed' THEN 3
      END;
  ELSIF mail_type <> 'smallint' THEN
    RAISE EXCEPTION 'message_mail_log.status must be varchar/text or smallint, got %', mail_type;
  ELSIF EXISTS (
    SELECT 1 FROM message_mail_log WHERE status NOT IN (1, 2, 3)
  ) THEN
    RAISE EXCEPTION 'message_mail_log contains an unknown numeric status';
  END IF;

  IF sms_type IN ('character varying', 'text') THEN
    IF EXISTS (
      SELECT 1 FROM message_sms_log
      WHERE status NOT IN ('pending', 'sent', 'failed')
    ) THEN
      RAISE EXCEPTION 'message_sms_log contains an unknown status';
    END IF;
    ALTER TABLE message_sms_log
      ALTER COLUMN status TYPE SMALLINT USING CASE status
        WHEN 'pending' THEN 1
        WHEN 'sent' THEN 2
        WHEN 'failed' THEN 3
      END;
  ELSIF sms_type <> 'smallint' THEN
    RAISE EXCEPTION 'message_sms_log.status must be varchar/text or smallint, got %', sms_type;
  ELSIF EXISTS (
    SELECT 1 FROM message_sms_log WHERE status NOT IN (1, 2, 3)
  ) THEN
    RAISE EXCEPTION 'message_sms_log contains an unknown numeric status';
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM message_mail_log
    WHERE scene NOT IN ('login', 'forget', 'bind_email', 'change_password')
  ) THEN
    RAISE EXCEPTION 'message_mail_log contains an unknown scene';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'message_mail_log'::regclass
      AND conname = 'ck_message_mail_log_scene'
  ) THEN
    ALTER TABLE message_mail_log
      ADD CONSTRAINT ck_message_mail_log_scene
      CHECK (scene IN ('login', 'forget', 'bind_email', 'change_password'));
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'message_mail_log'::regclass
      AND conname = 'message_mail_log_status_check'
  ) THEN
    ALTER TABLE message_mail_log
      ADD CONSTRAINT message_mail_log_status_check CHECK (status IN (1, 2, 3));
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'message_sms_log'::regclass
      AND conname = 'ck_message_sms_log_status'
  ) THEN
    ALTER TABLE message_sms_log
      ADD CONSTRAINT ck_message_sms_log_status CHECK (status IN (1, 2, 3));
  END IF;
END $$;

COMMIT;
