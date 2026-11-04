-- Admin AI official model catalogue seed v1.
-- Seeds 24 standard model identities (OpenAI 14 = 13 text + 1 image, Anthropic 10 text).
-- No providers, credentials, prices, menus, roles or settings are seeded here.
-- Existing administrator-owned rows are never overwritten: replay only inserts missing identities.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

DO $guard$
DECLARE missing text; stamp text;
BEGIN
  IF to_regclass('ai_official_model') IS NULL OR to_regclass('ai_provider') IS NULL
     OR to_regclass('user_account') IS NULL OR to_regclass('system_config_cache_state') IS NULL
     OR to_regclass('system_config_cache_outbox') IS NULL THEN
    RAISE EXCEPTION 'AI official model seed prerequisite table is missing';
  END IF;

  SELECT string_agg(required.attname, ',' ORDER BY required.attname) INTO missing
  FROM (VALUES
    ('vendor_code','varchar',true),('model_key','varchar',true),('name','varchar',true),
    ('operations','_text',true),('source_urls','jsonb',true),('status','int2',true),
    ('created_by','int8',true),('updated_by','int8',true),
    ('created_at','timestamptz',true),('updated_at','timestamptz',true)
  ) AS required(attname,typname,required_not_null)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_attribute a JOIN pg_type t ON t.oid=a.atttypid
    WHERE a.attrelid='ai_official_model'::regclass AND a.attname=required.attname
      AND a.attnum>0 AND NOT a.attisdropped AND t.typname=required.typname AND a.attnotnull=required.required_not_null
  );
  IF missing IS NOT NULL THEN RAISE EXCEPTION 'AI official model column drift: %', missing; END IF;

  IF NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='user_account'::regclass AND attname='status' AND attnum>0 AND NOT attisdropped)
     OR NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='user_account'::regclass AND attname='deleted_at' AND attnum>0 AND NOT attisdropped) THEN
    RAISE EXCEPTION 'AI official model seed user_account contract mismatch';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_index i
    WHERE i.indrelid='ai_official_model'::regclass AND i.indisunique AND i.indisvalid AND i.indisready AND i.indimmediate
      AND i.indnkeyatts=2 AND i.indnatts=2 AND i.indexprs IS NULL AND i.indpred IS NULL
      AND ARRAY(SELECT pg_get_indexdef(i.indexrelid,k,true) FROM generate_series(1,2) k)=ARRAY['vendor_code','model_key']
  ) THEN
    RAISE EXCEPTION 'AI official model unique identity drift';
  END IF;

  stamp := obj_description('ai_provider'::regclass,'pg_class');
  IF stamp IS NULL OR stamp NOT LIKE 'admin-ai-core-v1:%' THEN
    RAISE EXCEPTION 'AI core schema stamp is missing';
  END IF;
END $guard$;

CREATE TEMP TABLE _admin_ai_official_model_seed (
  vendor_code varchar(64) NOT NULL,
  model_key varchar(200) NOT NULL,
  name varchar(100) NOT NULL,
  operations text[] NOT NULL,
  source_urls jsonb NOT NULL,
  status smallint NOT NULL,
  PRIMARY KEY(vendor_code, model_key)
) ON COMMIT DROP;

INSERT INTO _admin_ai_official_model_seed(vendor_code,model_key,name,operations,source_urls,status) VALUES
('openai','gpt-6-astra','GPT-6 Astra',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-6.1-sol','GPT-6.1 Sol',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-6.1-terra','GPT-6.1 Terra',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-6-luna','GPT-6 Luna',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.6-sol','GPT-5.6 Sol',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.6-terra','GPT-5.6 Terra',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.6-luna','GPT-5.6 Luna',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.5','GPT-5.5',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.5-pro','GPT-5.5 Pro',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.4','GPT-5.4',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.4-pro','GPT-5.4 Pro',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.4-mini','GPT-5.4 Mini',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-5.4-nano','GPT-5.4 Nano',ARRAY['text.generate']::text[],'["https://platform.openai.com/docs/models"]'::jsonb,1),
('openai','gpt-image-2','GPT Image 2',ARRAY['image.generate']::text[],'["https://platform.openai.com/docs/guides/image-generation"]'::jsonb,1),
('anthropic','claude-opus-4-5','Claude Opus 4.5',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-sonnet-4-5','Claude Sonnet 4.5',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-haiku-4-5','Claude Haiku 4.5',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-opus-4-1','Claude Opus 4.1',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-sonnet-4','Claude Sonnet 4',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-opus-4','Claude Opus 4',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-3-7-sonnet','Claude 3.7 Sonnet',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-3-5-sonnet','Claude 3.5 Sonnet',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-3-5-haiku','Claude 3.5 Haiku',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1),
('anthropic','claude-3-opus','Claude 3 Opus',ARRAY['text.generate']::text[],'["https://docs.anthropic.com/en/docs/about-claude/models/overview"]'::jsonb,1);

