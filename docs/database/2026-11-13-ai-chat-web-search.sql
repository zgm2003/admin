-- Persist per-run web search intent and the citations used by the assistant answer.
ALTER TABLE public.ai_chat_run
    ADD COLUMN IF NOT EXISTS web_search BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE public.ai_chat_message
    ADD COLUMN IF NOT EXISTS citations JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN public.ai_chat_run.web_search IS
    'Whether this run was allowed to query the web search provider';
COMMENT ON COLUMN public.ai_chat_message.citations IS
    'Ordered web search citations stored as [{index,title,url,snippet}]';
