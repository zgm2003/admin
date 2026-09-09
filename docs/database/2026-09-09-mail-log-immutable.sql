-- Forward migration: make Mail delivery logs append-only and retire the
-- administrator delete action. Drain API/Worker writers before execution.
BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

LOCK TABLE permission_auth_platform, permission_menu,
  message_mail_log, message_mail_log_verification IN ACCESS EXCLUSIVE MODE;

DO $$
DECLARE
  has_deleted_rows boolean;
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'message_mail_log'
      AND column_name = 'deleted_at'
  ) THEN
    EXECUTE 'SELECT EXISTS (SELECT 1 FROM message_mail_log WHERE deleted_at IS NOT NULL)'
      INTO has_deleted_rows;
    IF has_deleted_rows THEN
      RAISE EXCEPTION 'message_mail_log contains soft-deleted audit records';
    END IF;
  END IF;

  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'message_mail_log_verification'
      AND column_name = 'deleted_at'
  ) THEN
    EXECUTE 'SELECT EXISTS (SELECT 1 FROM message_mail_log_verification WHERE deleted_at IS NOT NULL)'
      INTO has_deleted_rows;
    IF has_deleted_rows THEN
      RAISE EXCEPTION 'message_mail_log_verification contains soft-deleted audit records';
    END IF;
  END IF;
END
$$;

DROP INDEX IF EXISTS ux_message_mail_log_platform_challenge_active;
DROP INDEX IF EXISTS ux_message_mail_verification_log_active;

ALTER TABLE message_mail_log DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE message_mail_log_verification DROP COLUMN IF EXISTS deleted_at;

CREATE UNIQUE INDEX IF NOT EXISTS ux_message_mail_log_platform_challenge
  ON message_mail_log (platform_id, challenge_id)
  WHERE challenge_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_message_mail_verification_log
  ON message_mail_log_verification (mail_log_id);

CREATE INDEX IF NOT EXISTS ix_message_mail_log_platform_id_desc
  ON message_mail_log (platform_id, id DESC);

CREATE INDEX IF NOT EXISTS ix_message_mail_log_to_email_prefix
  ON message_mail_log (to_email varchar_pattern_ops, id DESC);

CREATE INDEX IF NOT EXISTS ix_message_mail_log_scene_id_desc
  ON message_mail_log (scene, id DESC);

CREATE INDEX IF NOT EXISTS ix_message_mail_log_status_id_desc
  ON message_mail_log (status, id DESC);

CREATE INDEX IF NOT EXISTS ix_message_mail_log_created_id_desc
  ON message_mail_log (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_permission_auth_platform_code_history_prefix
  ON permission_auth_platform (code varchar_pattern_ops, id);

CREATE TEMP TABLE affected_mail_log_delete_platform (
  platform_id bigint PRIMARY KEY
) ON COMMIT DROP;

INSERT INTO affected_mail_log_delete_platform (platform_id)
SELECT DISTINCT platform_id
FROM permission_menu
WHERE code = 'message:mail:log:delete'
  AND deleted_at IS NULL;

UPDATE permission_menu
SET is_enabled = 0,
    deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE code = 'message:mail:log:delete'
  AND deleted_at IS NULL;

UPDATE permission_auth_platform platform
SET menu_version = platform.menu_version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE platform.id IN (SELECT platform_id FROM affected_mail_log_delete_platform);

COMMIT;
