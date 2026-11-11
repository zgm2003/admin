-- AI / 供应商配置：新增“测试连接”动作权限。
-- 测试会使用供应商密钥发起上游 hello 请求，因此不复用只读权限。
-- 在调用方事务中执行；不自动授权普通角色，不删除 Redis key。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
    platform permission_auth_platform%ROWTYPE;
    page_id bigint;
    action_id bigint;
    entry permission_menu%ROWTYPE;
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

    SELECT id INTO action_id FROM permission_menu
    WHERE platform_id = platform.id AND code = 'ai:provider:test' AND menu_type = 'action'
      AND status = 1 AND deleted_at IS NULL AND parent_id = page_id;
    IF action_id IS NULL THEN
        IF EXISTS (
            SELECT 1 FROM permission_menu
            WHERE platform_id = platform.id AND code = 'ai:provider:test' AND deleted_at IS NULL
        ) THEN
            RAISE EXCEPTION 'provider test action conflicts with an existing menu';
        END IF;
        INSERT INTO permission_menu
            (platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,status,is_hidden,remark)
        VALUES
            (platform.id,page_id,'action','测试供应商连接','ai:provider:test',NULL,NULL,NULL,NULL,65,1,1,NULL)
        RETURNING id INTO action_id;
        changed := true;
    ELSE
        SELECT * INTO entry FROM permission_menu WHERE id = action_id;
        IF entry.name <> '测试供应商连接' OR entry.is_hidden <> 1
           OR entry.path IS NOT NULL OR entry.component_path IS NOT NULL OR entry.icon IS NOT NULL THEN
            RAISE EXCEPTION 'provider test action shape conflict';
        END IF;
    END IF;

    IF changed THEN
        UPDATE permission_auth_platform
        SET menu_version = menu_version + 1, updated_at = CURRENT_TIMESTAMP
        WHERE id = platform.id;
    END IF;
END $$;
