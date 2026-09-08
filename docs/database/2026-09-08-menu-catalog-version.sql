-- Offline forward migration. Menu revisions do not revoke authentication sessions.
BEGIN;
SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='30s';
ALTER TABLE permission_auth_platform ADD COLUMN IF NOT EXISTS menu_version BIGINT NOT NULL DEFAULT 1;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='permission_auth_platform'::regclass AND conname='ck_permission_auth_platform_menu_version') THEN
    ALTER TABLE permission_auth_platform ADD CONSTRAINT ck_permission_auth_platform_menu_version CHECK (menu_version >= 1);
  END IF;
END $$;
COMMIT;
