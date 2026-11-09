-- 官方模型厂商改用项目规范的 SMALLINT typed enum：
-- 1=OpenAI，2=Anthropic，3=Google。
-- 本迁移是 expand 阶段：新增 vendor 并回填；旧 vendor_code/vendor_name
-- 暂时保留为可空列，避免滚动部署时旧 API 写入失败。等新 API 稳定后，
-- 再执行 2026-11-09-ai-official-model-vendor-cleanup.sql 收缩旧列。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE ai_official_model ADD COLUMN IF NOT EXISTS vendor smallint;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'ai_official_model'
          AND column_name = 'vendor_code'
    ) THEN
        UPDATE ai_official_model
        SET vendor = CASE vendor_code
            WHEN 'openai' THEN 1
            WHEN 'anthropic' THEN 2
            WHEN 'google' THEN 3
        END
        WHERE vendor IS NULL;

        IF EXISTS (SELECT 1 FROM ai_official_model WHERE vendor IS NULL) THEN
            RAISE EXCEPTION 'ai_official_model contains an unsupported vendor_code';
        END IF;

        -- 新代码只写 vendor；旧列在收缩迁移前允许为空。
        ALTER TABLE ai_official_model ALTER COLUMN vendor_code DROP NOT NULL;
        ALTER TABLE ai_official_model ALTER COLUMN vendor_name DROP NOT NULL;
    END IF;
END $$;

ALTER TABLE ai_official_model ALTER COLUMN vendor SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'ai_official_model'::regclass
          AND conname = 'ai_official_model_vendor_check'
    ) THEN
        ALTER TABLE ai_official_model
            ADD CONSTRAINT ai_official_model_vendor_check
            CHECK (vendor IN (1, 2, 3));
    END IF;
END $$;

-- 旧唯一键仍基于 vendor_code/model_key；新代码需要 (vendor, model_key) 的并发保护。
CREATE UNIQUE INDEX IF NOT EXISTS ai_official_model_vendor_model_key_uq
    ON ai_official_model (vendor, model_key);
CREATE INDEX IF NOT EXISTS ai_official_model_vendor_sort_idx
    ON ai_official_model (vendor, sort_order);

COMMENT ON COLUMN ai_official_model.vendor IS '厂商：1=OpenAI，2=Anthropic，3=Google；后端 typed enum';
