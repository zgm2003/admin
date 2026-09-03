BEGIN;

DO
$$
DECLARE
  admin_platform_id BIGINT;
  changed_rows BIGINT;
  changed BOOLEAN := FALSE;
BEGIN
  SELECT id
  INTO admin_platform_id
  FROM auth_platform
  WHERE code = 'admin'
    AND is_enabled = 1
    AND deleted_at IS NULL;

  IF admin_platform_id IS NULL OR (
    SELECT count(*)
    FROM auth_platform
    WHERE code = 'admin'
      AND is_enabled = 1
      AND deleted_at IS NULL
  ) <> 1 THEN
    RAISE EXCEPTION 'expected exactly one active Admin platform';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM permission_menu
    WHERE platform_id = admin_platform_id
      AND code = 'message'
      AND menu_type = 'directory'
      AND deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'Admin message directory is missing';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM permission_menu
    WHERE platform_id = admin_platform_id
      AND code = 'message:mail:view'
      AND menu_type = 'page'
      AND deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'Admin mail page is missing';
  END IF;

  UPDATE permission_menu
  SET icon = 'lucide:message-square-more',
      updated_at = CURRENT_TIMESTAMP
  WHERE platform_id = admin_platform_id
    AND code = 'message'
    AND menu_type = 'directory'
    AND icon IS DISTINCT FROM 'lucide:message-square-more'
    AND deleted_at IS NULL;
  GET DIAGNOSTICS changed_rows = ROW_COUNT;
  changed := changed_rows > 0;

  UPDATE permission_menu
  SET icon = 'lucide:mail',
      updated_at = CURRENT_TIMESTAMP
  WHERE platform_id = admin_platform_id
    AND code = 'message:mail:view'
    AND menu_type = 'page'
    AND icon IS DISTINCT FROM 'lucide:mail'
    AND deleted_at IS NULL;
  GET DIAGNOSTICS changed_rows = ROW_COUNT;
  changed := changed OR changed_rows > 0;

  IF changed THEN
    UPDATE permission_access_version version
    SET version = version.version + 1,
        updated_at = CURRENT_TIMESTAMP
    FROM user_account account
    WHERE account.id = version.user_id
      AND account.is_enabled = 1
      AND account.deleted_at IS NULL;
  END IF;
END
$$;

COMMIT;
