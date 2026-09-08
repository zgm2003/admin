-- Enable verified-email registration, preserve the least-privilege default
-- role, and move the hidden profile page to the route root. Stop old writers.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
LOCK TABLE permission_auth_platform, permission_role, permission_menu,
  permission_role_menu, permission_user_role, permission_access_version
  IN SHARE ROW EXCLUSIVE MODE;

CREATE TEMP TABLE registration_context (
  admin_platform_id bigint NOT NULL,
  registered_role_id bigint NOT NULL,
  profile_page_id bigint NOT NULL
) ON COMMIT DROP;

INSERT INTO registration_context
SELECT platform.id, role.id, profile.id
FROM permission_auth_platform platform
CROSS JOIN permission_role role
CROSS JOIN permission_menu profile
WHERE platform.code = 'admin' AND platform.is_builtin = 1
  AND platform.is_enabled = 1 AND platform.deleted_at IS NULL
  AND role.code = 'registered_user' AND role.is_default = 1
  AND role.is_enabled = 1 AND role.deleted_at IS NULL
  AND profile.platform_id = platform.id
  AND profile.code = 'user:profile:view' AND profile.menu_type = 'page'
  AND profile.is_enabled = 1 AND profile.is_hidden = 1
  AND profile.path = '/user/profile' AND profile.component_path = 'user/profile'
  AND profile.deleted_at IS NULL;

CREATE TEMP TABLE registration_grants (
  code text PRIMARY KEY,
  menu_id bigint UNIQUE NOT NULL
) ON COMMIT DROP;

INSERT INTO registration_grants
SELECT menu.code, menu.id
FROM permission_menu menu, registration_context context
WHERE menu.platform_id = context.admin_platform_id
  AND menu.code IN (
    'user:profile:view', 'user:profile:detail', 'user:profile:update',
    'user:password:update', 'storage:object:upload'
  )
  AND menu.is_enabled = 1 AND menu.deleted_at IS NULL
  AND (
    (menu.code = 'user:profile:view' AND menu.menu_type = 'page')
    OR (menu.code <> 'user:profile:view' AND menu.menu_type = 'action')
  );

DO $$
BEGIN
  IF (SELECT count(*) FROM registration_context) <> 1 THEN
    RAISE EXCEPTION 'Expected one enabled Admin platform, registered_user role and hidden profile page';
  END IF;
  IF (SELECT count(*) FROM permission_role WHERE is_default = 1 AND is_enabled = 1 AND deleted_at IS NULL) <> 1 THEN
    RAISE EXCEPTION 'Expected exactly one enabled default role';
  END IF;
  IF (SELECT count(*) FROM registration_grants) <> 5 THEN
    RAISE EXCEPTION 'Default registration grants are incomplete';
  END IF;
  IF EXISTS (
    SELECT 1 FROM permission_menu action
    CROSS JOIN registration_context context
    WHERE action.code IN ('user:profile:detail','user:profile:update','user:password:update')
      AND action.platform_id = context.admin_platform_id
      AND action.parent_id IS DISTINCT FROM context.profile_page_id
  ) THEN
    RAISE EXCEPTION 'Profile action parent is invalid';
  END IF;
END $$;

CREATE TEMP TABLE registration_changed (kind text PRIMARY KEY) ON COMMIT DROP;

WITH updated AS (
  UPDATE permission_auth_platform platform
  SET allow_register = 1,
      policy_version = policy_version + 1,
      updated_at = CURRENT_TIMESTAMP
  FROM registration_context context
  WHERE platform.id = context.admin_platform_id AND platform.allow_register = 0
  RETURNING platform.id
)
INSERT INTO registration_changed
SELECT 'platform' FROM updated LIMIT 1 ON CONFLICT DO NOTHING;

WITH moved AS (
  UPDATE permission_menu profile
  SET parent_id = NULL, updated_at = CURRENT_TIMESTAMP
  FROM registration_context context
  WHERE profile.id = context.profile_page_id AND profile.parent_id IS NOT NULL
  RETURNING profile.id
)
INSERT INTO registration_changed
SELECT 'menu' FROM moved LIMIT 1 ON CONFLICT DO NOTHING;

WITH removed AS (
  UPDATE permission_role_menu grant_row
  SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
  FROM registration_context context
  WHERE grant_row.role_id = context.registered_role_id
    AND grant_row.deleted_at IS NULL
    AND NOT EXISTS (SELECT 1 FROM registration_grants wanted WHERE wanted.menu_id = grant_row.menu_id)
  RETURNING grant_row.id
)
INSERT INTO registration_changed
SELECT 'role_grants' FROM removed LIMIT 1 ON CONFLICT DO NOTHING;

WITH inserted AS (
  INSERT INTO permission_role_menu (role_id, menu_id, created_at, updated_at)
  SELECT context.registered_role_id, wanted.menu_id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
  FROM registration_context context
  CROSS JOIN registration_grants wanted
  WHERE NOT EXISTS (
    SELECT 1 FROM permission_role_menu current
    WHERE current.role_id = context.registered_role_id
      AND current.menu_id = wanted.menu_id AND current.deleted_at IS NULL
  )
  RETURNING id
)
INSERT INTO registration_changed
SELECT 'role_grants' FROM inserted LIMIT 1 ON CONFLICT DO NOTHING;

UPDATE permission_role role
SET updated_at = CURRENT_TIMESTAMP
FROM registration_context context
WHERE role.id = context.registered_role_id
  AND EXISTS (SELECT 1 FROM registration_changed WHERE kind = 'role_grants');

UPDATE permission_access_version version
SET version = version.version + 1, updated_at = CURRENT_TIMESTAMP
WHERE EXISTS (SELECT 1 FROM registration_changed WHERE kind = 'role_grants')
  AND version.user_id IN (
    SELECT user_role.user_id
    FROM permission_user_role user_role, registration_context context
    WHERE user_role.role_id = context.registered_role_id AND user_role.deleted_at IS NULL
  );

UPDATE permission_auth_platform platform
SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
FROM registration_context context
WHERE platform.id = context.admin_platform_id
  AND EXISTS (SELECT 1 FROM registration_changed WHERE kind = 'menu');

COMMIT;
