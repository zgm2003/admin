BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

-- user_session.user_id had two identical foreign keys left by an earlier migration.
ALTER TABLE user_session DROP CONSTRAINT IF EXISTS fk_auth_session_user;

-- Empty device identifiers cannot participate in device-bound session policy.
DELETE FROM user_session
WHERE device_id IS NOT NULL AND btrim(device_id) = '';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint c
    JOIN pg_class r ON r.oid = c.conrelid
    JOIN pg_namespace n ON n.oid = r.relnamespace
    WHERE n.nspname = current_schema()
      AND r.relname = 'user_session'
      AND c.conname = 'ck_user_session_device_id_nonempty'
  ) THEN
    ALTER TABLE user_session
      ADD CONSTRAINT ck_user_session_device_id_nonempty
      CHECK (btrim(device_id) <> '');
  END IF;
END $$;

-- Access versions are keyed by the real user id; they are not an identity column.
ALTER TABLE permission_access_version
  ALTER COLUMN user_id DROP DEFAULT;

DO $$
BEGIN
  IF to_regclass(current_schema() || '.rbac_access_version_user_id_seq') IS NOT NULL THEN
    ALTER SEQUENCE rbac_access_version_user_id_seq OWNED BY NONE;
    DROP SEQUENCE rbac_access_version_user_id_seq;
  END IF;
END $$;

COMMIT;
