-- AI / 供应商配置 菜单与动作权限。
-- 在调用方事务中执行；不自动授权普通角色，不删除 Redis key。
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
        RAISE EXCEPTION 'AI directory is missing';
    END IF;

    SELECT id INTO page_id FROM permission_menu
    WHERE platform_id = platform.id AND code = 'ai:provider:view' AND menu_type = 'page'
      AND status = 1 AND deleted_at IS NULL AND parent_id = directory_id;
    IF page_id IS NULL THEN
        IF EXISTS (
            SELECT 1 FROM permission_menu
            WHERE platform_id = platform.id AND deleted_at IS NULL
              AND (code = 'ai:provider:view' OR path = '/ai/provider' OR component_path = 'ai/provider')
        ) THEN
            RAISE EXCEPTION 'provider page conflicts with an existing menu';
        END IF;
        INSERT INTO permission_menu
            (platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,status,is_hidden,remark)
        VALUES
            (platform.id,directory_id,'page','供应商配置','ai:provider:view','navigation.aiProvider',
             '/ai/provider','ai/provider','lucide:server',20,1,0,'AI 供应商通道与密钥配置')
        RETURNING id INTO page_id;
        changed := true;
    ELSE
        SELECT * INTO entry FROM permission_menu WHERE id = page_id;
        IF entry.name <> '供应商配置' OR entry.i18n_key IS DISTINCT FROM 'navigation.aiProvider'
           OR entry.path IS DISTINCT FROM '/ai/provider' OR entry.component_path IS DISTINCT FROM 'ai/provider'
           OR entry.icon IS DISTINCT FROM 'lucide:server' OR entry.sort_order <> 20 OR entry.is_hidden <> 0 THEN
            RAISE EXCEPTION 'provider page shape conflict';
        END IF;
    END IF;

    FOR action_code, action_name, action_sort IN
        SELECT * FROM (VALUES
            ('ai:provider:list','查看供应商列表',10),
            ('ai:provider:detail','查看供应商详情',20),
            ('ai:provider:create','新增供应商',30),
            ('ai:provider:update','编辑供应商',40),
            ('ai:provider:status','启停供应商',50),
            ('ai:provider:delete','删除供应商',60)
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
                RAISE EXCEPTION 'provider action % conflicts with an existing menu', action_code;
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
                RAISE EXCEPTION 'provider action % shape conflict', action_code;
            END IF;
        END IF;
    END LOOP;

    IF changed THEN
        UPDATE permission_auth_platform
        SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
        WHERE id = platform.id;
    END IF;
END $$;
