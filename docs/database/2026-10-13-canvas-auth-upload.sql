-- Forward migration. The runner holds tokenized menu/user leases and executes
-- this file in one PostgreSQL transaction. Never run at API/Worker startup.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
LOCK TABLE permission_auth_platform, permission_menu, permission_role,
  permission_role_menu, permission_user_role, permission_access_version,
  user_account, storage_cos_config, storage_upload_rule, storage_upload_rule_code IN SHARE ROW EXCLUSIVE MODE;

DO $migration$
DECLARE
  admin_id BIGINT;
  canvas_id BIGINT;
  registered_role_id BIGINT;
  page_id BIGINT;
  action_id BIGINT;
  source_rule_id BIGINT;
  target_rule_id BIGINT;
  changed BOOLEAN := FALSE;
  grant_changed BOOLEAN := FALSE;
  source_count INTEGER;
  target_count INTEGER;
  source_codes TEXT[];
  target_codes TEXT[];
BEGIN
  SELECT id INTO admin_id
    FROM permission_auth_platform
   WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL;
  IF admin_id IS NULL OR (SELECT count(*) FROM permission_auth_platform WHERE code='admin' AND is_enabled=1 AND deleted_at IS NULL) <> 1 THEN
    RAISE EXCEPTION 'exactly one active Admin platform is required';
  END IF;
  SELECT id INTO canvas_id
    FROM permission_auth_platform
   WHERE code='canvas' AND is_enabled=1 AND deleted_at IS NULL;
  IF canvas_id IS NULL OR (SELECT count(*) FROM permission_auth_platform WHERE code='canvas' AND is_enabled=1 AND deleted_at IS NULL) <> 1 THEN
    RAISE EXCEPTION 'exactly one active Canvas platform is required';
  END IF;
  SELECT id INTO registered_role_id FROM permission_role WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL;
  IF registered_role_id IS NULL OR (SELECT count(*) FROM permission_role WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL) <> 1 THEN
    RAISE EXCEPTION 'exactly one active registered_user role is required';
  END IF;

  IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=canvas_id AND path='/canvas/workspace' AND code<>'canvas:workspace:view' AND deleted_at IS NULL)
     OR EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=canvas_id AND component_path='canvas/workspace' AND code<>'canvas:workspace:view' AND deleted_at IS NULL) THEN
    RAISE EXCEPTION 'Canvas workspace route conflicts with an existing menu';
  END IF;
  IF (SELECT count(*) FROM permission_menu WHERE platform_id=canvas_id AND code='canvas:workspace:view' AND deleted_at IS NULL) > 1
     OR (SELECT count(*) FROM permission_menu WHERE platform_id=canvas_id AND code='storage:object:upload' AND deleted_at IS NULL) > 1 THEN
    RAISE EXCEPTION 'Canvas capability menu codes are duplicated';
  END IF;
  SELECT id INTO page_id FROM permission_menu WHERE platform_id=canvas_id AND code='canvas:workspace:view' AND deleted_at IS NULL;
  IF page_id IS NULL THEN
    INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,sort_order,is_enabled,is_hidden,created_at,updated_at)
    VALUES(canvas_id,NULL,'page','Canvas 工作台','canvas:workspace:view','navigation.canvasWorkspace','/canvas/workspace','canvas/workspace',10,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
    RETURNING id INTO page_id;
    changed := TRUE;
  ELSE
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id=page_id AND (menu_type<>'page' OR parent_id IS NOT NULL OR path IS DISTINCT FROM '/canvas/workspace' OR component_path IS DISTINCT FROM 'canvas/workspace' OR i18n_key IS DISTINCT FROM 'navigation.canvasWorkspace' OR is_enabled<>1 OR is_hidden<>1)) THEN
      RAISE EXCEPTION 'Canvas workspace page has incompatible shape';
    END IF;
  END IF;

  SELECT id INTO action_id FROM permission_menu WHERE platform_id=canvas_id AND code='storage:object:upload' AND deleted_at IS NULL;
  IF action_id IS NULL THEN
    IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=canvas_id AND code='storage:object:upload' AND deleted_at IS NULL) THEN
      RAISE EXCEPTION 'Canvas upload action has an incompatible existing node';
    END IF;
    INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,sort_order,is_enabled,is_hidden,created_at,updated_at)
    VALUES(canvas_id,page_id,'action','上传对象','storage:object:upload',10,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
    RETURNING id INTO action_id;
    changed := TRUE;
  ELSE
    IF EXISTS (SELECT 1 FROM permission_menu WHERE id=action_id AND (menu_type<>'action' OR parent_id IS DISTINCT FROM page_id OR i18n_key IS NOT NULL OR path IS NOT NULL OR component_path IS NOT NULL OR icon IS NOT NULL OR is_enabled<>1 OR is_hidden<>1)) THEN
      RAISE EXCEPTION 'Canvas upload action has incompatible shape';
    END IF;
  END IF;

  IF EXISTS (SELECT 1 FROM permission_role_menu grant_row WHERE grant_row.role_id=registered_role_id AND grant_row.menu_id=page_id AND grant_row.deleted_at IS NULL) THEN
    RAISE EXCEPTION 'registered_user must not receive the Canvas workspace page';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM permission_role_menu grant_row WHERE grant_row.role_id=registered_role_id AND grant_row.menu_id=action_id AND grant_row.deleted_at IS NULL) THEN
    INSERT INTO permission_role_menu(role_id,menu_id,created_at,updated_at) VALUES(registered_role_id,action_id,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
    grant_changed := TRUE;
  END IF;
  IF grant_changed THEN
    UPDATE permission_role SET updated_at=CURRENT_TIMESTAMP WHERE id=registered_role_id;
    IF EXISTS (
      SELECT 1
        FROM permission_user_role ur
        JOIN user_account u ON u.id=ur.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL
        LEFT JOIN permission_access_version av ON av.user_id=u.id
       WHERE ur.role_id=registered_role_id AND ur.deleted_at IS NULL AND av.user_id IS NULL
    ) THEN RAISE EXCEPTION 'an active registered_user lacks an access version'; END IF;
    UPDATE permission_access_version
       SET version=version+1, updated_at=CURRENT_TIMESTAMP
     WHERE user_id IN (
       SELECT ur.user_id FROM permission_user_role ur
       JOIN user_account u ON u.id=ur.user_id AND u.is_enabled=1 AND u.deleted_at IS NULL
       WHERE ur.role_id=registered_role_id AND ur.deleted_at IS NULL
     );
  END IF;

  SELECT count(*), max(r.id), array_agg(c.code ORDER BY c.code)
    INTO source_count, source_rule_id, source_codes
    FROM storage_upload_rule r
    JOIN storage_upload_rule_code c ON c.rule_id=r.id AND c.deleted_at IS NULL AND c.code='file'
   WHERE r.platform_id=admin_id AND r.is_enabled=1 AND r.deleted_at IS NULL;
  IF source_count <> 1 OR source_codes <> ARRAY['file'] THEN
    RAISE EXCEPTION 'active Admin file upload rule is required';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM storage_upload_rule r
    JOIN storage_cos_config cfg ON cfg.id=r.cos_config_id AND cfg.is_enabled=1 AND cfg.deleted_at IS NULL
    JOIN storage_cos_config_version physical ON physical.cos_config_id=cfg.id AND physical.version=cfg.current_version
    WHERE r.id=source_rule_id AND (r.access_mode='private' OR NULLIF(btrim(physical.bucket_domain),'') IS NOT NULL)
  ) THEN RAISE EXCEPTION 'Admin file rule COS route is unavailable'; END IF;

  SELECT count(*), max(r.id) INTO target_count, target_rule_id
    FROM storage_upload_rule r JOIN storage_upload_rule_code c ON c.rule_id=r.id AND c.deleted_at IS NULL AND c.code='canvas'
   WHERE r.platform_id=canvas_id AND r.deleted_at IS NULL;
  IF target_count > 1 THEN RAISE EXCEPTION 'Canvas upload rule is duplicated'; END IF;
  IF target_count = 1 THEN
    SELECT array_agg(c.code ORDER BY c.code) INTO target_codes FROM storage_upload_rule_code c WHERE c.rule_id=target_rule_id AND c.deleted_at IS NULL;
    IF target_codes IS DISTINCT FROM ARRAY['canvas'] OR EXISTS (
      SELECT 1 FROM storage_upload_rule target WHERE target.id=target_rule_id AND target.is_enabled<>1
    ) THEN RAISE EXCEPTION 'Canvas upload rule has incompatible shape'; END IF;
  ELSE
    IF EXISTS (SELECT 1 FROM storage_upload_rule WHERE platform_id=canvas_id AND is_enabled=1 AND deleted_at IS NULL) THEN
      RAISE EXCEPTION 'an unrelated active Canvas upload rule already exists';
    END IF;
    INSERT INTO storage_upload_rule(platform_id,name,cos_config_id,max_file_size_bytes,allowed_extensions,allowed_mime_types,access_mode,is_enabled,remark,created_at,updated_at)
    SELECT canvas_id,'Canvas 上传规则',cos_config_id,max_file_size_bytes,allowed_extensions,allowed_mime_types,access_mode,1,remark,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
      FROM storage_upload_rule WHERE id=source_rule_id
    RETURNING id INTO target_rule_id;
    INSERT INTO storage_upload_rule_code(rule_id,code,created_at,updated_at) VALUES(target_rule_id,'canvas',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
  END IF;

  IF changed THEN
    UPDATE permission_auth_platform SET menu_version=menu_version+1, updated_at=CURRENT_TIMESTAMP WHERE id=canvas_id;
  END IF;
END $migration$;

DO $verify$
BEGIN
  IF (SELECT count(*) FROM permission_menu page JOIN permission_auth_platform p ON p.id=page.platform_id WHERE p.code='canvas' AND page.code='canvas:workspace:view' AND page.menu_type='page' AND page.parent_id IS NULL AND page.path='/canvas/workspace' AND page.component_path='canvas/workspace' AND page.i18n_key='navigation.canvasWorkspace' AND page.is_enabled=1 AND page.is_hidden=1 AND page.deleted_at IS NULL) <> 1 THEN RAISE EXCEPTION 'Canvas workspace page verification failed'; END IF;
  IF (SELECT count(*) FROM permission_menu action JOIN permission_menu page ON page.id=action.parent_id JOIN permission_auth_platform p ON p.id=action.platform_id WHERE p.code='canvas' AND page.code='canvas:workspace:view' AND action.code='storage:object:upload' AND action.menu_type='action' AND action.is_enabled=1 AND action.is_hidden=1 AND action.i18n_key IS NULL AND action.path IS NULL AND action.component_path IS NULL AND action.icon IS NULL AND action.deleted_at IS NULL) <> 1 THEN RAISE EXCEPTION 'Canvas upload action verification failed'; END IF;
  IF (SELECT count(*) FROM storage_upload_rule r JOIN permission_auth_platform p ON p.id=r.platform_id JOIN storage_upload_rule_code c ON c.rule_id=r.id AND c.code='canvas' AND c.deleted_at IS NULL WHERE p.code='canvas' AND r.is_enabled=1 AND r.deleted_at IS NULL) <> 1 THEN RAISE EXCEPTION 'Canvas upload rule verification failed'; END IF;
END $verify$;
