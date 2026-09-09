-- Forward migration: make Mail rate-limit policy catalogs platform-scoped.
-- Drain old API/Worker writers before execution. This does not touch Mail
-- quota keys, sessions, access snapshots, or message configuration.
BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

LOCK TABLE message_mail_rate_limit_policy, permission_auth_platform IN ACCESS EXCLUSIVE MODE;

DO $$
DECLARE
  policy_count bigint;
  platform_count bigint;
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'message_mail_rate_limit_policy'
      AND column_name = 'platform_id'
  ) THEN
    SELECT count(*) INTO platform_count FROM permission_auth_platform WHERE deleted_at IS NULL;
    IF (SELECT count(*) FROM message_mail_rate_limit_policy) <> platform_count * 2 THEN
      RAISE EXCEPTION 'existing platform policy catalog has an unexpected row count';
    END IF;
    RETURN;
  END IF;

  SELECT count(*) INTO policy_count
  FROM message_mail_rate_limit_policy
  WHERE policy_key IN ('business_email_minute', 'business_email_10m');
  IF policy_count <> 2 OR EXISTS (
    SELECT 1 FROM message_mail_rate_limit_policy
    WHERE policy_key NOT IN ('business_email_minute', 'business_email_10m')
  ) THEN
    RAISE EXCEPTION 'expected exactly the two existing email policies before platform migration';
  END IF;

  SELECT count(*) INTO platform_count
  FROM permission_auth_platform
  WHERE deleted_at IS NULL;
  IF platform_count < 1 THEN
    RAISE EXCEPTION 'at least one enabled authentication platform is required';
  END IF;
END
$$;

CREATE TEMP TABLE mail_rate_policy_source ON COMMIT DROP AS
SELECT policy_key, mode, dimension, limit_count, window_seconds, revision, created_at, updated_at
FROM message_mail_rate_limit_policy;

ALTER TABLE message_mail_rate_limit_policy
  DROP CONSTRAINT IF EXISTS message_mail_rate_limit_policy_pkey;

ALTER TABLE message_mail_rate_limit_policy
  ADD COLUMN IF NOT EXISTS platform_id bigint;

INSERT INTO message_mail_rate_limit_policy
  (platform_id, policy_key, mode, dimension, limit_count, window_seconds, revision, created_at, updated_at)
SELECT platform.id, source.policy_key, source.mode, source.dimension, source.limit_count,
       source.window_seconds, source.revision, source.created_at, source.updated_at
FROM permission_auth_platform platform
CROSS JOIN mail_rate_policy_source source
WHERE platform.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM message_mail_rate_limit_policy existing
    WHERE existing.platform_id IS NOT NULL
  );

DELETE FROM message_mail_rate_limit_policy WHERE platform_id IS NULL;

ALTER TABLE message_mail_rate_limit_policy
  ALTER COLUMN platform_id SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'message_mail_rate_limit_policy_pkey' AND conrelid = 'message_mail_rate_limit_policy'::regclass) THEN
    ALTER TABLE message_mail_rate_limit_policy ADD CONSTRAINT message_mail_rate_limit_policy_pkey PRIMARY KEY (platform_id, policy_key);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'message_mail_rate_limit_policy_platform_id_fkey' AND conrelid = 'message_mail_rate_limit_policy'::regclass) THEN
    ALTER TABLE message_mail_rate_limit_policy ADD CONSTRAINT message_mail_rate_limit_policy_platform_id_fkey FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_message_mail_rate_limit_policy_platform' AND conrelid = 'message_mail_rate_limit_policy'::regclass) THEN
    ALTER TABLE message_mail_rate_limit_policy ADD CONSTRAINT ck_message_mail_rate_limit_policy_platform CHECK (platform_id > 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_message_mail_rate_limit_policy_shape' AND conrelid = 'message_mail_rate_limit_policy'::regclass) THEN
    ALTER TABLE message_mail_rate_limit_policy ADD CONSTRAINT ck_message_mail_rate_limit_policy_shape CHECK (
      policy_key IN ('business_email_minute', 'business_email_10m')
      AND mode = 'business'
      AND dimension = 'platform_email'
    );
  END IF;
END
$$;

CREATE INDEX IF NOT EXISTS ix_message_mail_rate_limit_policy_platform_revision
  ON message_mail_rate_limit_policy (platform_id, revision);

COMMIT;
