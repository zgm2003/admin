-- Hide personal payment pages from the sidebar. Their page permissions remain
-- facts used by the Wallet tabs and their existing routes/actions are intact.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
SELECT pg_advisory_xact_lock(20261017, 402);

DO $migration$
DECLARE
  admin_id bigint;
  changed boolean := false;
BEGIN
  SELECT id INTO admin_id
    FROM permission_auth_platform
   WHERE code = 'admin' AND is_enabled = 1 AND deleted_at IS NULL;
  IF admin_id IS NULL THEN
    RAISE EXCEPTION 'active Admin platform is required';
  END IF;

  UPDATE permission_menu
     SET is_hidden = 1, updated_at = CURRENT_TIMESTAMP
   WHERE platform_id = admin_id
     AND code IN ('payment:recharge:view', 'payment:myWallet:view')
     AND menu_type = 'page'
     AND is_hidden <> 1
     AND deleted_at IS NULL;
  changed := FOUND;

  IF changed THEN
    UPDATE permission_auth_platform
       SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
     WHERE id = admin_id;
  END IF;
END
$migration$;
