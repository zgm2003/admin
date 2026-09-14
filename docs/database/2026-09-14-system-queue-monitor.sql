BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
  admin_platform_id BIGINT;
  system_menu_id BIGINT;
  queue_page_id BIGINT;
  changed BOOLEAN := FALSE;
BEGIN
  SELECT id INTO admin_platform_id
  FROM permission_auth_platform
  WHERE code = 'admin' AND deleted_at IS NULL AND is_enabled = 1;
  IF admin_platform_id IS NULL THEN
    RAISE EXCEPTION 'active Admin platform is required';
  END IF;

  SELECT id INTO system_menu_id
  FROM permission_menu
  WHERE platform_id = admin_platform_id AND code = 'system'
    AND menu_type = 'directory' AND deleted_at IS NULL;
  IF system_menu_id IS NULL THEN
    RAISE EXCEPTION 'active system directory is required';
  END IF;

  IF (SELECT count(*) FROM permission_menu
      WHERE platform_id = admin_platform_id AND code = 'system:queueMonitor:view'
        AND deleted_at IS NULL) > 1 THEN
    RAISE EXCEPTION 'duplicate active system:queueMonitor:view rows';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM permission_menu
    WHERE platform_id = admin_platform_id AND code = 'system:queueMonitor:view'
      AND deleted_at IS NULL) THEN
    INSERT INTO permission_menu
      (platform_id, parent_id, menu_type, name, code, i18n_key, path, component_path,
       icon, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES
      (admin_platform_id, system_menu_id, 'page', '任务队列', 'system:queueMonitor:view',
       'navigation.systemQueueMonitor', '/system/queueMonitor', 'system/queueMonitor',
       'lucide:list-tree', 90, 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  ELSE
    SELECT id INTO queue_page_id FROM permission_menu
    WHERE platform_id = admin_platform_id AND code = 'system:queueMonitor:view'
      AND deleted_at IS NULL;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id = queue_page_id AND
      (parent_id IS DISTINCT FROM system_menu_id OR menu_type <> 'page' OR name <> '任务队列' OR
       i18n_key IS DISTINCT FROM 'navigation.systemQueueMonitor' OR path IS DISTINCT FROM '/system/queueMonitor' OR
       component_path IS DISTINCT FROM 'system/queueMonitor' OR icon IS DISTINCT FROM 'lucide:list-tree' OR
       is_enabled <> 1 OR is_hidden <> 0)) THEN
      RAISE EXCEPTION 'system:queueMonitor:view menu shape mismatch';
    END IF;
  END IF;

  SELECT id INTO queue_page_id FROM permission_menu
  WHERE platform_id = admin_platform_id AND code = 'system:queueMonitor:view'
    AND deleted_at IS NULL;
  IF (SELECT count(*) FROM permission_menu
      WHERE platform_id = admin_platform_id AND code = 'system:queueMonitor:list'
        AND deleted_at IS NULL) > 1 THEN
    RAISE EXCEPTION 'duplicate active system:queueMonitor:list rows';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM permission_menu
    WHERE platform_id = admin_platform_id AND code = 'system:queueMonitor:list'
      AND deleted_at IS NULL) THEN
    INSERT INTO permission_menu
      (platform_id, parent_id, menu_type, name, code, sort_order, is_enabled, is_hidden, created_at, updated_at)
    VALUES (admin_platform_id, queue_page_id, 'action', '查看任务队列',
      'system:queueMonitor:list', 100, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    changed := TRUE;
  ELSE
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = admin_platform_id
      AND code = 'system:queueMonitor:list' AND deleted_at IS NULL
      AND (parent_id IS DISTINCT FROM queue_page_id OR menu_type <> 'action' OR name <> '查看任务队列'
        OR i18n_key IS NOT NULL OR path IS NOT NULL OR component_path IS NOT NULL OR icon IS NOT NULL
        OR is_enabled <> 1 OR is_hidden <> 1)) THEN
      RAISE EXCEPTION 'system:queueMonitor:list menu shape mismatch';
    END IF;
  END IF;

  IF changed THEN
    UPDATE permission_auth_platform SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
    WHERE id = admin_platform_id;
  END IF;
END $$;
COMMIT;
