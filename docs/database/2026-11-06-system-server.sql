-- 由维护者在全部 API/Worker 停止后，通过 migrate-system-server runner 执行。
-- 在调用方事务中执行；不自动授权普通角色，不删除 Redis key。
-- 提交后必须将 admin.menu_version 发布到 permission/state.MenuStore 再启动服务。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
    platform permission_auth_platform%ROWTYPE;
    directory_id bigint;
    page_id bigint;
    changed boolean := false;
    entry permission_menu%ROWTYPE;
BEGIN
    LOCK TABLE permission_menu IN SHARE ROW EXCLUSIVE MODE;
    SELECT * INTO STRICT platform FROM permission_auth_platform WHERE code = 'admin' FOR UPDATE;
    IF platform.deleted_at IS NOT NULL OR platform.status <> 1 OR platform.menu_version < 1 THEN
        RAISE EXCEPTION 'admin platform is unavailable';
    END IF;
    SELECT id INTO STRICT directory_id FROM permission_menu
    WHERE platform_id = platform.id AND code = 'system' AND menu_type = 'directory'
      AND status = 1 AND deleted_at IS NULL AND parent_id IS NULL;

    FOR entry IN SELECT * FROM permission_menu WHERE platform_id = platform.id
        AND (code IN ('system:server:view', 'system:server:detail')
             OR path = '/system/server' OR component_path = 'system/server')
    LOOP
        IF entry.deleted_at IS NOT NULL OR entry.status <> 1 THEN
            RAISE EXCEPTION 'server menu conflicts with an inactive entry';
        END IF;
        IF entry.code = 'system:server:view' THEN
            IF page_id IS NOT NULL OR entry.menu_type <> 'page'
               OR entry.parent_id IS DISTINCT FROM directory_id
               OR entry.path IS DISTINCT FROM '/system/server'
               OR entry.component_path IS DISTINCT FROM 'system/server'
               OR entry.i18n_key IS DISTINCT FROM 'navigation.systemServer'
               OR entry.is_hidden <> 0 THEN
                RAISE EXCEPTION 'server page shape conflict';
            END IF;
            page_id := entry.id;
        ELSE
            RAISE EXCEPTION 'server path or component already belongs to another menu';
        END IF;
    END LOOP;

    IF page_id IS NULL THEN
        INSERT INTO permission_menu (platform_id,parent_id,menu_type,name,code,i18n_key,path,component_path,icon,sort_order,status,is_hidden)
        VALUES (platform.id,directory_id,'page','服务器概览','system:server:view','navigation.systemServer','/system/server','system/server','Monitor',60,1,0)
        RETURNING id INTO page_id;
        changed := true;
    END IF;
    IF changed THEN
        UPDATE permission_auth_platform SET menu_version=menu_version+1, updated_at=CURRENT_TIMESTAMP WHERE id=platform.id;
    END IF;
END $$;
