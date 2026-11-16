-- Persist the model reasoning stream separately from the visible answer.
ALTER TABLE public.ai_chat_message
    ADD COLUMN IF NOT EXISTS reasoning_content TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN public.ai_chat_message.reasoning_content IS
    'Reasoning / thinking content emitted by reasoning-capable models; never rendered as the final answer';
