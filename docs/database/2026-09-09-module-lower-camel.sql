BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

LOCK TABLE permission_auth_platform, permission_menu IN ACCESS EXCLUSIVE MODE;

CREATE TEMP TABLE module_identity_map (
    old_code text PRIMARY KEY,
    new_code text NOT NULL UNIQUE
) ON COMMIT DROP;

INSERT INTO module_identity_map (old_code, new_code) VALUES
('permission:authplatform:view', 'permission:authPlatform:view'),
('permission:authplatform:list', 'permission:authPlatform:list'),
('permission:authplatform:create', 'permission:authPlatform:create'),
('permission:authplatform:update', 'permission:authPlatform:update'),
('permission:authplatform:status', 'permission:authPlatform:status'),
('permission:authplatform:delete', 'permission:authPlatform:delete'),
('system:operationlog:view', 'system:operationLog:view'),
('system:operationlog:list', 'system:operationLog:list'),
('user:loginlog:view', 'user:loginLog:view'),
('user:loginlog:list', 'user:loginLog:list'),
('storage:cosconfig:create', 'storage:cosConfig:create'),
('storage:cosconfig:update', 'storage:cosConfig:update'),
('storage:cosconfig:status', 'storage:cosConfig:status'),
('storage:cosconfig:test', 'storage:cosConfig:test'),
('storage:cosconfig:delete', 'storage:cosConfig:delete'),
('storage:uploadrule:create', 'storage:uploadRule:create'),
('storage:uploadrule:update', 'storage:uploadRule:update'),
('storage:uploadrule:status', 'storage:uploadRule:status'),
('storage:uploadrule:delete', 'storage:uploadRule:delete');

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM permission_menu legacy
        JOIN module_identity_map mapping ON mapping.old_code = legacy.code
        JOIN permission_menu canonical
          ON canonical.platform_id = legacy.platform_id
         AND canonical.code = mapping.new_code
         AND canonical.id <> legacy.id
         AND canonical.deleted_at IS NULL
        WHERE legacy.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'canonical lower camel permission code already exists';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM permission_menu legacy
        JOIN permission_menu canonical
          ON canonical.platform_id = legacy.platform_id
         AND canonical.id <> legacy.id
         AND canonical.deleted_at IS NULL
         AND canonical.path = CASE legacy.path
             WHEN '/permission/authplatform' THEN '/permission/authPlatform'
             WHEN '/system/operationlog' THEN '/system/operationLog'
             WHEN '/user/loginlog' THEN '/user/loginLog'
         END
        WHERE legacy.deleted_at IS NULL
          AND legacy.path IN ('/permission/authplatform', '/system/operationlog', '/user/loginlog')
    ) THEN
        RAISE EXCEPTION 'canonical lower camel page path already exists';
    END IF;
END
$$;

CREATE TEMP TABLE affected_module_platform (
    platform_id bigint PRIMARY KEY
) ON COMMIT DROP;

INSERT INTO affected_module_platform (platform_id)
SELECT DISTINCT menu.platform_id
FROM permission_menu menu
LEFT JOIN module_identity_map mapping ON mapping.old_code = menu.code
WHERE menu.deleted_at IS NULL
  AND (
      mapping.old_code IS NOT NULL
      OR menu.component_path IN ('permission/authplatform', 'system/operationlog', 'user/loginlog')
      OR menu.i18n_key IN ('navigation.permissionAuthplatform', 'navigation.systemOperationlog', 'navigation.userLoginlog')
  );

UPDATE permission_menu menu
SET code = mapping.new_code,
    updated_at = CURRENT_TIMESTAMP
FROM module_identity_map mapping
WHERE menu.code = mapping.old_code
  AND menu.deleted_at IS NULL;

UPDATE permission_menu
SET path = CASE path
        WHEN '/permission/authplatform' THEN '/permission/authPlatform'
        WHEN '/system/operationlog' THEN '/system/operationLog'
        WHEN '/user/loginlog' THEN '/user/loginLog'
        ELSE path
    END,
    component_path = CASE component_path
        WHEN 'permission/authplatform' THEN 'permission/authPlatform'
        WHEN 'system/operationlog' THEN 'system/operationLog'
        WHEN 'user/loginlog' THEN 'user/loginLog'
        ELSE component_path
    END,
    i18n_key = CASE i18n_key
        WHEN 'navigation.permissionAuthplatform' THEN 'navigation.permissionAuthPlatform'
        WHEN 'navigation.systemOperationlog' THEN 'navigation.systemOperationLog'
        WHEN 'navigation.userLoginlog' THEN 'navigation.userLoginLog'
        ELSE i18n_key
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE deleted_at IS NULL
  AND (
      path IN ('/permission/authplatform', '/system/operationlog', '/user/loginlog')
      OR component_path IN ('permission/authplatform', 'system/operationlog', 'user/loginlog')
      OR i18n_key IN ('navigation.permissionAuthplatform', 'navigation.systemOperationlog', 'navigation.userLoginlog')
  );

UPDATE permission_auth_platform platform
SET menu_version = platform.menu_version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE platform.id IN (SELECT platform_id FROM affected_module_platform);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM permission_menu
        WHERE deleted_at IS NULL
          AND (
              code IN (SELECT old_code FROM module_identity_map)
              OR component_path IN ('permission/authplatform', 'system/operationlog', 'user/loginlog')
              OR i18n_key IN ('navigation.permissionAuthplatform', 'navigation.systemOperationlog', 'navigation.userLoginlog')
          )
    ) THEN
        RAISE EXCEPTION 'legacy module identity remains after migration';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM permission_menu
        WHERE deleted_at IS NULL
          AND menu_type = 'page'
          AND component_path IN ('permission/authPlatform', 'system/operationLog', 'user/loginLog')
          AND path <> '/' || component_path
    ) THEN
        RAISE EXCEPTION 'lower camel page path and component path diverged';
    END IF;
END
$$;

COMMIT;
