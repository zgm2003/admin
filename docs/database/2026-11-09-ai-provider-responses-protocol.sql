-- 增加 OpenAI Responses 作为独立供应商协议。
-- 不回填现有通道；现有 1/2/3 的含义不变。
SET LOCAL lock_timeout = '10s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE ai_provider DROP CONSTRAINT IF EXISTS ai_provider_protocol_check;
ALTER TABLE ai_provider
    ADD CONSTRAINT ai_provider_protocol_check CHECK (protocol IN (1, 2, 3, 4));

COMMENT ON COLUMN ai_provider.protocol IS
    '协议：1=OpenAI Chat Completions 兼容，2=Anthropic，3=Gemini，4=OpenAI Responses';
