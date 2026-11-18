-- 删除已停用的官方媒体价表。
--
-- 背景：2026-11-18 起官方模型目录只承载对话模型，图片/视频/音频价格只在供应商模型
-- 的“供应商单位价格”里维护。ai_official_model_price 的数据已由
-- 2026-11-18-ai-official-model-chat-only.sql 清空，后端代码不再读写该表。
--
-- 本迁移可重复执行；表不存在时直接跳过。

BEGIN;

DROP TABLE IF EXISTS public.ai_official_model_price;

DO $$
BEGIN
    IF to_regclass('public.ai_official_model_price') IS NOT NULL THEN
        RAISE EXCEPTION 'ai_official_model_price still exists after drop';
    END IF;
END $$;

COMMIT;
