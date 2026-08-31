BEGIN;

DO $$
DECLARE
  admin_platform_id BIGINT;
  account_id BIGINT;
  profile_id BIGINT;
  login_log_id BIGINT;
  menu_page_id BIGINT;
  old_canvas_count BIGINT;
  new_canvas_count BIGINT;
  changed BOOLEAN := FALSE;
BEGIN
  SELECT id INTO admin_platform_id
  FROM auth_platform
  WHERE code = 'admin' AND deleted_at IS NULL AND is_enabled = 1;
  IF admin_platform_id IS NULL OR (SELECT count(*) FROM auth_platform WHERE code = 'admin' AND deleted_at IS NULL AND is_enabled = 1) <> 1 THEN
    RAISE EXCEPTION 'expected exactly one active Admin platform';
  END IF;

  SELECT id INTO account_id
  FROM rbac_menu
  WHERE platform_id = admin_platform_id AND code = 'account' AND menu_type = 'directory' AND deleted_at IS NULL;
  IF account_id IS NULL OR (SELECT count(*) FROM rbac_menu WHERE platform_id = admin_platform_id AND code = 'account' AND deleted_at IS NULL) <> 1 THEN
    RAISE EXCEPTION 'Admin account directory is missing or duplicated';
  END IF;

  SELECT count(*) INTO old_canvas_count FROM rbac_menu m
  JOIN auth_platform p ON p.id = m.platform_id
  WHERE p.code = 'canvas' AND p.deleted_at IS NULL AND m.code = 'canvas:test' AND m.deleted_at IS NULL;
  SELECT count(*) INTO new_canvas_count FROM rbac_menu m
  JOIN auth_platform p ON p.id = m.platform_id
  WHERE p.code = 'canvas' AND p.deleted_at IS NULL AND m.code = 'canvas:test:list' AND m.deleted_at IS NULL;
  IF old_canvas_count > 1 OR new_canvas_count > 1 OR (old_canvas_count > 0 AND new_canvas_count > 0) THEN
    RAISE EXCEPTION 'Canvas page code conflict';
  END IF;
  IF old_canvas_count = 1 THEN
    UPDATE rbac_menu m SET code = 'canvas:test:list', updated_at = CURRENT_TIMESTAMP
    FROM auth_platform p
    WHERE m.platform_id = p.id AND p.code = 'canvas' AND p.deleted_at IS NULL
      AND m.code = 'canvas:test' AND m.deleted_at IS NULL;
    changed := TRUE;
  ELSIF new_canvas_count = 1 THEN
    IF EXISTS (
      SELECT 1 FROM rbac_menu m JOIN auth_platform p ON p.id = m.platform_id
      WHERE p.code = 'canvas' AND p.deleted_at IS NULL AND m.code = 'canvas:test:list'
        AND (m.menu_type <> 'page' OR m.path IS NULL OR m.component_path IS NULL OR m.is_enabled <> 1 OR m.is_hidden <> 0)
    ) THEN RAISE EXCEPTION 'Canvas target page shape mismatch'; END IF;
  END IF;

  SELECT id INTO profile_id FROM rbac_menu
  WHERE platform_id = admin_platform_id AND code = 'account:profile:list' AND deleted_at IS NULL;
  IF profile_id IS NOT NULL THEN
    IF EXISTS (SELECT 1 FROM rbac_menu WHERE id = profile_id AND (parent_id IS DISTINCT FROM account_id OR menu_type <> 'page' OR path IS DISTINCT FROM '/account/profile' OR component_path IS DISTINCT FROM 'account/profile' OR i18n_key IS DISTINCT FROM 'layout.account.profile' OR is_hidden <> 1)) THEN
      RAISE EXCEPTION 'profile page shape mismatch';
    END IF;
  ELSE
    INSERT INTO rbac_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, account_id, 'page', '个人中心', 'account:profile:list', 'layout.account.profile', '/account/profile', 'account/profile', 'lucide:user-circle', 90, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    RETURNING id INTO profile_id;
    changed := TRUE;
  END IF;

  IF EXISTS (SELECT 1 FROM rbac_menu WHERE platform_id = admin_platform_id AND code IN ('account:profile:update', 'account:password:update') AND deleted_at IS NULL GROUP BY code HAVING count(*) <> 1) THEN
    RAISE EXCEPTION 'profile action code conflict';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM rbac_menu WHERE parent_id = profile_id AND code = 'account:profile:update' AND deleted_at IS NULL) THEN
    INSERT INTO rbac_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, profile_id, 'action', '修改资料', 'account:profile:update', NULL, NULL, NULL, NULL, 10, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM rbac_menu WHERE parent_id = profile_id AND code = 'account:password:update' AND deleted_at IS NULL) THEN
    INSERT INTO rbac_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, profile_id, 'action', '修改密码', 'account:password:update', NULL, NULL, NULL, NULL, 20, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  END IF;

  SELECT id INTO login_log_id FROM rbac_menu
  WHERE platform_id = admin_platform_id AND code = 'account:user:loginlog:list' AND deleted_at IS NULL;
  IF (SELECT count(*) FROM rbac_menu WHERE platform_id = admin_platform_id AND code = 'account:user:loginlog:list' AND deleted_at IS NULL) > 1 THEN
    RAISE EXCEPTION 'login log page code conflict';
  END IF;
  IF login_log_id IS NOT NULL THEN
    IF EXISTS (SELECT 1 FROM rbac_menu WHERE id = login_log_id AND (parent_id IS DISTINCT FROM account_id OR menu_type <> 'page' OR path IS DISTINCT FROM '/account/login-logs' OR component_path IS DISTINCT FROM 'user/login-logs' OR i18n_key IS DISTINCT FROM 'navigation.accountLoginLogs' OR icon IS DISTINCT FROM 'lucide:lock-keyhole' OR sort_order <> 30 OR is_enabled <> 1 OR is_hidden <> 0)) THEN
      RAISE EXCEPTION 'login log page shape mismatch';
    END IF;
  ELSE
    INSERT INTO rbac_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, account_id, 'page', '登录日志', 'account:user:loginlog:list', 'navigation.accountLoginLogs', '/account/login-logs', 'user/login-logs', 'lucide:lock-keyhole', 30, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  END IF;

  SELECT id INTO menu_page_id FROM rbac_menu
  WHERE platform_id = admin_platform_id AND code = 'rbac:menu:list' AND deleted_at IS NULL;
  IF menu_page_id IS NULL THEN
    RAISE EXCEPTION 'menu management page is missing';
  END IF;
  IF (SELECT count(*) FROM rbac_menu WHERE platform_id = admin_platform_id AND code = 'rbac:access-cache:rebuild' AND deleted_at IS NULL) > 1 THEN
    RAISE EXCEPTION 'access cache rebuild action code conflict';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM rbac_menu WHERE platform_id = admin_platform_id AND parent_id = menu_page_id AND code = 'rbac:access-cache:rebuild' AND deleted_at IS NULL) THEN
    INSERT INTO rbac_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, menu_page_id, 'action', '重建访问缓存', 'rbac:access-cache:rebuild', NULL, NULL, NULL, NULL, 40, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  END IF;

  IF changed THEN
    UPDATE rbac_access_version v SET version = v.version + 1, updated_at = CURRENT_TIMESTAMP
    FROM user_account u
    WHERE u.id = v.user_id AND u.is_enabled = 1 AND u.deleted_at IS NULL;
  END IF;
END $$;

COMMIT;
