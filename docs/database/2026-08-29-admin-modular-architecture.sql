BEGIN;

-- Validate every known conflict before changing a relation.
DO $$
DECLARE n BIGINT;
BEGIN
  IF to_regclass(current_schema() || '.auth_session') IS NOT NULL
     AND to_regclass(current_schema() || '.user_session') IS NOT NULL THEN
    RAISE EXCEPTION 'both auth_session and user_session exist';
  END IF;
  IF to_regclass(current_schema() || '.user_account') IS NOT NULL THEN
    SELECT count(*) INTO n FROM (SELECT lower(username) FROM user_account WHERE deleted_at IS NULL GROUP BY lower(username) HAVING count(*) > 1) d;
    IF n > 0 THEN RAISE EXCEPTION 'duplicate active username'; END IF;
    SELECT count(*) INTO n FROM (SELECT lower(email) FROM user_account WHERE deleted_at IS NULL GROUP BY lower(email) HAVING count(*) > 1) d;
    IF n > 0 THEN RAISE EXCEPTION 'duplicate active email'; END IF;
    SELECT count(*) INTO n FROM (SELECT phone FROM user_account WHERE deleted_at IS NULL AND phone IS NOT NULL AND phone <> '' GROUP BY phone HAVING count(*) > 1) d;
    IF n > 0 THEN RAISE EXCEPTION 'duplicate active phone'; END IF;
  END IF;
  IF to_regclass(current_schema() || '.auth_platform') IS NOT NULL AND to_regclass(current_schema() || '.auth_session') IS NOT NULL
     AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='auth_session' AND column_name='platform') THEN
    SELECT count(*) INTO n FROM auth_session s LEFT JOIN auth_platform p ON p.code = s.platform AND p.deleted_at IS NULL WHERE s.platform IS NULL OR btrim(s.platform) = '' OR p.id IS NULL;
    IF n > 0 THEN RAISE EXCEPTION 'unknown or empty auth_session platform'; END IF;
  END IF;
  IF to_regclass(current_schema() || '.auth_platform') IS NOT NULL AND to_regclass(current_schema() || '.audit_operation_log') IS NOT NULL
     AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='audit_operation_log' AND column_name='platform') THEN
    SELECT count(*) INTO n FROM audit_operation_log l LEFT JOIN auth_platform p ON p.code = l.platform AND p.deleted_at IS NULL WHERE l.platform IS NOT NULL AND btrim(l.platform) <> '' AND p.id IS NULL;
    IF n > 0 THEN RAISE EXCEPTION 'unknown audit_operation_log platform'; END IF;
  END IF;
  IF to_regclass(current_schema() || '.rbac_role') IS NOT NULL THEN
    SELECT count(*) INTO n FROM (SELECT 1 FROM rbac_role WHERE is_default = 1 AND deleted_at IS NULL GROUP BY 1 HAVING count(*) > 1) d;
    IF n > 0 THEN RAISE EXCEPTION 'duplicate active default role'; END IF;
  END IF;
  IF to_regclass(current_schema() || '.foundation_task') IS NOT NULL THEN
    SELECT count(*) INTO n
      FROM pg_constraint c
      WHERE c.confrelid = to_regclass(current_schema() || '.foundation_task')
        AND c.conrelid <> c.confrelid;
    IF n > 0 THEN RAISE EXCEPTION 'foundation_task has non-demo foreign key references'; END IF;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS user_profile (
  user_id BIGINT PRIMARY KEY,
  birthday DATE NULL,
  gender SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_user_profile_account FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT,
  CONSTRAINT ck_user_profile_gender CHECK (gender IN (0, 1, 2))
);

