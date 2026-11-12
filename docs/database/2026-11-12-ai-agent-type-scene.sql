-- AI Agent 类型与业务场景。

-- 由维护者执行；API/Worker 启动不执行 DDL。
-- 可重复执行：字段和约束均先判断/先删除旧约束再重建。
-- scene 是后端固定枚举：1=对话，2=工具生成，3=生图，4=视频生成，5=音频生成。

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE public.ai_agent
    ADD COLUMN IF NOT EXISTS agent_type smallint NOT NULL DEFAULT 1;

ALTER TABLE public.ai_agent
    DROP CONSTRAINT IF EXISTS ai_agent_agent_type_check;
ALTER TABLE public.ai_agent
    ADD CONSTRAINT ai_agent_agent_type_check CHECK (agent_type IN (1, 2, 3, 4));

-- 早期 free-text 版本先把 scene 转成 smallint；没有该列时直接新增。
DO $$
DECLARE
    scene_type text;
BEGIN
    SELECT data_type INTO scene_type
    FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'ai_agent' AND column_name = 'scene';

    IF scene_type IN ('character varying', 'text') THEN
        ALTER TABLE public.ai_agent DROP CONSTRAINT IF EXISTS ai_agent_scene_check;
        ALTER TABLE public.ai_agent ADD COLUMN IF NOT EXISTS scene_code smallint;
        UPDATE public.ai_agent
        SET scene_code = CASE lower(btrim(scene))
            WHEN 'general' THEN 1
            WHEN 'chat' THEN 1
            WHEN 'conversation' THEN 1
            WHEN '对话' THEN 1
            WHEN 'tool' THEN 2
            WHEN 'tool_generation' THEN 2
            WHEN '工具生成' THEN 2
            WHEN 'image' THEN 3
            WHEN 'image_generation' THEN 3
            WHEN '生图' THEN 3
            WHEN 'video' THEN 4
            WHEN 'video_generation' THEN 4
            WHEN '视频生成' THEN 4
            WHEN 'audio' THEN 5
            WHEN 'audio_generation' THEN 5
            WHEN '音频生成' THEN 5
            ELSE NULL
        END;
        IF EXISTS (SELECT 1 FROM public.ai_agent WHERE scene_code IS NULL) THEN
            RAISE EXCEPTION 'ai_agent contains an unknown free-text scene; map it to a scene code before migration';
        END IF;
        ALTER TABLE public.ai_agent DROP COLUMN scene;
        ALTER TABLE public.ai_agent RENAME COLUMN scene_code TO scene;
        ALTER TABLE public.ai_agent ALTER COLUMN scene SET DEFAULT 1;
        ALTER TABLE public.ai_agent ALTER COLUMN scene SET NOT NULL;
    ELSIF scene_type IS NULL THEN
        ALTER TABLE public.ai_agent
            ADD COLUMN scene smallint NOT NULL DEFAULT 1;
    END IF;
END $$;

ALTER TABLE public.ai_agent
    DROP CONSTRAINT IF EXISTS ai_agent_scene_check;
ALTER TABLE public.ai_agent
    ADD CONSTRAINT ai_agent_scene_check CHECK (scene IN (1, 2, 3, 4, 5));

COMMENT ON TABLE public.ai_agent IS '可配置智能体；按类型绑定一个已启用且配置完整的模型，并记录业务场景';
COMMENT ON COLUMN public.ai_agent.agent_type IS '智能体类型：1=对话，2=图片，3=视频，4=音频；必须与绑定模型的 model_kind 一致';
COMMENT ON COLUMN public.ai_agent.scene IS '业务场景枚举：1=对话，2=工具生成，3=生图，4=视频生成，5=音频生成';
COMMENT ON COLUMN public.ai_agent.provider_model_id IS '绑定的供应商模型；运行时不能使用已停用、已删除或类型不匹配的模型';

CREATE INDEX IF NOT EXISTS ai_agent_type_status_id_idx
    ON public.ai_agent (agent_type, status, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS ai_agent_scene_status_id_idx
    ON public.ai_agent (scene, status, id DESC)
    WHERE deleted_at IS NULL;
