-- AI / 官方模型 菜单与动作权限。
-- 在调用方事务中执行；不自动授权普通角色，不删除 Redis key。
-- 提交后必须将 admin.menu_version 发布到 permission/state.MenuStore 再启动服务。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
    platform permission_auth_platform%ROWTYPE;
    directory_id bigint;
    page_id bigint;
    action_id bigint;
    entry permission_menu%ROWTYPE;
    action_code text;
    action_name text;
    action_sort integer;
    changed boolean := false;
BEGIN
    LOCK TABLE permission_menu IN SHARE ROW EXCLUSIVE MODE;
    SELECT * INTO STRICT platform FROM permission_auth_platform WHERE code = 'admin' FOR UPDATE;
    IF platform.deleted_at IS NOT NULL OR platform.status <> 1 OR platform.menu_version < 1 THEN
        RAISE EXCEPTION 'admin platform is unavailable';
    END IF;

    SELECT id INTO directory_id FROM permission_menu
    WHERE platform_id = platform.id AND code = 'ai' AND menu_type = 'directory'
      AND status = 1 AND deleted_at IS NULL AND parent_id IS NULL;
    IF directory_id IS NULL THEN
        IF EXISTS (
            SELECT 1 FROM permission_menu
            WHERE platform_id = platform.id AND deleted_at IS NULL
              AND (code = 'ai' OR (parent_id IS NULL AND name = 'AI'))
        ) THEN
            RAISE EXCEPTION 'AI directory conflicts with an existing menu';
        END IF;
        INSERT INTO permission_menu
            (platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,status,is_hidden,remark)
        VALUES
            (platform.id,NULL,'directory','AI','ai','navigation.ai',NULL,NULL,'lucide:bot',40,1,0,'AI 应用')
        RETURNING id INTO directory_id;
        changed := true;
    ELSE
        SELECT * INTO entry FROM permission_menu WHERE id = directory_id;
        IF entry.name <> 'AI' OR entry.i18n_key IS DISTINCT FROM 'navigation.ai'
           OR entry.icon IS DISTINCT FROM 'lucide:bot' OR entry.sort_order <> 40
           OR entry.path IS NOT NULL OR entry.component_path IS NOT NULL OR entry.is_hidden <> 0 THEN
            RAISE EXCEPTION 'AI directory shape conflict';
        END IF;
    END IF;

    SELECT id INTO page_id FROM permission_menu
    WHERE platform_id = platform.id AND code = 'ai:officialModel:view' AND menu_type = 'page'
      AND status = 1 AND deleted_at IS NULL AND parent_id = directory_id;
    IF page_id IS NULL THEN
        IF EXISTS (
            SELECT 1 FROM permission_menu
            WHERE platform_id = platform.id AND deleted_at IS NULL
              AND (code = 'ai:officialModel:view' OR path = '/ai/officialModel' OR component_path = 'ai/officialModel')
        ) THEN
            RAISE EXCEPTION 'official model page conflicts with an existing menu';
        END IF;
        INSERT INTO permission_menu
            (platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,status,is_hidden,remark)
        VALUES
            (platform.id,directory_id,'page','官方模型','ai:officialModel:view','navigation.aiOfficialModel',
             '/ai/officialModel','ai/officialModel','lucide:bot',10,1,0,'同步官方模型价格并维护人民币价')
        RETURNING id INTO page_id;
        changed := true;
    ELSE
        SELECT * INTO entry FROM permission_menu WHERE id = page_id;
        IF entry.name <> '官方模型' OR entry.i18n_key IS DISTINCT FROM 'navigation.aiOfficialModel'
           OR entry.path IS DISTINCT FROM '/ai/officialModel' OR entry.component_path IS DISTINCT FROM 'ai/officialModel'
           OR entry.icon IS DISTINCT FROM 'lucide:bot' OR entry.sort_order <> 10 OR entry.is_hidden <> 0 THEN
            RAISE EXCEPTION 'official model page shape conflict';
        END IF;
    END IF;

    FOR action_code, action_name, action_sort IN
        SELECT * FROM (VALUES
            ('ai:officialModel:list','查看官方模型',10),
            ('ai:officialModel:update','修改人民币价',20),
            ('ai:officialModel:sync','同步官方目录',30),
            ('ai:officialModel:reset','按官方价重置',40)
        ) AS actions(code,name,sort_order)
    LOOP
        SELECT id INTO action_id FROM permission_menu
        WHERE platform_id = platform.id AND code = action_code AND menu_type = 'action'
          AND status = 1 AND deleted_at IS NULL AND parent_id = page_id;
        IF action_id IS NULL THEN
            IF EXISTS (
                SELECT 1 FROM permission_menu
                WHERE platform_id = platform.id AND code = action_code AND deleted_at IS NULL
            ) THEN
                RAISE EXCEPTION 'official model action % conflicts with an existing menu', action_code;
            END IF;
            INSERT INTO permission_menu
                (platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,status,is_hidden,remark)
            VALUES
                (platform.id,page_id,'action',action_name,action_code,NULL,NULL,NULL,NULL,action_sort,1,1,NULL);
            changed := true;
        ELSE
            SELECT * INTO entry FROM permission_menu WHERE id = action_id;
            IF entry.name <> action_name OR entry.is_hidden <> 1
               OR entry.path IS NOT NULL OR entry.component_path IS NOT NULL OR entry.icon IS NOT NULL THEN
                RAISE EXCEPTION 'official model action % shape conflict', action_code;
            END IF;
        END IF;
    END LOOP;

    IF changed THEN
        UPDATE permission_auth_platform
        SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
        WHERE id = platform.id;
    END IF;
END $$;
