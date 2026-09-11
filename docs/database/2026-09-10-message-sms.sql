-- SMS vertical slice: seven tables, phone normalization, RBAC facts and seeds.
-- Must be executed by the maintainer only; API/Worker never run DDL.
BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

LOCK TABLE user_account, permission_auth_platform, permission_menu,
           permission_role, permission_role_menu,
           system_dictionary, system_dictionary_item
  IN ACCESS EXCLUSIVE MODE;

-- 1. Normalize the existing mainland phone facts and lock the format down.
DO $$
DECLARE
  invalid_count   BIGINT;
  collision_count BIGINT;
  updated_count   BIGINT;
BEGIN
  SELECT count(*) INTO invalid_count
    FROM user_account
   WHERE phone IS NOT NULL
     AND regexp_replace(regexp_replace(phone, '[ -]', '', 'g'), '^(\+86|86)', '')
         !~ '^1[3-9][0-9]{9}$';
  IF invalid_count > 0 THEN
    RAISE EXCEPTION 'cannot normalize % user_account.phone values to mainland E.164', invalid_count;
  END IF;

  SELECT count(*) INTO collision_count
    FROM (
      SELECT regexp_replace(regexp_replace(phone, '[ -]', '', 'g'), '^(\+86|86)', '') AS national
        FROM user_account
       WHERE phone IS NOT NULL AND deleted_at IS NULL
       GROUP BY 1
      HAVING count(*) > 1
    ) duplicated;
  IF collision_count > 0 THEN
    RAISE EXCEPTION 'phone normalization collides for % values', collision_count;
  END IF;

  WITH normalized AS (
    SELECT id,
           '+86' || regexp_replace(regexp_replace(phone, '[ -]', '', 'g'), '^(\+86|86)', '') AS e164
      FROM user_account
     WHERE phone IS NOT NULL
  )
  UPDATE user_account AS target
     SET phone = normalized.e164,
         updated_at = CURRENT_TIMESTAMP
    FROM normalized
   WHERE target.id = normalized.id
     AND target.phone <> normalized.e164;
  GET DIAGNOSTICS updated_count = ROW_COUNT;
  RAISE NOTICE 'normalized % user_account.phone values', updated_count;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conrelid = 'user_account'::regclass
       AND conname = 'ck_user_account_phone_e164'
  ) THEN
    ALTER TABLE user_account
      ADD CONSTRAINT ck_user_account_phone_e164
      CHECK (phone IS NULL OR phone ~ '^\+861[3-9][0-9]{9}$');
  END IF;
END $$;

