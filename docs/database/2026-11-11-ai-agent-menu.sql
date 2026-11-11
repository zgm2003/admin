-- AI Agent 配置页面、动作权限和菜单。
-- 在调用方事务中执行；不自动授权普通角色，不删除 Redis key。

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
    platform permission_auth_platform%ROWTYPE;
    ai_directory_id bigint;
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

    SELECT id INTO ai_directory_id FROM permission_menu
    WHERE platform_id = platform.id AND code = 'ai' AND menu_type = 'directory'
      AND status = 1 AND deleted_at IS NULL;
    IF ai_directory_id IS NULL THEN
        RAISE EXCEPTION 'AI directory is missing';
    END IF;

    SELECT id INTO page_id FROM permission_menu
    WHERE platform_id = platform.id AND code = 'ai:agent:view' AND menu_type = 'page'
      AND status = 1 AND deleted_at IS NULL;
    IF page_id IS NULL THEN
        IF EXISTS (
            SELECT 1 FROM permission_menu
            WHERE platform_id = platform.id AND code = 'ai:agent:view' AND deleted_at IS NULL
        ) THEN
            RAISE EXCEPTION 'agent page code conflicts with an existing menu';
        END IF;
        INSERT INTO permission_menu
            (platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,status,is_hidden,remark)
        VALUES
            (platform.id,ai_directory_id,'page','智能体','ai:agent:view','navigation.aiAgent','/ai/agent','ai/agent','lucide:bot',30,1,0,NULL)
        RETURNING id INTO page_id;
        changed := true;
    ELSE
        SELECT * INTO entry FROM permission_menu WHERE id = page_id;
        IF entry.parent_id <> ai_directory_id OR entry.name <> '智能体'
           OR entry.i18n_key <> 'navigation.aiAgent' OR entry.path <> '/ai/agent'
           OR entry.component_path <> 'ai/agent' OR entry.status <> 1 OR entry.is_hidden <> 0 THEN
            RAISE EXCEPTION 'agent page shape conflict';
        END IF;
    END IF;

    FOR action_code, action_name, action_sort IN
        SELECT * FROM (VALUES
            ('ai:agent:list','查看智能体列表',10),
            ('ai:agent:detail','查看智能体详情',20),
            ('ai:agent:create','新增智能体',30),
            ('ai:agent:update','编辑智能体',40),
            ('ai:agent:status','启停智能体',50),
            ('ai:agent:delete','删除智能体',60)
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
                RAISE EXCEPTION 'agent action % conflicts with an existing menu', action_code;
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
                RAISE EXCEPTION 'agent action % shape conflict', action_code;
            END IF;
        END IF;
    END LOOP;

    IF changed THEN
        UPDATE permission_auth_platform
        SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
        WHERE id = platform.id;
    END IF;
END $$;
