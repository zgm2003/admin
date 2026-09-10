BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
LOCK TABLE permission_auth_platform, permission_menu IN ACCESS EXCLUSIVE MODE;

DO $$
DECLARE
  p RECORD;
  system_parent_id BIGINT;
  page_id BIGINT;
  next_sort INTEGER;
  changed BOOLEAN;
  action_code TEXT;
BEGIN
  FOR p IN
    SELECT platform.id
    FROM permission_auth_platform AS platform
    WHERE platform.deleted_at IS NULL
      AND EXISTS (
        SELECT 1
        FROM permission_menu AS sm
        WHERE sm.platform_id = platform.id
          AND sm.menu_type = 'directory'
          AND sm.i18n_key = 'navigation.system'
          AND sm.deleted_at IS NULL
      )
  LOOP
    SELECT id INTO system_parent_id FROM permission_menu
      WHERE platform_id = p.id AND menu_type = 'directory' AND i18n_key = 'navigation.system' AND deleted_at IS NULL LIMIT 1;
    IF system_parent_id IS NULL THEN RAISE EXCEPTION 'system directory is missing for platform %', p.id; END IF;
    SELECT COALESCE(MAX(m.sort_order), 0) + 1 INTO next_sort FROM permission_menu AS m WHERE m.platform_id = p.id AND m.parent_id = system_parent_id AND m.deleted_at IS NULL;
    SELECT id INTO page_id FROM permission_menu WHERE platform_id = p.id AND code = 'system:dictionary:view' AND deleted_at IS NULL LIMIT 1;
    changed := FALSE;
    IF page_id IS NULL THEN
      INSERT INTO permission_menu (parent_id, menu_type, code, i18n_key, path, icon, sort_order, is_enabled, created_at, updated_at, component_path, is_hidden, name, platform_id, remark)
      VALUES (system_parent_id, 'page', 'system:dictionary:view', 'navigation.systemDictionary', '/system/dictionary', 'lucide:list-tree', next_sort, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'system/dictionary', 0, '字典管理', p.id, 'System dictionary management')
      RETURNING id INTO page_id;
      changed := TRUE;
    END IF;
    FOREACH action_code IN ARRAY ARRAY['system:dictionary:list','system:dictionary:detail','system:dictionary:create','system:dictionary:update','system:dictionary:status','system:dictionary:delete'] LOOP
      IF NOT EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = p.id AND code = action_code AND deleted_at IS NULL) THEN
        INSERT INTO permission_menu (parent_id, menu_type, code, i18n_key, path, icon, sort_order, is_enabled, created_at, updated_at, component_path, is_hidden, name, platform_id, remark)
        VALUES (page_id, 'action', action_code, NULL, NULL, NULL, 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, NULL, 1, action_code, p.id, 'System dictionary action');
        changed := TRUE;
      END IF;
    END LOOP;
    IF changed THEN UPDATE permission_auth_platform SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = p.id; END IF;
  END LOOP;
END $$;
COMMIT;
