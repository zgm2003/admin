-- Menu-only forward migration. Execute inside one maintenance transaction.
-- No role/user grants, upload configuration or Redis operations.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
LOCK TABLE permission_auth_platform, permission_menu IN SHARE ROW EXCLUSIVE MODE;

DO $migration$
DECLARE
  canvas_id BIGINT;
  page_id BIGINT;
  action_id BIGINT;
  changed BOOLEAN := FALSE;
  action RECORD;
BEGIN
  SELECT id INTO canvas_id FROM permission_auth_platform
    WHERE code='canvas' AND is_enabled=1 AND deleted_at IS NULL;
  IF canvas_id IS NULL OR (SELECT count(*) FROM permission_auth_platform WHERE code='canvas' AND is_enabled=1 AND deleted_at IS NULL)<>1 THEN
    RAISE EXCEPTION 'exactly one active Canvas platform is required';
  END IF;
  IF EXISTS (SELECT 1 FROM permission_menu WHERE platform_id=canvas_id AND deleted_at IS NULL
    AND (path='/user/profile' OR component_path='user/profile') AND code<>'user:profile:view') THEN
    RAISE EXCEPTION 'Canvas profile route conflicts with an existing menu';
  END IF;
  SELECT id INTO page_id FROM permission_menu WHERE platform_id=canvas_id AND code='user:profile:view' AND deleted_at IS NULL;
  IF page_id IS NULL THEN
    INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,sort_order,is_enabled,is_hidden,remark,created_at,updated_at)
    VALUES(canvas_id,NULL,'page','个人中心','user:profile:view','layout.user.profile','/user/profile','user/profile',90,1,1,
      '允许进入个人中心页面；读取、保存和账号安全操作分别授权。',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP) RETURNING id INTO page_id;
    changed := TRUE;
  ELSIF EXISTS (SELECT 1 FROM permission_menu WHERE id=page_id AND
    (menu_type<>'page' OR parent_id IS NOT NULL OR path IS DISTINCT FROM '/user/profile'
    OR component_path IS DISTINCT FROM 'user/profile' OR i18n_key IS DISTINCT FROM 'layout.user.profile' OR is_hidden<>1 OR is_enabled<>1)) THEN
    RAISE EXCEPTION 'Canvas profile page has incompatible shape';
  END IF;

  FOR action IN SELECT * FROM (VALUES
    ('user:profile:detail','读取个人资料',5),
    ('user:profile:update','修改资料',10),
    ('user:password:update','修改密码',20),
    ('user:email:update','绑定或换绑邮箱',30),
    ('user:phone:update','绑定或换绑手机',40)
  ) AS actions(code,name,sort_order) LOOP
    action_id := NULL;
    SELECT id INTO action_id FROM permission_menu WHERE platform_id=canvas_id AND code=action.code AND deleted_at IS NULL;
    IF action_id IS NULL THEN
      INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,sort_order,is_enabled,is_hidden,remark,created_at,updated_at)
      VALUES(canvas_id,page_id,'action',action.name,action.code,action.sort_order,1,1,
        '控制“'||action.name||'”对应的具体操作。',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
      changed := TRUE;
    ELSIF EXISTS (SELECT 1 FROM permission_menu WHERE id=action_id AND
      (menu_type<>'action' OR parent_id IS DISTINCT FROM page_id OR is_hidden<>1 OR is_enabled<>1
      OR path IS NOT NULL OR component_path IS NOT NULL OR i18n_key IS NOT NULL OR icon IS NOT NULL)) THEN
      RAISE EXCEPTION 'Canvas profile action % has incompatible shape', action.code;
    END IF;
  END LOOP;
  IF changed THEN
    UPDATE permission_auth_platform SET menu_version=menu_version+1,updated_at=CURRENT_TIMESTAMP WHERE id=canvas_id;
  END IF;
END $migration$;
