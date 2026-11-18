-- AI 官方模型收敛为对话模型。
--
-- 定价边界（方案 A）：
--   官方模型目录只承载对话模型的官方 token 价，供应商倍率只作用于这些对话模型。
--   图片、视频、音频模型不进入官方目录，其价格只在供应商模型的“供应商单位价格”里维护。
--
-- 本迁移一次完成三件事，可重复执行：
--   1. 解除供应商模型对媒体官方模型的引用（媒体模型本就不要求官方映射）；
--   2. 删除官方视频价表里属于媒体官方模型的行；
--   3. 删除官方目录中的图片、视频、音频模型。
--
-- 迁移后 ai_official_model_price 为空表，后端代码已不再读写它；保留表结构用于审计，
-- 待确认无回滚需求后再单独清理。

BEGIN;

DO $$
DECLARE
    media_models int;
    media_prices int;
BEGIN
    SELECT count(*) INTO media_models FROM ai_official_model WHERE model_kind <> 1;
    SELECT count(*) INTO media_prices FROM ai_official_model_price;
    RAISE NOTICE 'before: media official models=%, official media prices=%', media_models, media_prices;
END $$;

UPDATE ai_provider_model
SET official_model_id = NULL, updated_at = now()
WHERE model_kind <> 1
  AND official_model_id IN (SELECT id FROM ai_official_model WHERE model_kind <> 1);

DELETE FROM ai_official_model_price
WHERE official_model_id IN (SELECT id FROM ai_official_model WHERE model_kind <> 1);

DELETE FROM ai_official_model WHERE model_kind <> 1;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM ai_official_model WHERE model_kind <> 1) THEN
        RAISE EXCEPTION 'media official models still exist after cleanup';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM ai_official_model_price price
        JOIN ai_official_model model ON model.id = price.official_model_id
        WHERE model.model_kind <> 1
    ) THEN
        RAISE EXCEPTION 'official media prices still exist after cleanup';
    END IF;
END $$;

COMMENT ON TABLE ai_official_model IS '官方对话模型目录；只维护对话模型官方 token 价，图片/视频/音频价格由供应商模型单位价维护';
COMMENT ON COLUMN ai_official_model.model_kind IS '模型类型固定为 1=对话；媒体模型不进入官方目录';
COMMENT ON TABLE ai_official_model_price IS '历史官方媒体价表，已停用；媒体价格由供应商模型单位价维护';

DO $$
DECLARE
    chat_models int;
BEGIN
    SELECT count(*) INTO chat_models FROM ai_official_model WHERE model_kind = 1;
    RAISE NOTICE 'after: chat official models=%', chat_models;
END $$;

COMMIT;
