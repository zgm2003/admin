-- 允许停用的 token 模型暂缺官方映射；启用时必须补齐。
-- 与 providerModel service 的“停用草稿可保存、启用严格校验”保持一致。

SET LOCAL lock_timeout = '10s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE ai_provider_model
    DROP CONSTRAINT IF EXISTS ai_provider_model_token_mode_check;

ALTER TABLE ai_provider_model
    ADD CONSTRAINT ai_provider_model_token_mode_check
        CHECK (
            pricing_mode <> 1
            OR (price_unit = 1 AND (status = 0 OR official_model_id IS NOT NULL))
        );

COMMENT ON COLUMN ai_provider_model.official_model_id IS
    'token 计费启用时必须映射官方模型；停用草稿可为空，媒体模型可为空';