LOCK TABLE ai_official_model, system_config_cache_state, system_config_cache_outbox IN SHARE ROW EXCLUSIVE MODE;

DO $seed$
DECLARE
  item record;
  actor_id bigint;
  runtime_generation bigint;
  expected integer;
  inserted integer := 0;
  inserted_rows integer;
BEGIN
  SELECT count(*) INTO expected FROM _admin_ai_official_model_seed;
  IF expected <> 24 THEN
    RAISE EXCEPTION 'AI official model seed definition must contain 24 identities';
  END IF;

  actor_id := NULL;
  IF to_regclass('permission_user_role') IS NOT NULL AND to_regclass('permission_role') IS NOT NULL THEN
    SELECT min(ur.user_id) INTO actor_id
    FROM permission_user_role ur
    JOIN permission_role r ON r.id=ur.role_id
    WHERE ur.deleted_at IS NULL AND r.deleted_at IS NULL AND r.status=1 AND r.code='super_admin';
  END IF;
  IF actor_id IS NULL THEN
    SELECT id INTO actor_id FROM user_account
    WHERE status=1 AND deleted_at IS NULL ORDER BY id LIMIT 1;
  END IF;
  IF actor_id IS NULL THEN
    RAISE EXCEPTION 'AI official model seed requires an active administrator account';
  END IF;

  SELECT generation INTO runtime_generation FROM system_config_cache_state
  WHERE namespace='ai.runtime' AND scope_key='global' FOR UPDATE;
  IF runtime_generation IS NULL OR runtime_generation<1 THEN
    RAISE EXCEPTION 'ai.runtime/global generation is missing; apply the AI core seed first';
  END IF;
  IF runtime_generation=9223372036854775807 THEN
    RAISE EXCEPTION 'ai.runtime/global generation overflow';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM system_config_cache_outbox
      WHERE namespace='ai.runtime' AND scope_key='global' AND generation=runtime_generation)
     OR EXISTS(SELECT 1 FROM system_config_cache_outbox
      WHERE namespace='ai.runtime' AND scope_key='global' AND generation>runtime_generation) THEN
    RAISE EXCEPTION 'ai.runtime/global generation/outbox drift';
  END IF;

  FOR item IN SELECT * FROM _admin_ai_official_model_seed ORDER BY vendor_code, model_key LOOP
    INSERT INTO ai_official_model(vendor_code,model_key,name,operations,source_urls,status,created_by,updated_by,created_at,updated_at)
    SELECT item.vendor_code,item.model_key,item.name,item.operations,item.source_urls,item.status,actor_id,actor_id,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
    WHERE NOT EXISTS (
      SELECT 1 FROM ai_official_model m
      WHERE m.vendor_code=item.vendor_code AND m.model_key=item.model_key
    );
    GET DIAGNOSTICS inserted_rows=ROW_COUNT;
    inserted:=inserted+inserted_rows;
  END LOOP;

  IF (SELECT count(*) FROM ai_official_model m
      JOIN _admin_ai_official_model_seed e ON e.vendor_code=m.vendor_code AND e.model_key=m.model_key) <> expected THEN
    RAISE EXCEPTION 'AI official model seed is incomplete';
  END IF;

  IF inserted>0 THEN
    IF inserted>expected THEN
      RAISE EXCEPTION 'AI official model seed inserted unexpected rows';
    END IF;
    UPDATE system_config_cache_state
      SET generation=runtime_generation+1, updated_at=CURRENT_TIMESTAMP
      WHERE namespace='ai.runtime' AND scope_key='global' AND generation=runtime_generation;
    IF NOT FOUND THEN
      RAISE EXCEPTION 'ai.runtime/global generation changed during seed';
    END IF;
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation)
      VALUES('ai.runtime','global',runtime_generation+1);
  END IF;
END $seed$;
