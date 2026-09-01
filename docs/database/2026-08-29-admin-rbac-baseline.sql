BEGIN;

DO $$
DECLARE
  admin_platform_id BIGINT;
  account_id BIGINT;
  profile_id BIGINT;
  login_log_id BIGINT;
  menu_page_id BIGINT;
  read_action_id BIGINT;
  read_page_id BIGINT;
  read_definition RECORD;
  old_canvas_count BIGINT;
  new_canvas_count BIGINT;
  changed_rows BIGINT;
  changed BOOLEAN := FALSE;
BEGIN
  UPDATE permission_menu
  SET code = regexp_replace(code, ':list$', ':view'), updated_at = CURRENT_TIMESTAMP
  WHERE menu_type = 'page' AND code LIKE '%:list' AND deleted_at IS NULL;
  GET DIAGNOSTICS changed_rows = ROW_COUNT;
  changed := changed_rows > 0;
  SELECT id INTO admin_platform_id
  FROM auth_platform
  WHERE code = 'admin' AND deleted_at IS NULL AND is_enabled = 1;
  IF admin_platform_id IS NULL OR (SELECT count(*) FROM auth_platform WHERE code = 'admin' AND deleted_at IS NULL AND is_enabled = 1) <> 1 THEN
    RAISE EXCEPTION 'expected exactly one active Admin platform';
  END IF;

  SELECT id INTO account_id
  FROM permission_menu
  WHERE platform_id = admin_platform_id AND code = 'account' AND menu_type = 'directory' AND deleted_at IS NULL;
  IF account_id IS NULL OR (SELECT count(*) FROM permission_menu WHERE platform_id = admin_platform_id AND code = 'account' AND deleted_at IS NULL) <> 1 THEN
    RAISE EXCEPTION 'Admin account directory is missing or duplicated';
  END IF;

  SELECT count(*) INTO old_canvas_count FROM permission_menu m
  JOIN auth_platform p ON p.id = m.platform_id
  WHERE p.code = 'canvas' AND p.deleted_at IS NULL AND m.code = 'canvas:test' AND m.deleted_at IS NULL;
  SELECT count(*) INTO new_canvas_count FROM permission_menu m
  JOIN auth_platform p ON p.id = m.platform_id
  WHERE p.code = 'canvas' AND p.deleted_at IS NULL AND m.code = 'canvas:test:view' AND m.deleted_at IS NULL;
  IF old_canvas_count > 1 OR new_canvas_count > 1 OR (old_canvas_count > 0 AND new_canvas_count > 0) THEN
    RAISE EXCEPTION 'Canvas page code conflict';
  END IF;
  IF old_canvas_count = 1 THEN
    UPDATE permission_menu m SET code = 'canvas:test:view', updated_at = CURRENT_TIMESTAMP
    FROM auth_platform p
    WHERE m.platform_id = p.id AND p.code = 'canvas' AND p.deleted_at IS NULL
      AND m.code = 'canvas:test' AND m.deleted_at IS NULL;
    changed := TRUE;
  ELSIF new_canvas_count = 1 THEN
    IF EXISTS (
      SELECT 1 FROM permission_menu m JOIN auth_platform p ON p.id = m.platform_id
      WHERE p.code = 'canvas' AND p.deleted_at IS NULL AND m.code = 'canvas:test:view'
        AND (m.menu_type <> 'page' OR m.path IS NULL OR m.component_path IS NULL OR m.is_enabled <> 1 OR m.is_hidden <> 0)
    ) THEN RAISE EXCEPTION 'Canvas target page shape mismatch'; END IF;
  END IF;

  SELECT id INTO profile_id FROM permission_menu
  WHERE platform_id = admin_platform_id AND code = 'account:profile:view' AND deleted_at IS NULL;
  IF profile_id IS NOT NULL THEN
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id = profile_id AND (parent_id IS DISTINCT FROM account_id OR menu_type <> 'page' OR path IS DISTINCT FROM '/account/profile' OR component_path IS DISTINCT FROM 'account/profile' OR i18n_key IS DISTINCT FROM 'layout.account.profile' OR is_hidden <> 1)) THEN
      RAISE EXCEPTION 'profile page shape mismatch';
    END IF;
  ELSE
    INSERT INTO permission_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, account_id, 'page', '个人中心', 'account:profile:view', 'layout.account.profile', '/account/profile', 'account/profile', 'lucide:user-circle', 90, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    RETURNING id INTO profile_id;
    changed := TRUE;
  END IF;

  IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = admin_platform_id AND code IN ('account:profile:update', 'account:password:update', 'account:profile:detail') AND deleted_at IS NULL GROUP BY code HAVING count(*) <> 1) THEN
    RAISE EXCEPTION 'profile action code conflict';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM permission_menu WHERE parent_id = profile_id AND code = 'account:profile:update' AND deleted_at IS NULL) THEN
    INSERT INTO permission_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, profile_id, 'action', '修改资料', 'account:profile:update', NULL, NULL, NULL, NULL, 10, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM permission_menu WHERE parent_id = profile_id AND code = 'account:password:update' AND deleted_at IS NULL) THEN
    INSERT INTO permission_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, profile_id, 'action', '修改密码', 'account:password:update', NULL, NULL, NULL, NULL, 20, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  END IF;

  SELECT id INTO login_log_id FROM permission_menu
  WHERE platform_id = admin_platform_id AND code = 'account:user:loginlog:view' AND deleted_at IS NULL;
  IF (SELECT count(*) FROM permission_menu WHERE platform_id = admin_platform_id AND code = 'account:user:loginlog:view' AND deleted_at IS NULL) > 1 THEN
    RAISE EXCEPTION 'login log page code conflict';
  END IF;
  IF login_log_id IS NOT NULL THEN
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id = login_log_id AND (parent_id IS DISTINCT FROM account_id OR menu_type <> 'page' OR path IS DISTINCT FROM '/account/login-logs' OR component_path IS DISTINCT FROM 'user/login-logs' OR i18n_key IS DISTINCT FROM 'navigation.accountLoginLogs' OR icon IS DISTINCT FROM 'lucide:lock-keyhole' OR sort_order <> 30 OR is_enabled <> 1 OR is_hidden <> 0)) THEN
      RAISE EXCEPTION 'login log page shape mismatch';
    END IF;
  ELSE
    INSERT INTO permission_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, account_id, 'page', '登录日志', 'account:user:loginlog:view', 'navigation.accountLoginLogs', '/account/login-logs', 'user/login-logs', 'lucide:lock-keyhole', 30, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  END IF;

  SELECT id INTO menu_page_id FROM permission_menu
  WHERE platform_id = admin_platform_id AND code = 'permission:menu:view' AND deleted_at IS NULL;
  IF menu_page_id IS NULL THEN
    RAISE EXCEPTION 'menu management page is missing';
  END IF;

  FOR read_definition IN
    SELECT * FROM (VALUES
      ('permission:menu:view', 'permission:menu:list', '读取菜单列表', 5),
      ('permission:role:view', 'permission:role:list', '读取角色列表', 5),
      ('account:user:view', 'account:user:list', '读取用户列表', 5),
      ('auth:platform:view', 'auth:platform:list', '读取认证平台列表', 5),
      ('auth:session:view', 'auth:session:list', '读取会话列表', 5),
      ('system:operation-log:view', 'system:operation-log:list', '读取操作日志', 5),
      ('account:user:loginlog:view', 'account:user:loginlog:list', '读取登录日志', 5),
      ('account:profile:view', 'account:profile:detail', '读取个人资料', 5)
    ) AS definitions(parent_code, action_code, action_name, sort_order)
  LOOP
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = admin_platform_id AND code = read_definition.action_code AND deleted_at IS NULL GROUP BY code HAVING count(*) <> 1) THEN
      RAISE EXCEPTION 'read action code conflict: %', read_definition.action_code;
    END IF;
    SELECT id INTO read_page_id FROM permission_menu
    WHERE platform_id = admin_platform_id AND code = read_definition.parent_code AND menu_type = 'page' AND deleted_at IS NULL;
    IF read_page_id IS NULL THEN
      CONTINUE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM permission_menu WHERE parent_id = read_page_id AND code = read_definition.action_code AND deleted_at IS NULL) THEN
      INSERT INTO permission_menu (platform_id, parent_id, menu_type, name, code, sort_order, is_enabled, is_hidden, created_at, updated_at)
      VALUES (admin_platform_id, read_page_id, 'action', read_definition.action_name, read_definition.action_code, read_definition.sort_order, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
      RETURNING id INTO read_action_id;
      changed := TRUE;
      INSERT INTO permission_role_menu (role_id, menu_id, created_at, updated_at)
      SELECT DISTINCT grants.role_id, read_action_id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
      FROM permission_role_menu grants
      JOIN permission_menu granted ON granted.id = grants.menu_id AND grants.deleted_at IS NULL
      WHERE (granted.id = read_page_id OR granted.parent_id = read_page_id)
        AND NOT EXISTS (SELECT 1 FROM permission_role_menu existing WHERE existing.role_id = grants.role_id AND existing.menu_id = read_action_id AND existing.deleted_at IS NULL);
    END IF;
  END LOOP;
  IF (SELECT count(*) FROM permission_menu WHERE platform_id = admin_platform_id AND code = 'permission:access-cache:rebuild' AND deleted_at IS NULL) > 1 THEN
    RAISE EXCEPTION 'access cache rebuild action code conflict';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = admin_platform_id AND parent_id = menu_page_id AND code = 'permission:access-cache:rebuild' AND deleted_at IS NULL) THEN
    INSERT INTO permission_menu (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path, icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, menu_page_id, 'action', '重建访问缓存', 'permission:access-cache:rebuild', NULL, NULL, NULL, NULL, 40, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  END IF;

  IF changed THEN
    UPDATE permission_access_version v SET version = v.version + 1, updated_at = CURRENT_TIMESTAMP
    FROM user_account u
    WHERE u.id = v.user_id AND u.is_enabled = 1 AND u.deleted_at IS NULL;
  END IF;
END $$;

COMMIT;
