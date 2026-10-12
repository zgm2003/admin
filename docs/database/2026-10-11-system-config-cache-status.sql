-- 配置缓存状态命名与菜单契约迁移。
-- 原位重命名 generation 表；generation、outbox 记录、菜单 ID 和角色授权均保留。
-- 幂等：重复执行不推进 generation/outbox/menu_version，也不修改已符合目标的时间戳。

BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
  old_table REGCLASS := to_regclass('system_config_cache_generation');
  new_table REGCLASS := to_regclass('system_config_cache_state');
BEGIN
  IF old_table IS NOT NULL AND new_table IS NOT NULL THEN
    RAISE EXCEPTION 'both system_config_cache_generation and system_config_cache_state exist';
  ELSIF old_table IS NOT NULL THEN
    ALTER TABLE system_config_cache_generation RENAME TO system_config_cache_state;
  ELSIF new_table IS NULL THEN
    RAISE EXCEPTION 'system_config_cache_generation/state table is missing';
  END IF;
END $$;

-- Constraint names are part of the schema contract. PostgreSQL updates the FK target
-- during the table rename; the explicit rename makes the target and dump auditable.
DO $$
DECLARE
  old_name TEXT;
  new_name TEXT;
  table_oid OID := 'system_config_cache_state'::REGCLASS;
  constraint_pair TEXT[][] := ARRAY[
    ARRAY['pk_system_config_cache_generation', 'pk_system_config_cache_state'],
    ARRAY['ck_system_config_cache_generation_namespace', 'ck_system_config_cache_state_namespace'],
    ARRAY['ck_system_config_cache_generation_scope_key', 'ck_system_config_cache_state_scope_key'],
    ARRAY['ck_system_config_cache_generation_value', 'ck_system_config_cache_state_value']
  ];
  pair TEXT[];
BEGIN
  FOREACH pair SLICE 1 IN ARRAY constraint_pair LOOP
    old_name := pair[1];
    new_name := pair[2];
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = table_oid AND conname = old_name)
       AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = table_oid AND conname = new_name) THEN
      RAISE EXCEPTION 'both constraints % and % exist', old_name, new_name;
    ELSIF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = table_oid AND conname = old_name) THEN
      EXECUTE format('ALTER TABLE system_config_cache_state RENAME CONSTRAINT %I TO %I', old_name, new_name);
    END IF;
  END LOOP;

  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_config_cache_outbox'::REGCLASS AND conname = 'fk_system_config_cache_outbox_generation')
     AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_config_cache_outbox'::REGCLASS AND conname = 'fk_system_config_cache_outbox_state') THEN
    RAISE EXCEPTION 'both outbox generation/state FK constraints exist';
  ELSIF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_config_cache_outbox'::REGCLASS AND conname = 'fk_system_config_cache_outbox_generation') THEN
    ALTER TABLE system_config_cache_outbox
      RENAME CONSTRAINT fk_system_config_cache_outbox_generation TO fk_system_config_cache_outbox_state;
  END IF;
END $$;

CREATE TEMP TABLE tmp_config_cache_status_platforms (platform_id BIGINT PRIMARY KEY) ON COMMIT DROP;

DO $$
DECLARE
  platform_row RECORD;
  page_id BIGINT;
  old_page_id BIGINT;
  target_page_id BIGINT;
  action_id BIGINT;
  old_action_id BIGINT;
  target_action_id BIGINT;
  system_menu_id BIGINT;
  changed BOOLEAN;
  affected INTEGER;
