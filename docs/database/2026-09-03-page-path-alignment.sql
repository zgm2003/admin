BEGIN;

DO $$
DECLARE
  target RECORD;
  changed_rows BIGINT := 0;
  row_count BIGINT;
BEGIN
  FOR target IN
    SELECT *
    FROM (VALUES
      ('permission:menu:view', '/access/menus', 'access/menus', '/permission/menus', 'permission/menus'),
      ('permission:role:view', '/access/roles', 'access/roles', '/permission/roles', 'permission/roles'),
      ('auth:platform:view', '/access/auth-platforms', 'access/auth-platforms', '/permission/auth-platforms', 'permission/auth-platforms'),
      ('account:user:loginlog:view', '/account/login-logs', 'user/login-logs', '/account/login-logs', 'account/login-logs'),
      ('storage:object:view', '/cloud/object-storage', 'storage/object', '/cloud/storage-object', 'cloud/storage-object')
    ) AS mapping(code, old_path, old_component_path, new_path, new_component_path)
  LOOP
    IF EXISTS (
      SELECT 1
      FROM permission_menu
      WHERE deleted_at IS NULL
        AND code = target.code
        AND NOT (
          (path IS NOT DISTINCT FROM target.old_path AND component_path IS NOT DISTINCT FROM target.old_component_path)
          OR (path IS NOT DISTINCT FROM target.new_path AND component_path IS NOT DISTINCT FROM target.new_component_path)
        )
    ) THEN
      RAISE EXCEPTION 'menu % has an unexpected path shape', target.code;
    END IF;

    IF EXISTS (
      SELECT 1
      FROM permission_menu source
      JOIN permission_menu occupied
        ON occupied.platform_id = source.platform_id
       AND occupied.id <> source.id
       AND occupied.deleted_at IS NULL
       AND occupied.menu_type = 'page'
       AND occupied.path = target.new_path
      WHERE source.deleted_at IS NULL
        AND source.code = target.code
    ) THEN
      RAISE EXCEPTION 'menu % target path is occupied', target.code;
    END IF;

    UPDATE permission_menu
    SET path = target.new_path,
        component_path = target.new_component_path,
        updated_at = CURRENT_TIMESTAMP
    WHERE deleted_at IS NULL
      AND code = target.code
      AND path = target.old_path
      AND component_path = target.old_component_path;
    GET DIAGNOSTICS row_count = ROW_COUNT;
    changed_rows := changed_rows + row_count;
  END LOOP;

  IF changed_rows > 0 THEN
    UPDATE permission_access_version
    SET version = version + 1, updated_at = CURRENT_TIMESTAMP;
  END IF;
END $$;

COMMIT;