-- 2. message_sms_config: global single active configuration, soft deleted.
CREATE TABLE IF NOT EXISTS message_sms_config (
  id                    BIGSERIAL PRIMARY KEY,
  secret_id_ciphertext  TEXT NOT NULL,
  secret_key_ciphertext TEXT NOT NULL,
  secret_id_hint        VARCHAR(32) NOT NULL DEFAULT '',
  secret_key_hint       VARCHAR(32) NOT NULL DEFAULT '',
  sms_sdk_app_id        VARCHAR(64) NOT NULL,
  sign_name             VARCHAR(128) NOT NULL,
  region                VARCHAR(64) NOT NULL,
  endpoint              VARCHAR(255),
  ttl_minutes           SMALLINT NOT NULL,
  is_enabled            SMALLINT NOT NULL DEFAULT 0,
  last_test_at          TIMESTAMPTZ,
  last_test_error       VARCHAR(512) NOT NULL DEFAULT '',
  created_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at            TIMESTAMPTZ,
  CONSTRAINT ck_message_sms_config_is_enabled CHECK (is_enabled IN (0, 1)),
  CONSTRAINT ck_message_sms_config_ttl_minutes CHECK (ttl_minutes >= 1 AND ttl_minutes <= 60)
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_sms_config_active_singleton
  ON message_sms_config ((true)) WHERE deleted_at IS NULL;

-- 3. message_sms_template: the four fixed scenes, never deleted.
CREATE TABLE IF NOT EXISTS message_sms_template (
  id                  BIGSERIAL PRIMARY KEY,
  scene               VARCHAR(32) NOT NULL,
  name                VARCHAR(128) NOT NULL,
  tencent_template_id VARCHAR(64) NOT NULL DEFAULT '',
  parameter_keys      JSONB NOT NULL,
  example_variables   JSONB NOT NULL,
  is_enabled          SMALLINT NOT NULL DEFAULT 0,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT ck_message_sms_template_scene
    CHECK (scene IN ('login', 'forget', 'bind_phone', 'change_password')),
  CONSTRAINT ck_message_sms_template_is_enabled CHECK (is_enabled IN (0, 1)),
  CONSTRAINT ck_message_sms_template_parameter_keys
    CHECK (parameter_keys = '["code","ttl_minutes"]'::jsonb)
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_sms_template_scene
  ON message_sms_template (scene);

-- 4. message_sms_recipient_rule: global allow/deny rules with ciphertext only.
CREATE TABLE IF NOT EXISTS message_sms_recipient_rule (
  id                 BIGSERIAL PRIMARY KEY,
  scope              VARCHAR(16) NOT NULL,
  pattern_ciphertext TEXT NOT NULL,
  pattern_hint       VARCHAR(64) NOT NULL,
  pattern_hmac       VARCHAR(128) NOT NULL,
  action             VARCHAR(16) NOT NULL,
  name               VARCHAR(128) NOT NULL,
  remark             VARCHAR(512) NOT NULL DEFAULT '',
  is_enabled         SMALLINT NOT NULL DEFAULT 1,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at         TIMESTAMPTZ,
  CONSTRAINT ck_message_sms_recipient_rule_scope CHECK (scope IN ('phone', 'prefix')),
  CONSTRAINT ck_message_sms_recipient_rule_action CHECK (action IN ('allow', 'deny')),
  CONSTRAINT ck_message_sms_recipient_rule_is_enabled CHECK (is_enabled IN (0, 1))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_sms_recipient_rule_pattern_action_active
  ON message_sms_recipient_rule (scope, pattern_hmac, action) WHERE deleted_at IS NULL;

-- 5. message_sms_rate_limit_policy: two fixed platform scoped policies.
CREATE TABLE IF NOT EXISTS message_sms_rate_limit_policy (
  platform_id    BIGINT NOT NULL,
  policy_key     VARCHAR(64) NOT NULL,
  mode           VARCHAR(16) NOT NULL,
  dimension      VARCHAR(64) NOT NULL,
  limit_count    INTEGER NOT NULL,
  window_seconds INTEGER NOT NULL,
  revision       BIGINT NOT NULL DEFAULT 1,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT message_sms_rate_limit_policy_pkey PRIMARY KEY (platform_id, policy_key),
  CONSTRAINT fk_message_sms_rate_limit_policy_platform
    FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id),
  CONSTRAINT ck_message_sms_rate_limit_policy_platform CHECK (platform_id > 0),
  CONSTRAINT ck_message_sms_rate_limit_policy_revision CHECK (revision >= 1),
  CONSTRAINT ck_message_sms_rate_limit_policy_shape
    CHECK (policy_key IN ('business_phone_minute', 'business_phone_10m')
           AND mode = 'business'
           AND dimension = 'platform_phone'),
  CONSTRAINT ck_message_sms_rate_limit_policy_values
    CHECK (limit_count >= 1 AND limit_count <= 100000
           AND window_seconds >= 1 AND window_seconds <= 86400)
);
CREATE INDEX IF NOT EXISTS ix_message_sms_rate_limit_policy_platform_revision
  ON message_sms_rate_limit_policy (platform_id, revision);

-- 6. message_sms_log: append-only audit keeping the source platform.
CREATE TABLE IF NOT EXISTS message_sms_log (
  id                  BIGSERIAL PRIMARY KEY,
  platform_id         BIGINT NOT NULL,
  challenge_id        VARCHAR(128),
  user_id             BIGINT,
  scene               VARCHAR(32) NOT NULL,
  template_id         BIGINT NOT NULL,
  to_phone_ciphertext TEXT NOT NULL,
  to_phone_hint       VARCHAR(32) NOT NULL,
  to_phone_hmac       VARCHAR(128) NOT NULL,
  status              VARCHAR(16) NOT NULL,
  request_id          VARCHAR(128) NOT NULL DEFAULT '',
  serial_no           VARCHAR(128) NOT NULL DEFAULT '',
  fee                 INTEGER NOT NULL DEFAULT 0,
  error_code          VARCHAR(128) NOT NULL DEFAULT '',
  error_summary       VARCHAR(512) NOT NULL DEFAULT '',
  latency_ms          BIGINT NOT NULL DEFAULT 0,
  sent_at             TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT uq_message_sms_log_id_platform UNIQUE (id, platform_id),
  CONSTRAINT fk_message_sms_log_platform
    FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id),
  CONSTRAINT fk_message_sms_log_template
    FOREIGN KEY (template_id) REFERENCES message_sms_template(id),
  CONSTRAINT ck_message_sms_log_scene
    CHECK (scene IN ('login', 'forget', 'bind_phone', 'change_password')),
  CONSTRAINT ck_message_sms_log_status CHECK (status IN ('pending', 'sent', 'failed')),
  CONSTRAINT ck_message_sms_log_fee CHECK (fee >= 0)
);
CREATE INDEX IF NOT EXISTS ix_message_sms_log_created_id_desc
  ON message_sms_log (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS ix_message_sms_log_platform_id_desc
  ON message_sms_log (platform_id, id DESC);
CREATE INDEX IF NOT EXISTS ix_message_sms_log_scene_id_desc
  ON message_sms_log (scene, id DESC);
CREATE INDEX IF NOT EXISTS ix_message_sms_log_status_id_desc
  ON message_sms_log (status, id DESC);
CREATE INDEX IF NOT EXISTS ix_message_sms_log_to_phone_hmac_id_desc
  ON message_sms_log (to_phone_hmac, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_sms_log_platform_challenge
  ON message_sms_log (platform_id, challenge_id) WHERE challenge_id IS NOT NULL;

-- 7. message_sms_log_verification: append-only ciphertext audit of the code.
CREATE TABLE IF NOT EXISTS message_sms_log_verification (
  id              BIGSERIAL PRIMARY KEY,
  platform_id     BIGINT NOT NULL,
  sms_log_id      BIGINT NOT NULL,
  key_version     VARCHAR(16) NOT NULL,
  code_ciphertext TEXT NOT NULL,
  expires_at      TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_message_sms_log_verification_platform
    FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id),
  CONSTRAINT fk_message_sms_log_verification_log
    FOREIGN KEY (sms_log_id, platform_id) REFERENCES message_sms_log(id, platform_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_sms_log_verification_log
  ON message_sms_log_verification (sms_log_id);

-- 8. user_phone_change_log: append-only audit of the phone identity changes.
CREATE TABLE IF NOT EXISTS user_phone_change_log (
  id             BIGSERIAL PRIMARY KEY,
  user_id        BIGINT NOT NULL,
  platform_id    BIGINT NOT NULL,
  action         VARCHAR(16) NOT NULL,
  old_phone_hint VARCHAR(32) NOT NULL DEFAULT '',
  old_phone_hmac VARCHAR(128) NOT NULL DEFAULT '',
  new_phone_hint VARCHAR(32) NOT NULL DEFAULT '',
  new_phone_hmac VARCHAR(128) NOT NULL DEFAULT '',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT ck_user_phone_change_log_action CHECK (action IN ('bind', 'change')),
  CONSTRAINT fk_user_phone_change_log_user
    FOREIGN KEY (user_id) REFERENCES user_account(id),
  CONSTRAINT fk_user_phone_change_log_platform
    FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id)
);
CREATE INDEX IF NOT EXISTS ix_user_phone_change_log_user_id_desc
  ON user_phone_change_log (user_id, id DESC);

-- 8b. user_email_change_log: append-only audit of the email identity changes.
CREATE TABLE IF NOT EXISTS user_email_change_log (
  id             BIGSERIAL PRIMARY KEY,
  user_id        BIGINT NOT NULL,
  platform_id    BIGINT NOT NULL,
  action         VARCHAR(16) NOT NULL,
  old_email_hint VARCHAR(128) NOT NULL DEFAULT '',
  old_email_hmac VARCHAR(128) NOT NULL DEFAULT '',
  new_email_hint VARCHAR(128) NOT NULL DEFAULT '',
  new_email_hmac VARCHAR(128) NOT NULL DEFAULT '',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT ck_user_email_change_log_action CHECK (action IN ('bind', 'change')),
  CONSTRAINT fk_user_email_change_log_user
    FOREIGN KEY (user_id) REFERENCES user_account(id),
  CONSTRAINT fk_user_email_change_log_platform
    FOREIGN KEY (platform_id) REFERENCES permission_auth_platform(id)
);
CREATE INDEX IF NOT EXISTS ix_user_email_change_log_user_id_desc
  ON user_email_change_log (user_id, id DESC);

-- 9. Seed the four fixed templates. Provider ids stay empty until configured.
INSERT INTO message_sms_template
  (scene, name, tencent_template_id, parameter_keys, example_variables, is_enabled, created_at, updated_at)
VALUES
  ('login', '登录验证码', '', '["code","ttl_minutes"]'::jsonb, '{"code":"123456","ttl_minutes":"5"}'::jsonb, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('forget', '找回密码', '', '["code","ttl_minutes"]'::jsonb, '{"code":"123456","ttl_minutes":"5"}'::jsonb, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('bind_phone', '绑定/换绑手机', '', '["code","ttl_minutes"]'::jsonb, '{"code":"123456","ttl_minutes":"5"}'::jsonb, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('change_password', '修改密码', '', '["code","ttl_minutes"]'::jsonb, '{"code":"123456","ttl_minutes":"5"}'::jsonb, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (scene) DO NOTHING;

-- 10. Provision the two default policies for every active platform.
INSERT INTO message_sms_rate_limit_policy
  (platform_id, policy_key, mode, dimension, limit_count, window_seconds, revision, created_at, updated_at)
SELECT platform.id, defaults.policy_key, 'business', 'platform_phone',
       defaults.limit_count, defaults.window_seconds, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
  FROM permission_auth_platform AS platform
 CROSS JOIN (VALUES
        ('business_phone_minute', 1, 60),
        ('business_phone_10m', 5, 600)
     ) AS defaults(policy_key, limit_count, window_seconds)
 WHERE platform.deleted_at IS NULL
ON CONFLICT (platform_id, policy_key) DO NOTHING;

-- 11. RBAC facts: SMS page/actions under the message directory,
--     Email/Phone identity actions under the hidden personal center page.
DO $$
DECLARE
  platform        RECORD;
  registered_role RECORD;
  message_dir_id  BIGINT;
  profile_page_id BIGINT;
  phone_action_id BIGINT;
  email_action_id BIGINT;
  page_id         BIGINT;
  next_sort       INTEGER;
  changed         BOOLEAN;
  action_code     TEXT;
  action_codes    TEXT[] := ARRAY[
    'message:sms:list',
    'message:sms:detail',
    'message:sms:config:update',
    'message:sms:config:delete',
    'message:sms:test',
    'message:sms:template:update',
    'message:sms:template:status',
    'message:sms:rule:create',
    'message:sms:rule:update',
    'message:sms:rule:status',
    'message:sms:rule:delete',
    'message:sms:rate-limit:update'
  ];
BEGIN
  FOR platform IN SELECT id FROM permission_auth_platform WHERE deleted_at IS NULL ORDER BY id LOOP
    changed := FALSE;

    SELECT menu.id INTO message_dir_id
      FROM permission_menu AS menu
     WHERE menu.platform_id = platform.id
       AND menu.menu_type = 'directory'
       AND menu.code = 'message'
       AND menu.deleted_at IS NULL
     LIMIT 1;

    IF message_dir_id IS NOT NULL THEN
      SELECT menu.id INTO page_id
        FROM permission_menu AS menu
       WHERE menu.platform_id = platform.id
         AND menu.code = 'message:sms:view'
         AND menu.deleted_at IS NULL
       LIMIT 1;

      IF page_id IS NULL THEN
        SELECT COALESCE(MAX(menu.sort_order), 0) + 1 INTO next_sort
          FROM permission_menu AS menu
         WHERE menu.platform_id = platform.id
           AND menu.parent_id = message_dir_id
           AND menu.deleted_at IS NULL;
        INSERT INTO permission_menu
          (platform_id, parent_id, menu_type, code, i18n_key, path, component_path, icon,
           sort_order, is_enabled, is_hidden, name, remark, created_at, updated_at)
        VALUES
          (platform.id, message_dir_id, 'page', 'message:sms:view', 'navigation.sms',
           '/message/sms', 'message/sms', 'lucide:message-square-more',
           next_sort, 1, 0, '短信服务', 'SMS service management', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
        RETURNING id INTO page_id;
        changed := TRUE;
      END IF;

      FOREACH action_code IN ARRAY action_codes LOOP
        IF NOT EXISTS (
          SELECT 1 FROM permission_menu AS menu
           WHERE menu.platform_id = platform.id
             AND menu.code = action_code
             AND menu.deleted_at IS NULL
        ) THEN
          INSERT INTO permission_menu
            (platform_id, parent_id, menu_type, code, i18n_key, path, component_path, icon,
             sort_order, is_enabled, is_hidden, name, remark, created_at, updated_at)
          VALUES
            (platform.id, page_id, 'action', action_code, NULL, NULL, NULL, NULL,
             0, 1, 1, action_code, 'SMS action', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
          changed := TRUE;
        END IF;
      END LOOP;
    END IF;

    SELECT menu.id INTO profile_page_id
      FROM permission_menu AS menu
     WHERE menu.platform_id = platform.id
       AND menu.code = 'user:profile:view'
       AND menu.deleted_at IS NULL
     LIMIT 1;

    IF profile_page_id IS NOT NULL THEN
      SELECT menu.id INTO phone_action_id
        FROM permission_menu AS menu
       WHERE menu.platform_id = platform.id
         AND menu.code = 'user:phone:update'
         AND menu.deleted_at IS NULL
       LIMIT 1;

      IF phone_action_id IS NULL THEN
        INSERT INTO permission_menu
          (platform_id, parent_id, menu_type, code, i18n_key, path, component_path, icon,
           sort_order, is_enabled, is_hidden, name, remark, created_at, updated_at)
        VALUES
          (platform.id, profile_page_id, 'action', 'user:phone:update', NULL, NULL, NULL, NULL,
           0, 1, 1, 'user:phone:update', 'Bind or change the account phone',
           CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
        RETURNING id INTO phone_action_id;
        changed := TRUE;
      END IF;

      SELECT menu.id INTO email_action_id
        FROM permission_menu AS menu
       WHERE menu.platform_id = platform.id
         AND menu.code = 'user:email:update'
         AND menu.deleted_at IS NULL
       LIMIT 1;

      IF email_action_id IS NULL THEN
        INSERT INTO permission_menu
          (platform_id, parent_id, menu_type, code, i18n_key, path, component_path, icon,
           sort_order, is_enabled, is_hidden, name, remark, created_at, updated_at)
        VALUES
          (platform.id, profile_page_id, 'action', 'user:email:update', NULL, NULL, NULL, NULL,
           0, 1, 1, 'user:email:update', 'Bind or change the account email',
           CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
        RETURNING id INTO email_action_id;
        changed := TRUE;
      END IF;

      -- Page and action grants stay explicit: the default role receives both
      -- identity actions without inheriting any unrelated page or action.
      FOR registered_role IN
        SELECT app_role.id
          FROM permission_role AS app_role
         WHERE app_role.code = 'registered_user'
           AND app_role.is_enabled = 1
           AND app_role.deleted_at IS NULL
      LOOP
        IF NOT EXISTS (
          SELECT 1 FROM permission_role_menu AS grant_row
           WHERE grant_row.role_id = registered_role.id
             AND grant_row.menu_id = phone_action_id
             AND grant_row.deleted_at IS NULL
        ) THEN
          INSERT INTO permission_role_menu (role_id, menu_id, created_at, updated_at)
          VALUES (registered_role.id, phone_action_id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
          changed := TRUE;
        END IF;

        IF NOT EXISTS (
          SELECT 1 FROM permission_role_menu AS grant_row
           WHERE grant_row.role_id = registered_role.id
             AND grant_row.menu_id = email_action_id
             AND grant_row.deleted_at IS NULL
        ) THEN
          INSERT INTO permission_role_menu (role_id, menu_id, created_at, updated_at)
          VALUES (registered_role.id, email_action_id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
          changed := TRUE;
        END IF;
      END LOOP;
    END IF;

    IF changed THEN
      UPDATE permission_auth_platform
         SET menu_version = menu_version + 1,
             updated_at = CURRENT_TIMESTAMP
       WHERE id = platform.id;
    END IF;
  END LOOP;
END $$;

-- 12. Seed the SMS region display option. Scenes and statuses stay in code.
DO $$
DECLARE
  v_dictionary_id BIGINT;
BEGIN
  SELECT id INTO v_dictionary_id
    FROM system_dictionary
   WHERE code = 'message.sms.region'
     AND deleted_at IS NULL;

  IF v_dictionary_id IS NULL THEN
    INSERT INTO system_dictionary
      (code, name_zh, name_en, description, is_enabled, is_builtin, created_at, updated_at)
    VALUES
      ('message.sms.region', '短信地域', 'SMS Regions',
       'Tencent Cloud SMS region options', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    RETURNING id INTO v_dictionary_id;
  END IF;

  INSERT INTO system_dictionary_item
    (dictionary_id, value, label_zh, label_en, sort, is_enabled, is_builtin, created_at, updated_at)
  VALUES
    (v_dictionary_id, 'ap-guangzhou', '广州', 'Guangzhou', 10, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
  ON CONFLICT (dictionary_id, value) WHERE deleted_at IS NULL DO NOTHING;
END $$;

COMMIT;