BEGIN
  FOR platform_row IN
    SELECT id FROM permission_auth_platform
    WHERE deleted_at IS NULL AND is_enabled = 1
    ORDER BY id
    FOR UPDATE
  LOOP
    SELECT id INTO system_menu_id
    FROM permission_menu
    WHERE platform_id = platform_row.id
      AND code = 'system' AND menu_type = 'directory' AND deleted_at IS NULL
    FOR UPDATE;
    IF system_menu_id IS NULL THEN
      CONTINUE;
    END IF;

    IF (SELECT count(*) FROM permission_menu WHERE platform_id = platform_row.id AND code = 'system:cacheGeneration:view' AND deleted_at IS NULL) > 1
       OR (SELECT count(*) FROM permission_menu WHERE platform_id = platform_row.id AND code = 'system:configCacheStatus:view' AND deleted_at IS NULL) > 1 THEN
      RAISE EXCEPTION 'duplicate active config cache status page rows on platform %', platform_row.id;
    END IF;
    IF (SELECT count(*) FROM permission_menu WHERE platform_id = platform_row.id AND code = 'system:cacheGeneration:list' AND deleted_at IS NULL) > 1
       OR (SELECT count(*) FROM permission_menu WHERE platform_id = platform_row.id AND code = 'system:configCacheStatus:list' AND deleted_at IS NULL) > 1 THEN
      RAISE EXCEPTION 'duplicate active config cache status action rows on platform %', platform_row.id;
    END IF;

    SELECT id INTO old_page_id FROM permission_menu
    WHERE platform_id = platform_row.id AND code = 'system:cacheGeneration:view' AND deleted_at IS NULL;
    SELECT id INTO target_page_id FROM permission_menu
    WHERE platform_id = platform_row.id AND code = 'system:configCacheStatus:view' AND deleted_at IS NULL;
    IF old_page_id IS NOT NULL AND target_page_id IS NOT NULL AND old_page_id <> target_page_id THEN
      RAISE EXCEPTION 'old and target config cache status page IDs differ on platform %', platform_row.id;
    END IF;
    page_id := COALESCE(target_page_id, old_page_id);
    IF page_id IS NULL THEN
      IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = platform_row.id AND code = 'system:cacheGeneration:list' AND deleted_at IS NULL)
         OR EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = platform_row.id AND code = 'system:configCacheStatus:list' AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'config cache status list action is orphaned on platform %', platform_row.id;
      END IF;
      CONTINUE;
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id = page_id AND menu_type <> 'page') THEN
      RAISE EXCEPTION 'config cache status page code is not a page on platform %', platform_row.id;
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id = page_id AND parent_id IS DISTINCT FROM system_menu_id) THEN
      RAISE EXCEPTION 'config cache status page is not under system directory on platform %', platform_row.id;
    END IF;

    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = platform_row.id AND deleted_at IS NULL AND path = '/system/configCacheStatus' AND id <> page_id) THEN
      RAISE EXCEPTION 'target config cache status path is occupied on platform %', platform_row.id;
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id = platform_row.id AND deleted_at IS NULL AND component_path = 'system/configCacheStatus' AND id <> page_id) THEN
      RAISE EXCEPTION 'target config cache status component path is occupied on platform %', platform_row.id;
    END IF;

    changed := FALSE;
    UPDATE permission_menu
    SET code = 'system:configCacheStatus:view',
        name = '配置缓存状态',
        i18n_key = 'navigation.systemConfigCacheStatus',
        path = '/system/configCacheStatus',
        component_path = 'system/configCacheStatus',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = page_id
      AND (code IS DISTINCT FROM 'system:configCacheStatus:view'
        OR name IS DISTINCT FROM '配置缓存状态'
        OR i18n_key IS DISTINCT FROM 'navigation.systemConfigCacheStatus'
        OR path IS DISTINCT FROM '/system/configCacheStatus'
        OR component_path IS DISTINCT FROM 'system/configCacheStatus');
    GET DIAGNOSTICS affected = ROW_COUNT;
    changed := affected > 0;

    SELECT id INTO old_action_id FROM permission_menu
    WHERE platform_id = platform_row.id AND code = 'system:cacheGeneration:list' AND deleted_at IS NULL;
    SELECT id INTO target_action_id FROM permission_menu
    WHERE platform_id = platform_row.id AND code = 'system:configCacheStatus:list' AND deleted_at IS NULL;
    IF old_action_id IS NOT NULL AND target_action_id IS NOT NULL AND old_action_id <> target_action_id THEN
      RAISE EXCEPTION 'old and target config cache status action IDs differ on platform %', platform_row.id;
    END IF;
    action_id := COALESCE(target_action_id, old_action_id);
    IF action_id IS NULL THEN
      RAISE EXCEPTION 'config cache status page has no list action on platform %', platform_row.id;
    END IF;
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id = action_id AND menu_type <> 'action') THEN
      RAISE EXCEPTION 'config cache status list code is not an action on platform %', platform_row.id;
    END IF;

    UPDATE permission_menu
    SET parent_id = page_id,
        name = '查看配置缓存状态',
        code = 'system:configCacheStatus:list',
        i18n_key = NULL,
        path = NULL,
        component_path = NULL,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = action_id
      AND (parent_id IS DISTINCT FROM page_id
        OR name IS DISTINCT FROM '查看配置缓存状态'
        OR code IS DISTINCT FROM 'system:configCacheStatus:list'
        OR i18n_key IS NOT NULL OR path IS NOT NULL OR component_path IS NOT NULL);
    GET DIAGNOSTICS affected = ROW_COUNT;
    changed := changed OR affected > 0;

    IF changed THEN
      INSERT INTO tmp_config_cache_status_platforms(platform_id)
      VALUES (platform_row.id) ON CONFLICT DO NOTHING;
    END IF;
  END LOOP;
END $$;

UPDATE permission_auth_platform AS platform
SET menu_version = platform.menu_version + 1,
    updated_at = CURRENT_TIMESTAMP
FROM tmp_config_cache_status_platforms AS changed
WHERE platform.id = changed.platform_id;

COMMIT;
