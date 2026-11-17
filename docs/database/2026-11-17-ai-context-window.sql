-- AI 上下文窗口配置
-- 官方模型目录保存模型的原生上下文窗口和最大输出 token 数，运行时据此做上下文预算。

ALTER TABLE ai_official_model
    ADD COLUMN IF NOT EXISTS context_window_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS max_output_tokens BIGINT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ai_official_model_context_window_tokens_check'
          AND conrelid = 'public.ai_official_model'::regclass
    ) THEN
        ALTER TABLE ai_official_model
            ADD CONSTRAINT ai_official_model_context_window_tokens_check
            CHECK (
                context_window_tokens IS NULL
                OR (context_window_tokens >= 1024 AND context_window_tokens <= 10000000)
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ai_official_model_max_output_tokens_check'
          AND conrelid = 'public.ai_official_model'::regclass
    ) THEN
        ALTER TABLE ai_official_model
            ADD CONSTRAINT ai_official_model_max_output_tokens_check
            CHECK (
                max_output_tokens IS NULL
                OR (max_output_tokens >= 1 AND max_output_tokens <= 1000000)
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ai_official_model_context_budget_check'
          AND conrelid = 'public.ai_official_model'::regclass
    ) THEN
        ALTER TABLE ai_official_model
            ADD CONSTRAINT ai_official_model_context_budget_check
            CHECK (
                context_window_tokens IS NULL
                OR max_output_tokens IS NULL
                OR max_output_tokens < context_window_tokens
            );
    END IF;
END $$;
