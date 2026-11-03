-- Admin AI menu/action seed. Execute only after all six AI pages are deployed.
SET LOCAL lock_timeout='2s';
SET LOCAL statement_timeout='30s';
CREATE TEMP TABLE _ai_menu_seed_expected(code varchar(128) PRIMARY KEY, menu_type varchar(16) NOT NULL, parent_code varchar(128), path varchar(255), component_path varchar(255), i18n_key varchar(128), is_hidden smallint NOT NULL) ON COMMIT DROP;
INSERT INTO _ai_menu_seed_expected(code,menu_type,parent_code,path,component_path,i18n_key,is_hidden) VALUES
('ai','directory',NULL,NULL,NULL,'navigation.ai',0),
('ai:provider:view','page','ai','/ai/provider','ai/provider','navigation.aiProvider',0),
('ai:officialModel:view','page','ai','/ai/officialModel','ai/officialModel','navigation.aiOfficialModel',0),
('ai:agent:view','page','ai','/ai/agent','ai/agent','navigation.aiAgent',0),
('ai:tool:view','page','ai','/ai/tool','ai/tool','navigation.aiTool',0),
('ai:run:view','page','ai','/ai/run','ai/run','navigation.aiRun',0),
('ai:chat:view','page','ai','/ai/chat','ai/chat','navigation.aiChat',0),
('ai:provider:list','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:detail','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:create','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:update','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:status','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:delete','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:test','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:sync','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:syncPrice','action','ai:provider:view',NULL,NULL,NULL,1),('ai:provider:updatePrice','action','ai:provider:view',NULL,NULL,NULL,1),
('ai:officialModel:list','action','ai:officialModel:view',NULL,NULL,NULL,1),('ai:officialModel:detail','action','ai:officialModel:view',NULL,NULL,NULL,1),('ai:officialModel:create','action','ai:officialModel:view',NULL,NULL,NULL,1),('ai:officialModel:update','action','ai:officialModel:view',NULL,NULL,NULL,1),('ai:officialModel:status','action','ai:officialModel:view',NULL,NULL,NULL,1),
('ai:agent:list','action','ai:agent:view',NULL,NULL,NULL,1),('ai:agent:detail','action','ai:agent:view',NULL,NULL,NULL,1),('ai:agent:create','action','ai:agent:view',NULL,NULL,NULL,1),('ai:agent:update','action','ai:agent:view',NULL,NULL,NULL,1),('ai:agent:status','action','ai:agent:view',NULL,NULL,NULL,1),('ai:agent:delete','action','ai:agent:view',NULL,NULL,NULL,1),('ai:agent:switchBinding','action','ai:agent:view',NULL,NULL,NULL,1),
('ai:tool:list','action','ai:tool:view',NULL,NULL,NULL,1),('ai:tool:detail','action','ai:tool:view',NULL,NULL,NULL,1),('ai:tool:create','action','ai:tool:view',NULL,NULL,NULL,1),('ai:tool:update','action','ai:tool:view',NULL,NULL,NULL,1),('ai:tool:status','action','ai:tool:view',NULL,NULL,NULL,1),('ai:tool:delete','action','ai:tool:view',NULL,NULL,NULL,1),
('ai:run:list','action','ai:run:view',NULL,NULL,NULL,1),('ai:run:detail','action','ai:run:view',NULL,NULL,NULL,1),
('ai:chat:generate','action','ai:chat:view',NULL,NULL,NULL,1),('ai:chat:list','action','ai:chat:view',NULL,NULL,NULL,1),('ai:chat:detail','action','ai:chat:view',NULL,NULL,NULL,1),('ai:chat:create','action','ai:chat:view',NULL,NULL,NULL,1),('ai:chat:update','action','ai:chat:view',NULL,NULL,NULL,1),('ai:chat:delete','action','ai:chat:view',NULL,NULL,NULL,1),('ai:chat:cancel','action','ai:chat:view',NULL,NULL,NULL,1),('ai:chat:retryTransfer','action','ai:chat:view',NULL,NULL,NULL,1);
DO $seed$
DECLARE v_platform_id bigint; v_platform_version bigint; v_count integer; v_expected integer; v_parent_id bigint; item record; actual record; v_inserted integer:=0;
BEGIN
 SELECT id,menu_version INTO v_platform_id,v_platform_version FROM permission_auth_platform WHERE code='admin' AND deleted_at IS NULL AND status=1 FOR UPDATE;
 IF v_platform_id IS NULL THEN RAISE EXCEPTION 'active Admin auth platform is missing'; END IF;
 SELECT count(*) INTO v_expected FROM _ai_menu_seed_expected;
 SELECT count(*) INTO v_count FROM permission_menu WHERE platform_id=v_platform_id AND deleted_at IS NULL AND (code='ai' OR code LIKE 'ai:%');
 IF v_count NOT IN(0,v_expected) THEN RAISE EXCEPTION 'AI menu is partial or has unexpected nodes'; END IF;
 FOR item IN SELECT * FROM _ai_menu_seed_expected ORDER BY CASE menu_type WHEN 'directory' THEN 0 WHEN 'page' THEN 1 ELSE 2 END,code LOOP
  v_parent_id:=NULL;
  IF item.parent_code IS NOT NULL THEN
   SELECT id INTO v_parent_id FROM permission_menu WHERE platform_id=v_platform_id AND code=item.parent_code AND deleted_at IS NULL;
   IF v_parent_id IS NULL THEN RAISE EXCEPTION 'AI menu parent missing'; END IF;
  END IF;
  IF NOT EXISTS(SELECT 1 FROM permission_menu WHERE platform_id=v_platform_id AND code=item.code AND deleted_at IS NULL) THEN
   INSERT INTO permission_menu(platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,status,is_hidden,created_at,updated_at)
   VALUES(v_platform_id,v_parent_id,item.menu_type,
    CASE item.code WHEN 'ai' THEN 'AI' WHEN 'ai:provider:view' THEN '供应商配置' WHEN 'ai:officialModel:view' THEN '官方模型' WHEN 'ai:agent:view' THEN '智能体配置' WHEN 'ai:tool:view' THEN '工具管理' WHEN 'ai:run:view' THEN '运行监控' WHEN 'ai:chat:view' THEN 'AI 工作台' ELSE item.code END,
    item.code,item.i18n_key,item.path,item.component_path,NULL,900,1,item.is_hidden,now(),now());
   v_inserted:=v_inserted+1;
  END IF;
  SELECT * INTO actual FROM permission_menu WHERE platform_id=v_platform_id AND code=item.code AND deleted_at IS NULL;
  IF actual.id IS NULL OR actual.parent_id IS DISTINCT FROM v_parent_id OR actual.menu_type IS DISTINCT FROM item.menu_type
   OR actual.path IS DISTINCT FROM item.path OR actual.component_path IS DISTINCT FROM item.component_path OR actual.i18n_key IS DISTINCT FROM item.i18n_key
   OR actual.is_hidden IS DISTINCT FROM item.is_hidden OR actual.status<>1 OR (item.menu_type='action' AND actual.icon IS NOT NULL) THEN
   RAISE EXCEPTION 'AI menu definition drift: %',item.code;
  END IF;
 END LOOP;
 IF v_inserted>0 THEN
  IF v_inserted<>v_expected OR v_platform_version=9223372036854775807 THEN RAISE EXCEPTION 'AI menu partial insertion or version overflow'; END IF;
  UPDATE permission_auth_platform SET menu_version=menu_version+1,updated_at=now() WHERE id=v_platform_id AND menu_version=v_platform_version;
  IF NOT FOUND THEN RAISE EXCEPTION 'Admin menu version changed'; END IF;
 END IF;
END $seed$;
