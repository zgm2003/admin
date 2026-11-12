-- 智能体级响应超时。
-- 运行时的上游请求超时由智能体配置控制；供应商超时仅保留给供应商连通性测试。
-- 可重复执行。

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE public.ai_agent
    ADD COLUMN IF NOT EXISTS timeout_ms integer NOT NULL DEFAULT 60000;

ALTER TABLE public.ai_agent
    DROP CONSTRAINT IF EXISTS ai_agent_timeout_ms_check;
ALTER TABLE public.ai_agent
    ADD CONSTRAINT ai_agent_timeout_ms_check CHECK (timeout_ms BETWEEN 1000 AND 300000);

COMMENT ON COLUMN public.ai_agent.timeout_ms IS '智能体运行超时（毫秒），1 秒到 5 分钟；对话运行时使用该值控制上游请求';
