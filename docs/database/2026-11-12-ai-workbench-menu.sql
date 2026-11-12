-- AI 工作台与运行日志菜单、动作权限及默认角色授权。
-- 普通用户获得自己的工作台和运行日志；ai:run:viewAll 只授予超级管理员。
-- 可重复执行。

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
    v_platform_id bigint := 1;
    v_ai_id bigint;
    v_workbench_id bigint;
    v_run_id bigint;
    v_menu_id bigint;
BEGIN
    SELECT id INTO v_ai_id
    FROM public.permission_menu
    WHERE platform_id = v_platform_id AND code = 'ai' AND deleted_at IS NULL;

    IF v_ai_id IS NULL THEN
        RAISE EXCEPTION 'AI directory menu is missing';
    END IF;

    INSERT INTO public.permission_menu (
        parent_id, menu_type, code, i18n_key, path, component_path, icon,
        sort_order, status, is_hidden, created_at, updated_at, platform_id, name, remark
    ) VALUES (
        v_ai_id, 'page', 'ai:workbench:view', 'navigation.aiWorkbench',
        '/ai/workbench', 'ai/workbench', 'lucide:message-square-more',
        40, 1, 0, now(), now(), v_platform_id, 'AI 工作台', '文本对话工作台'
    )
    ON CONFLICT (platform_id, code) WHERE deleted_at IS NULL DO NOTHING
    RETURNING id INTO v_workbench_id;

    IF v_workbench_id IS NULL THEN
        SELECT id INTO v_workbench_id
        FROM public.permission_menu
        WHERE platform_id = v_platform_id AND code = 'ai:workbench:view' AND deleted_at IS NULL;
    END IF;

    INSERT INTO public.permission_menu (
        parent_id, menu_type, code, i18n_key, path, component_path, icon,
        sort_order, status, is_hidden, created_at, updated_at, platform_id, name, remark
    ) VALUES (
        v_ai_id, 'page', 'ai:run:view', 'navigation.aiRun',
        '/ai/run', 'ai/run', 'lucide:scroll-text',
        50, 1, 0, now(), now(), v_platform_id, 'AI 运行日志', 'AI 对话运行记录'
    )
    ON CONFLICT (platform_id, code) WHERE deleted_at IS NULL DO NOTHING
    RETURNING id INTO v_run_id;

    IF v_run_id IS NULL THEN
        SELECT id INTO v_run_id
        FROM public.permission_menu
        WHERE platform_id = v_platform_id AND code = 'ai:run:view' AND deleted_at IS NULL;
    END IF;

    INSERT INTO public.permission_menu (
        parent_id, menu_type, code, name, sort_order, status, is_hidden, created_at, updated_at, platform_id
    )
    SELECT v_workbench_id, 'action', action_code, action_name, action_sort, 1, 1, now(), now(), v_platform_id
    FROM (VALUES
        ('ai:workbench:list', '查询工作台', 10),
        ('ai:workbench:detail', '读取会话', 20),
        ('ai:workbench:create', '新建会话', 30),
        ('ai:workbench:update', '重命名会话', 40),
        ('ai:workbench:delete', '删除会话', 50),
        ('ai:workbench:send', '发送消息和停止生成', 60)
    ) AS actions(action_code, action_name, action_sort)
    WHERE NOT EXISTS (
        SELECT 1 FROM public.permission_menu
        WHERE platform_id = v_platform_id AND code = actions.action_code AND deleted_at IS NULL
    );

    INSERT INTO public.permission_menu (
        parent_id, menu_type, code, name, sort_order, status, is_hidden, created_at, updated_at, platform_id
    )
    SELECT v_run_id, 'action', action_code, action_name, action_sort, 1, 1, now(), now(), v_platform_id
    FROM (VALUES
        ('ai:run:list', '查询自己的运行日志', 10),
        ('ai:run:detail', '读取自己的运行详情', 20),
        ('ai:run:viewAll', '查看全员运行日志', 30)
    ) AS actions(action_code, action_name, action_sort)
    WHERE NOT EXISTS (
        SELECT 1 FROM public.permission_menu
        WHERE platform_id = v_platform_id AND code = actions.action_code AND deleted_at IS NULL
    );

    -- 普通用户：工作台 + 自己的运行日志。
    FOR v_menu_id IN
        SELECT id FROM public.permission_menu
        WHERE platform_id = v_platform_id
          AND deleted_at IS NULL
          AND code IN (
              'ai:workbench:view', 'ai:workbench:list', 'ai:workbench:detail',
              'ai:workbench:create', 'ai:workbench:update', 'ai:workbench:delete',
              'ai:workbench:send',
              'ai:run:view', 'ai:run:list', 'ai:run:detail'
          )
    LOOP
        INSERT INTO public.permission_role_menu (role_id, menu_id, created_at, updated_at)
        SELECT 42, v_menu_id, now(), now()
        WHERE NOT EXISTS (
            SELECT 1 FROM public.permission_role_menu
            WHERE role_id = 42 AND menu_id = v_menu_id AND deleted_at IS NULL
        );
    END LOOP;

    -- 超级管理员：显式拥有全量运行日志动作。
    FOR v_menu_id IN
        SELECT id FROM public.permission_menu
        WHERE platform_id = v_platform_id
          AND deleted_at IS NULL
          AND code IN (
              'ai:workbench:view', 'ai:run:view',
              'ai:workbench:list', 'ai:workbench:detail', 'ai:workbench:create',
              'ai:workbench:update', 'ai:workbench:delete', 'ai:workbench:send',
              'ai:run:list', 'ai:run:detail', 'ai:run:viewAll'
          )
    LOOP
        INSERT INTO public.permission_role_menu (role_id, menu_id, created_at, updated_at)
        SELECT 41, v_menu_id, now(), now()
        WHERE NOT EXISTS (
            SELECT 1 FROM public.permission_role_menu
            WHERE role_id = 41 AND menu_id = v_menu_id AND deleted_at IS NULL
        );
    END LOOP;
END $$;
