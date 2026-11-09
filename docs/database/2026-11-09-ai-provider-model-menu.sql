-- AI 供应商模型管理动作权限。
-- 在调用方事务中执行；不自动授权普通角色，不删除 Redis key。

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
    platform permission_auth_platform%ROWTYPE;
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

    SELECT id INTO page_id FROM permission_menu
    WHERE platform_id = platform.id AND code = 'ai:provider:view' AND menu_type = 'page'
      AND status = 1 AND deleted_at IS NULL;
    IF page_id IS NULL THEN
        RAISE EXCEPTION 'provider page is missing';
    END IF;

    FOR action_code, action_name, action_sort IN
        SELECT * FROM (VALUES
            ('ai:provider:model:list','查看供应商模型',70),
            ('ai:provider:model:sync','同步供应商模型',80),
            ('ai:provider:model:update','配置供应商模型',90)
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
                RAISE EXCEPTION 'provider model action % conflicts with an existing menu', action_code;
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
                RAISE EXCEPTION 'provider model action % shape conflict', action_code;
            END IF;
        END IF;
    END LOOP;

    IF changed THEN
        UPDATE permission_auth_platform
        SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
        WHERE id = platform.id;
    END IF;
END $$;
