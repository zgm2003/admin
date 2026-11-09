-- 收缩阶段：新 API 已稳定使用 vendor 后执行，删除滚动部署遗留的字符串列。
-- 执行前确认没有旧 API 实例仍在写 vendor_code/vendor_name。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE ai_official_model DROP CONSTRAINT IF EXISTS ai_official_model_vendor_model_key;
DROP INDEX IF EXISTS ai_official_model_vendor_idx;
ALTER TABLE ai_official_model DROP COLUMN IF EXISTS vendor_code;
ALTER TABLE ai_official_model DROP COLUMN IF EXISTS vendor_name;

-- 新代码用唯一索引保护同一厂商下的模型身份。
CREATE UNIQUE INDEX IF NOT EXISTS ai_official_model_vendor_model_key_uq
    ON ai_official_model (vendor, model_key);
