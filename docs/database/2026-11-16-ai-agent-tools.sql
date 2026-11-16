-- AI 工具：智能体挂载关系、自定义 HTTP 工具字段和完整工具权限。
-- 可重复执行。

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE public.ai_tool
    ADD COLUMN IF NOT EXISTS api_name varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS kind smallint NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS parameters jsonb NOT NULL DEFAULT '{"type":"object","properties":{}}'::jsonb,
    ADD COLUMN IF NOT EXISTS method varchar(8) NOT NULL DEFAULT 'POST',
    ADD COLUMN IF NOT EXISTS url varchar(2000) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS timeout_ms integer NOT NULL DEFAULT 10000;

UPDATE public.ai_tool
SET api_name = regexp_replace(replace(code, '.', '_'), '[^a-zA-Z0-9_-]', '_', 'g')
WHERE api_name = '';

UPDATE public.ai_tool
SET kind = 1,
    parameters = CASE code
        WHEN 'system.user_count' THEN '{"type":"object","properties":{}}'::jsonb
        WHEN 'system.server_overview' THEN '{"type":"object","properties":{}}'::jsonb
        ELSE parameters
    END
WHERE code IN ('system.user_count', 'system.server_overview');

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ai_tool_kind_check' AND conrelid = 'public.ai_tool'::regclass
    ) THEN
        ALTER TABLE public.ai_tool
            ADD CONSTRAINT ai_tool_kind_check CHECK (kind IN (1, 2));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ai_tool_method_check' AND conrelid = 'public.ai_tool'::regclass
    ) THEN
        ALTER TABLE public.ai_tool
            ADD CONSTRAINT ai_tool_method_check CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE'));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ai_tool_timeout_check' AND conrelid = 'public.ai_tool'::regclass
    ) THEN
        ALTER TABLE public.ai_tool
            ADD CONSTRAINT ai_tool_timeout_check CHECK (timeout_ms BETWEEN 1000 AND 60000);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ai_tool_url_check' AND conrelid = 'public.ai_tool'::regclass
    ) THEN
        ALTER TABLE public.ai_tool
            ADD CONSTRAINT ai_tool_url_check CHECK (length(url) <= 2000);
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS ux_ai_tool_api_name_active
    ON public.ai_tool (api_name)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS public.ai_agent_tool (
    agent_id bigint NOT NULL REFERENCES public.ai_agent (id) ON DELETE CASCADE,
    tool_id bigint NOT NULL REFERENCES public.ai_tool (id) ON DELETE CASCADE,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, tool_id)
);

CREATE INDEX IF NOT EXISTS ix_ai_agent_tool_tool
    ON public.ai_agent_tool (tool_id);

DO $$
DECLARE
    v_platform_id bigint := 1;
    v_page_id bigint;
    v_menu_id bigint;
BEGIN
    SELECT id INTO v_page_id
    FROM public.permission_menu
    WHERE platform_id = v_platform_id AND code = 'ai:tool:view' AND deleted_at IS NULL;

    IF v_page_id IS NULL THEN
        RAISE EXCEPTION 'AI tool page menu is missing';
    END IF;

    INSERT INTO public.permission_menu (
        parent_id, menu_type, code, name, sort_order, status, is_hidden, created_at, updated_at, platform_id
    )
    SELECT v_page_id, 'action', action_code, action_name, action_sort, 1, 1, now(), now(), v_platform_id
    FROM (VALUES
        ('ai:tool:detail', '查看 AI 工具详情', 30),
        ('ai:tool:create', '新增 AI 工具', 40),
        ('ai:tool:update', '编辑 AI 工具', 50),
        ('ai:tool:delete', '删除 AI 工具', 60)
    ) AS actions(action_code, action_name, action_sort)
    WHERE NOT EXISTS (
        SELECT 1 FROM public.permission_menu
        WHERE platform_id = v_platform_id AND code = actions.action_code AND deleted_at IS NULL
    );

    FOR v_menu_id IN
        SELECT id FROM public.permission_menu
        WHERE platform_id = v_platform_id
          AND deleted_at IS NULL
          AND code IN (
              'ai:tool:view', 'ai:tool:list', 'ai:tool:status',
              'ai:tool:detail', 'ai:tool:create', 'ai:tool:update', 'ai:tool:delete'
          )
    LOOP
        INSERT INTO public.permission_role_menu (role_id, menu_id, created_at, updated_at)
        SELECT 41, v_menu_id, now(), now()
        WHERE NOT EXISTS (
            SELECT 1 FROM public.permission_role_menu
            WHERE role_id = 41 AND menu_id = v_menu_id AND deleted_at IS NULL
        );
    END LOOP;

    UPDATE public.permission_auth_platform
    SET menu_version = menu_version + 1, updated_at = now()
    WHERE id = v_platform_id;
END $$;
