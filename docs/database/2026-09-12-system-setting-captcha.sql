BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS system_setting (
  id BIGSERIAL PRIMARY KEY,
  setting_key VARCHAR(128) NOT NULL,
  value TEXT NOT NULL,
  value_type SMALLINT NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  is_enabled SMALLINT NOT NULL DEFAULT 1,
  is_builtin SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMPTZ,
  CONSTRAINT ck_system_setting_key CHECK (setting_key ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$'),
  CONSTRAINT ck_system_setting_value_type CHECK (value_type IN (1, 2, 3, 4)),
  CONSTRAINT ck_system_setting_is_enabled CHECK (is_enabled IN (0, 1)),
  CONSTRAINT ck_system_setting_is_builtin CHECK (is_builtin IN (0, 1))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_system_setting_key_active
  ON system_setting (setting_key) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_system_setting_enabled_key
  ON system_setting (is_enabled, setting_key) WHERE deleted_at IS NULL;

INSERT INTO system_setting (setting_key, value, value_type, description, is_enabled, is_builtin, created_at, updated_at)
SELECT 'auth.captcha.ttl_minutes', '2', 2, 'Captcha challenge lifetime in minutes', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'auth.captcha.ttl_minutes' AND deleted_at IS NULL);

INSERT INTO system_setting (setting_key, value, value_type, description, is_enabled, is_builtin, created_at, updated_at)
SELECT 'auth.captcha.slide_padding', '10', 2, 'Captcha slide tolerance in pixels', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'auth.captcha.slide_padding' AND deleted_at IS NULL);

LOCK TABLE permission_auth_platform, permission_menu IN ACCESS EXCLUSIVE MODE;

DO $$
DECLARE
  platform_row RECORD;
  v_system_parent_id BIGINT;
  page_id BIGINT;
  next_sort INTEGER;
  action_code TEXT;
  changed BOOLEAN;
BEGIN
  FOR platform_row IN
    SELECT id FROM permission_auth_platform
    WHERE deleted_at IS NULL AND is_enabled = 1
  LOOP
    SELECT m.id INTO v_system_parent_id
    FROM permission_menu AS m
    WHERE m.platform_id = platform_row.id
      AND m.menu_type = 'directory'
      AND m.i18n_key = 'navigation.system'
      AND m.deleted_at IS NULL
    LIMIT 1;
    IF v_system_parent_id IS NULL THEN CONTINUE; END IF;

    SELECT id INTO page_id FROM permission_menu
    WHERE platform_id = platform_row.id
      AND code = 'system:setting:view'
      AND deleted_at IS NULL
    LIMIT 1;
    changed := FALSE;
    IF page_id IS NULL THEN
      SELECT COALESCE(MAX(sort_order), 0) + 1 INTO next_sort
      FROM permission_menu AS m
      WHERE m.platform_id = platform_row.id AND m.parent_id = v_system_parent_id AND m.deleted_at IS NULL;
      INSERT INTO permission_menu
        (parent_id, menu_type, code, i18n_key, path, component_path, icon,
         sort_order, is_enabled, is_hidden, created_at, updated_at, name, platform_id, remark)
      VALUES
        (v_system_parent_id, 'page', 'system:setting:view', 'navigation.systemSetting',
         '/system/setting', 'system/setting', 'lucide:sliders-horizontal',
         next_sort, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
         '系统设置', platform_row.id, 'System setting management')
      RETURNING id INTO page_id;
      changed := TRUE;
    END IF;

    FOREACH action_code IN ARRAY ARRAY[
      'system:setting:list', 'system:setting:detail', 'system:setting:create',
      'system:setting:update', 'system:setting:status', 'system:setting:delete'
    ] LOOP
      IF NOT EXISTS (
        SELECT 1 FROM permission_menu
        WHERE platform_id = platform_row.id AND code = action_code AND deleted_at IS NULL
      ) THEN
        INSERT INTO permission_menu
          (parent_id, menu_type, code, i18n_key, path, component_path, icon,
           sort_order, is_enabled, is_hidden, created_at, updated_at, name, platform_id, remark)
        VALUES
          (page_id, 'action', action_code, NULL, NULL, NULL, NULL,
           0, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
           action_code, platform_row.id, 'System setting action');
        changed := TRUE;
      END IF;
    END LOOP;
    IF changed THEN
      UPDATE permission_auth_platform
      SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
      WHERE id = platform_row.id;
    END IF;
  END LOOP;
END $$;

COMMIT;