DO $$
DECLARE missing BIGINT;
BEGIN
  IF to_regclass(current_schema() || '.user_profile') IS NOT NULL THEN
    SELECT count(*) INTO missing
    FROM (VALUES
      ('user_id', 'bigint', 'NO'),
      ('birthday', 'date', 'YES'),
      ('gender', 'smallint', 'NO'),
      ('created_at', 'timestamp with time zone', 'NO'),
      ('updated_at', 'timestamp with time zone', 'NO')) expected(column_name, data_type, is_nullable)
    WHERE NOT EXISTS (
      SELECT 1 FROM information_schema.columns c
      WHERE c.table_schema = current_schema()
        AND c.table_name = 'user_profile'
        AND c.column_name = expected.column_name
        AND c.data_type = expected.data_type
        AND c.is_nullable = expected.is_nullable
    );
    IF missing > 0 THEN RAISE EXCEPTION 'user_profile structure mismatch'; END IF;
    IF NOT EXISTS (
      SELECT 1 FROM pg_constraint c
      WHERE c.conrelid = to_regclass(current_schema() || '.user_profile')
        AND c.conname = 'ck_user_profile_gender'
    ) THEN RAISE EXCEPTION 'user_profile gender constraint missing'; END IF;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_user_profile_account' AND conrelid = to_regclass(current_schema() || '.user_profile')) THEN
    ALTER TABLE user_profile ADD CONSTRAINT fk_user_profile_account FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$
BEGIN
  IF to_regclass(current_schema() || '.auth_session') IS NOT NULL THEN
    ALTER TABLE auth_session RENAME TO user_session;
  END IF;
END $$;

DO $$
BEGIN
  IF to_regclass(current_schema() || '.user_session') IS NULL THEN
    CREATE TABLE user_session (
      id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, platform_id BIGINT NOT NULL, device_id VARCHAR(36) NOT NULL,
      refresh_token_hash CHAR(64) NOT NULL, version BIGINT NOT NULL DEFAULT 1, client_ip VARCHAR(64) NOT NULL,
      user_agent VARCHAR(512) NOT NULL, refresh_expires_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ NULL,
      created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
  END IF;
END $$;

ALTER TABLE user_session ADD COLUMN IF NOT EXISTS platform_id BIGINT;
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_session' AND column_name='platform') THEN
    UPDATE user_session s SET platform_id = p.id FROM auth_platform p WHERE p.code = s.platform AND p.deleted_at IS NULL;
    IF EXISTS (SELECT 1 FROM user_session WHERE platform_id IS NULL) THEN RAISE EXCEPTION 'user_session platform mapping incomplete'; END IF;
    ALTER TABLE user_session DROP COLUMN platform;
  END IF;
  ALTER TABLE user_session ALTER COLUMN platform_id SET NOT NULL;
END $$;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_user_session_user' AND conrelid=to_regclass(current_schema() || '.user_session')) THEN
    ALTER TABLE user_session ADD CONSTRAINT fk_user_session_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_user_session_platform' AND conrelid=to_regclass(current_schema() || '.user_session')) THEN
    ALTER TABLE user_session ADD CONSTRAINT fk_user_session_platform FOREIGN KEY (platform_id) REFERENCES auth_platform(id) ON DELETE RESTRICT;
  END IF;
END $$;
DROP INDEX IF EXISTS ux_auth_session_current;
DROP INDEX IF EXISTS ux_auth_session_refresh_hash;
DROP INDEX IF EXISTS ix_auth_session_user_created;
DROP INDEX IF EXISTS ix_auth_session_user_platform_active;

CREATE TABLE IF NOT EXISTS user_login_log (
  id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  user_id BIGINT NULL,
  session_id BIGINT NULL,
  platform_id BIGINT NOT NULL,
  login_account VARCHAR(254) NOT NULL,
  event_type VARCHAR(16) NOT NULL,
  login_type VARCHAR(32) NULL,
  is_success SMALLINT NOT NULL,
  reason_code VARCHAR(64) NOT NULL,
  client_ip VARCHAR(64) NOT NULL,
  user_agent VARCHAR(512) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT ck_user_login_log_event_type CHECK (event_type IN ('login', 'logout')),
  CONSTRAINT ck_user_login_log_login_type CHECK ((event_type = 'login' AND login_type IS NOT NULL) OR (event_type = 'logout' AND login_type IS NULL)),
  CONSTRAINT ck_user_login_log_is_success CHECK (is_success IN (0, 1)),
  CONSTRAINT fk_user_login_log_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT,
  CONSTRAINT fk_user_login_log_session FOREIGN KEY (session_id) REFERENCES user_session(id) ON DELETE RESTRICT,
  CONSTRAINT fk_user_login_log_platform FOREIGN KEY (platform_id) REFERENCES auth_platform(id) ON DELETE RESTRICT
);

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'user_login_log'
      AND column_name = 'id'
      AND is_identity = 'NO'
      AND column_default IS NULL
  ) THEN
    ALTER TABLE user_login_log ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY;
  END IF;
