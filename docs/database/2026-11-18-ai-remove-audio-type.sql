-- 移除音频模型类型与音频生成场景。
--
-- 背景：音频计费维度不统一（TTS 常按字符/token，ASR 按时长），当前产品决定
-- 不再把音频纳入 AI 模块，只保留对话、图片、视频三类。
--
-- 约束调整：
--   ai_provider_model.model_kind  仅允许 1 对话 / 2 图片 / 3 视频
--   ai_provider_model.price_unit  仅允许 1 百万 Token / 2 张 / 3 秒
--   ai_agent.agent_type           仅允许 1 对话 / 2 图片 / 3 视频
--   ai_agent.scene                仅允许 1 对话 / 2 工具生成 / 3 生图 / 4 视频生成
--
-- 先清理存量音频数据，再收紧约束；可重复执行。
-- 若音频智能体仍被会话或运行引用，删除会触发外键错误并整体回滚，需要先人工处理。

BEGIN;

DO $$
DECLARE
    agent_rows int;
    model_rows int;
BEGIN
    SELECT count(*) INTO agent_rows FROM ai_agent WHERE agent_type = 4 OR scene = 5;
    SELECT count(*) INTO model_rows FROM ai_provider_model WHERE model_kind = 4;
    RAISE NOTICE 'audio rows before cleanup: agents=%, provider_models=%', agent_rows, model_rows;
END $$;

DELETE FROM ai_agent WHERE agent_type = 4 OR scene = 5;
DELETE FROM ai_provider_model WHERE model_kind = 4;

-- 修正历史数据：对话模型必须走官方 Token 计价；图片/视频必须使用对应单位。
-- 同步逻辑早期给未匹配模型写过“供应商单位价 + 次”，这里一并纠正。
UPDATE ai_provider_model
SET status = 0, updated_at = now()
WHERE model_kind = 1 AND pricing_mode = 2 AND status = 1 AND official_model_id IS NULL;

UPDATE ai_provider_model
SET pricing_mode = 1, price_unit = 1, updated_at = now()
WHERE model_kind = 1 AND (pricing_mode <> 1 OR price_unit <> 1);

UPDATE ai_provider_model
SET pricing_mode = 2, updated_at = now()
WHERE model_kind IN (2, 3) AND pricing_mode <> 2;

UPDATE ai_provider_model
SET price_unit = 2, updated_at = now()
WHERE model_kind = 2 AND price_unit <> 2;

UPDATE ai_provider_model
SET price_unit = 3, updated_at = now()
WHERE model_kind = 3 AND price_unit <> 3;

ALTER TABLE ai_provider_model DROP CONSTRAINT IF EXISTS ai_provider_model_model_kind_check;
ALTER TABLE ai_provider_model ADD CONSTRAINT ai_provider_model_model_kind_check
    CHECK (model_kind IN (1, 2, 3));

ALTER TABLE ai_provider_model DROP CONSTRAINT IF EXISTS ai_provider_model_price_unit_check;
ALTER TABLE ai_provider_model ADD CONSTRAINT ai_provider_model_price_unit_check
    CHECK (price_unit IN (1, 2, 3));

ALTER TABLE ai_agent DROP CONSTRAINT IF EXISTS ai_agent_agent_type_check;
ALTER TABLE ai_agent ADD CONSTRAINT ai_agent_agent_type_check
    CHECK (agent_type IN (1, 2, 3));

ALTER TABLE ai_agent DROP CONSTRAINT IF EXISTS ai_agent_scene_check;
ALTER TABLE ai_agent ADD CONSTRAINT ai_agent_scene_check
    CHECK (scene IN (1, 2, 3, 4));

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM ai_agent WHERE agent_type = 4 OR scene = 5) THEN
        RAISE EXCEPTION 'audio agent rows remain after cleanup';
    END IF;
    IF EXISTS (SELECT 1 FROM ai_provider_model WHERE model_kind = 4) THEN
        RAISE EXCEPTION 'audio provider models remain after cleanup';
    END IF;
    IF EXISTS (SELECT 1 FROM ai_provider_model WHERE price_unit NOT IN (1, 2, 3)) THEN
        RAISE EXCEPTION 'audio price units remain after cleanup';
    END IF;
END $$;

COMMIT;
