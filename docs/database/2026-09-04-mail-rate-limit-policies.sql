BEGIN;
CREATE TABLE IF NOT EXISTS message_mail_rate_limit_policy (
  policy_key VARCHAR(64) PRIMARY KEY,
  mode VARCHAR(16) NOT NULL,
  dimension VARCHAR(64) NOT NULL,
  limit_count INTEGER NOT NULL,
  window_seconds INTEGER NOT NULL,
  revision BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT ck_message_mail_rate_limit_policy_values
    CHECK (limit_count BETWEEN 1 AND 100000 AND window_seconds BETWEEN 1 AND 86400),
  CONSTRAINT ck_message_mail_rate_limit_policy_revision CHECK (revision >= 1),
  CONSTRAINT ck_message_mail_rate_limit_policy_shape CHECK (
    (policy_key = 'business_email_minute' AND mode = 'business' AND dimension = 'platform_scene_email') OR
    (policy_key = 'business_email_10m' AND mode = 'business' AND dimension = 'platform_scene_email') OR
    (policy_key = 'business_ip_minute' AND mode = 'business' AND dimension = 'platform_ip') OR
    (policy_key = 'business_scene_minute' AND mode = 'business' AND dimension = 'platform_scene') OR
    (policy_key = 'admin_test_user_10m' AND mode = 'admin_test' AND dimension = 'admin_user') OR
    (policy_key = 'admin_test_ip_minute' AND mode = 'admin_test' AND dimension = 'ip') OR
    (policy_key = 'admin_test_email_10m' AND mode = 'admin_test' AND dimension = 'email')
  )
);
INSERT INTO message_mail_rate_limit_policy
  (policy_key, mode, dimension, limit_count, window_seconds, revision)
VALUES
  ('business_email_minute', 'business', 'platform_scene_email', 1, 60, 1),
  ('business_email_10m', 'business', 'platform_scene_email', 5, 600, 1),
  ('business_ip_minute', 'business', 'platform_ip', 10, 60, 1),
  ('business_scene_minute', 'business', 'platform_scene', 30, 60, 1),
  ('admin_test_user_10m', 'admin_test', 'admin_user', 5, 600, 1),
  ('admin_test_ip_minute', 'admin_test', 'ip', 10, 60, 1),
  ('admin_test_email_10m', 'admin_test', 'email', 3, 600, 1)
ON CONFLICT (policy_key) DO NOTHING;

DO $$
DECLARE
  admin_platform_id BIGINT;
  mail_page_id BIGINT;
  mail_page_count INTEGER;
  action_id BIGINT;
  action_count INTEGER;
  changed BOOLEAN := FALSE;
BEGIN
  SELECT id INTO admin_platform_id
  FROM auth_platform
  WHERE code = 'admin' AND deleted_at IS NULL AND is_enabled = 1;
  IF admin_platform_id IS NULL THEN
    RAISE EXCEPTION 'active Admin platform is required';
  END IF;

  SELECT count(*) INTO mail_page_count
  FROM permission_menu
  WHERE platform_id = admin_platform_id
    AND code = 'message:mail:view'
    AND menu_type = 'page'
    AND deleted_at IS NULL;
  IF mail_page_count <> 1 THEN
    RAISE EXCEPTION 'exactly one active message:mail:view page is required';
  END IF;
  SELECT id INTO mail_page_id
  FROM permission_menu
  WHERE platform_id = admin_platform_id
    AND code = 'message:mail:view'
    AND menu_type = 'page'
    AND deleted_at IS NULL;

  SELECT count(*) INTO action_count
  FROM permission_menu
  WHERE platform_id = admin_platform_id
    AND code = 'message:mail:rate-limit:update' AND deleted_at IS NULL;
  IF action_count > 1 THEN
    RAISE EXCEPTION 'message:mail:rate-limit:update has duplicate active rows';
  END IF;
  IF action_count = 1 THEN
    SELECT id INTO action_id
    FROM permission_menu
    WHERE platform_id = admin_platform_id
      AND code = 'message:mail:rate-limit:update' AND deleted_at IS NULL;
    IF EXISTS (
      SELECT 1 FROM permission_menu
      WHERE id = action_id
        AND (platform_id IS DISTINCT FROM admin_platform_id
          OR parent_id IS DISTINCT FROM mail_page_id
          OR menu_type <> 'action'
          OR name <> '修改邮件限流策略'
          OR i18n_key IS NOT NULL
          OR path IS NOT NULL
          OR component_path IS NOT NULL
          OR icon IS NOT NULL
          OR is_enabled <> 1
          OR is_hidden <> 1)
    ) THEN
      RAISE EXCEPTION 'message:mail:rate-limit:update menu shape mismatch';
    END IF;
  ELSE
    INSERT INTO permission_menu
      (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon,
       sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES
      (admin_platform_id, mail_page_id, 'action', '修改邮件限流策略',
       'message:mail:rate-limit:update', NULL, NULL, NULL, NULL, 100, 1, 1,
       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    RETURNING id INTO action_id;
    changed := TRUE;
  END IF;

  IF changed THEN
    UPDATE permission_access_version
    SET version = version + 1, updated_at = CURRENT_TIMESTAMP;
  END IF;
END $$;
COMMIT;