END $$;

ALTER TABLE audit_operation_log ADD COLUMN IF NOT EXISTS platform_id BIGINT;
DROP INDEX IF EXISTS ux_audit_operation_log_request_id;
DROP INDEX IF EXISTS ix_audit_operation_log_user_created;
DROP INDEX IF EXISTS ix_audit_operation_log_action_created;
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='audit_operation_log' AND column_name='platform') THEN
    UPDATE audit_operation_log l SET platform_id = p.id FROM auth_platform p WHERE l.platform = p.code AND p.deleted_at IS NULL;
    ALTER TABLE audit_operation_log DROP COLUMN platform;
  END IF;
END $$;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_audit_operation_log_platform' AND conrelid=to_regclass(current_schema() || '.audit_operation_log')) THEN
    ALTER TABLE audit_operation_log ADD CONSTRAINT fk_audit_operation_log_platform FOREIGN KEY (platform_id) REFERENCES auth_platform(id) ON DELETE RESTRICT;
  END IF;
END $$;

INSERT INTO rbac_access_version (user_id, version, created_at, updated_at)
SELECT id, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP FROM user_account u
WHERE NOT EXISTS (SELECT 1 FROM rbac_access_version v WHERE v.user_id = u.id);
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_rbac_access_version_user' AND conrelid=to_regclass(current_schema() || '.rbac_access_version')) THEN
    ALTER TABLE rbac_access_version ADD CONSTRAINT fk_rbac_access_version_user FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE RESTRICT;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_rbac_access_version_version' AND conrelid=to_regclass(current_schema() || '.rbac_access_version')) THEN
    ALTER TABLE rbac_access_version ADD CONSTRAINT ck_rbac_access_version_version CHECK (version >= 1);
  END IF;
END $$;

DROP TABLE IF EXISTS foundation_task;

CREATE UNIQUE INDEX IF NOT EXISTS ux_user_account_username_active ON user_account (lower(username)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_account_email_active ON user_account (lower(email)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_account_phone_active ON user_account (phone) WHERE phone IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_user_session_refresh_token_hash ON user_session (refresh_token_hash);
CREATE INDEX IF NOT EXISTS ix_user_session_user_created_at ON user_session (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_user_session_user_platform_created_at ON user_session (user_id, platform_id, created_at DESC, id DESC) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_user_login_log_created_at ON user_login_log (created_at DESC);
CREATE INDEX IF NOT EXISTS ix_user_login_log_user_created_at ON user_login_log (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_user_login_log_platform_created_at ON user_login_log (platform_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_user_login_log_account_created_at ON user_login_log (login_account, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS ux_auth_platform_code_active ON auth_platform (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_role_code_active ON rbac_role (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_role_name_active ON rbac_role (name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_role_default_active ON rbac_role (is_default) WHERE is_default = 1 AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_user_role_active ON rbac_user_role (user_id, role_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_menu_platform_code_active ON rbac_menu (platform_id, code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_menu_platform_path_active ON rbac_menu (platform_id, path) WHERE path IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_rbac_menu_platform_parent_sort ON rbac_menu (platform_id, parent_id, sort_order, id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_rbac_role_menu_active ON rbac_role_menu (role_id, menu_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_audit_operation_log_event_id ON audit_operation_log (event_id);
CREATE INDEX IF NOT EXISTS ix_audit_operation_log_request_id ON audit_operation_log (request_id);
CREATE INDEX IF NOT EXISTS ix_audit_operation_log_created_at ON audit_operation_log (created_at DESC);
CREATE INDEX IF NOT EXISTS ix_audit_operation_log_user_created_at ON audit_operation_log (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_audit_operation_log_action_created_at ON audit_operation_log (action, created_at DESC);

COMMIT;
