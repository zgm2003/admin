-- AI 工作台消息软删除。
-- 删除/编辑只影响聊天记录可见性，运行日志继续保留原始消息。

BEGIN;

ALTER TABLE public.ai_chat_message
    ADD COLUMN IF NOT EXISTS deleted_at timestamp with time zone;

COMMENT ON COLUMN public.ai_chat_message.deleted_at IS
    '聊天记录软删除时间；运行日志仍保留该消息';

CREATE INDEX IF NOT EXISTS ai_chat_message_visible_idx
    ON public.ai_chat_message (conversation_id, seq DESC)
    WHERE deleted_at IS NULL;

COMMIT;
