-- Admin AI core maintenance seed. Run in one transaction after ai-core.sql.
-- This file contains engineering policy only: no credentials, providers, models or prices.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Parse the required shared-table contract in PostgreSQL itself. Extra unrelated
-- columns/checks are allowed; weakened columns, checks and uniqueness are not.
CREATE TEMP TABLE _admin_ai_seed_setting (
 id bigint NOT NULL, setting_key varchar(128) NOT NULL, value text NOT NULL,
 value_type smallint NOT NULL, description varchar(512) NOT NULL, status smallint NOT NULL,
 is_builtin smallint NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz,
 CONSTRAINT ck_system_setting_key CHECK(setting_key ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$'),
 CONSTRAINT ck_system_setting_value_type CHECK(value_type IN(1,2,3,4,5,6)),
 CONSTRAINT ck_system_setting_status CHECK(status IN(0,1)),
 CONSTRAINT ck_system_setting_is_builtin CHECK(is_builtin IN(0,1))
) ON COMMIT DROP;
CREATE UNIQUE INDEX _admin_ai_seed_setting_key ON _admin_ai_seed_setting(setting_key) WHERE deleted_at IS NULL;
CREATE TEMP TABLE _admin_ai_seed_state (
 namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY(namespace,scope_key), CONSTRAINT ck_system_config_cache_state_value CHECK(generation>=1)
) ON COMMIT DROP;
CREATE TEMP TABLE _admin_ai_seed_outbox (
 id bigint NOT NULL, namespace varchar(128) NOT NULL, scope_key varchar(128) NOT NULL, generation bigint NOT NULL,
 attempts integer NOT NULL, available_at timestamptz NOT NULL, locked_until timestamptz, lock_token varchar(64),
 last_error varchar(512) NOT NULL, published_at timestamptz, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 UNIQUE(namespace,scope_key,generation),
 CONSTRAINT ck_system_config_cache_outbox_generation_value CHECK(generation>=1),
 CONSTRAINT ck_system_config_cache_outbox_attempts CHECK(attempts>=0),
 CONSTRAINT ck_system_config_cache_outbox_lock CHECK((locked_until IS NULL AND lock_token IS NULL) OR (locked_until IS NOT NULL AND lock_token IS NOT NULL)),
 CONSTRAINT ck_system_config_cache_outbox_published_lock CHECK(published_at IS NULL OR (locked_until IS NULL AND lock_token IS NULL))
) ON COMMIT DROP;
CREATE TEMP TABLE _admin_ai_seed_schedule (
 id bigint NOT NULL, name varchar(128) NOT NULL, description varchar(512) NOT NULL, task_type varchar(128) NOT NULL,
 cron_expression varchar(128) NOT NULL, timezone varchar(64) NOT NULL, params jsonb NOT NULL, status smallint NOT NULL,
 next_run_at timestamptz, builtin_key varchar(128), created_by bigint, updated_by bigint,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz,
 CONSTRAINT ck_system_scheduler_schedule_status CHECK(status IN(0,1)),
 CONSTRAINT ck_system_scheduler_schedule_params CHECK(jsonb_typeof(params)='object')
) ON COMMIT DROP;
CREATE UNIQUE INDEX _admin_ai_seed_schedule_key ON _admin_ai_seed_schedule(builtin_key) WHERE deleted_at IS NULL AND builtin_key IS NOT NULL;

DO $contract$
DECLARE spec record; col record; check_ record; actual oid; template oid;
BEGIN
 FOR spec IN SELECT * FROM (VALUES
  ('system_setting','_admin_ai_seed_setting'), ('system_config_cache_state','_admin_ai_seed_state'),
  ('system_config_cache_outbox','_admin_ai_seed_outbox'), ('system_scheduler_schedule','_admin_ai_seed_schedule')
 ) v(table_name,template_name) LOOP
  actual := to_regclass(format('%I.%I',current_schema(),spec.table_name));
  template := to_regclass('pg_temp.'||spec.template_name);
  IF actual IS NULL THEN RAISE EXCEPTION 'AI seed prerequisite missing: %',spec.table_name; END IF;
  FOR col IN SELECT attname,atttypid,atttypmod,attnotnull FROM pg_attribute WHERE attrelid=template AND attnum>0 AND NOT attisdropped LOOP
   IF NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=actual AND attname=col.attname AND attnum>0 AND NOT attisdropped
       AND atttypid=col.atttypid AND atttypmod=col.atttypmod AND attnotnull=col.attnotnull) THEN
    RAISE EXCEPTION 'AI seed prerequisite column drift: %.%',spec.table_name,col.attname;
   END IF;
  END LOOP;
  FOR check_ IN SELECT conname,pg_get_constraintdef(oid) definition FROM pg_constraint WHERE conrelid=template AND contype='c' LOOP
   IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=actual AND conname=check_.conname AND contype='c' AND convalidated
      AND pg_get_constraintdef(oid)=check_.definition) THEN
    RAISE EXCEPTION 'AI seed prerequisite check drift: %.%',spec.table_name,check_.conname;
   END IF;
  END LOOP;
   -- Compare unique key columns and parsed predicates, rather than trusting an index name.
   IF EXISTS(SELECT 1 FROM pg_index e WHERE e.indrelid=template AND e.indisunique AND NOT EXISTS(
    SELECT 1 FROM pg_index a WHERE a.indrelid=actual AND a.indisunique AND a.indisvalid AND a.indisready AND a.indimmediate
     AND a.indnkeyatts=e.indnkeyatts AND a.indexprs IS NULL
     AND pg_get_expr(a.indpred,a.indrelid) IS NOT DISTINCT FROM pg_get_expr(e.indpred,e.indrelid)
     AND ARRAY(SELECT pg_get_indexdef(a.indexrelid,k,true) FROM generate_series(1,a.indnkeyatts) k)
       = ARRAY(SELECT pg_get_indexdef(e.indexrelid,k,true) FROM generate_series(1,e.indnkeyatts) k)
   )) THEN RAISE EXCEPTION 'AI seed prerequisite unique index drift: %',spec.table_name; END IF;
 END LOOP;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint c WHERE conrelid='system_config_cache_outbox'::regclass
   AND contype='f' AND convalidated AND NOT condeferrable AND confrelid='system_config_cache_state'::regclass
   AND confdeltype='a' AND confupdtype='a'
   AND ARRAY(SELECT attname::text FROM unnest(c.conkey) WITH ORDINALITY k(n,p) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.n ORDER BY p)=ARRAY['namespace','scope_key']
   AND ARRAY(SELECT attname::text FROM unnest(c.confkey) WITH ORDINALITY k(n,p) JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.n ORDER BY p)=ARRAY['namespace','scope_key']) THEN
  RAISE EXCEPTION 'AI seed prerequisite outbox foreign key drift';
 END IF;
END $contract$;

LOCK TABLE system_setting,system_config_cache_state,system_config_cache_outbox,system_scheduler_schedule IN SHARE ROW EXCLUSIVE MODE;

DO $seed$
DECLARE
  item record;
  setting_generation bigint;
  runtime_generation bigint;
  inserted_count integer := 0;
  inserted_rows integer;
  policy_check text;
  expected text[] := ARRAY[
    'ai.run.user_inflight_limit','ai.run.user_rate_per_second','ai.run.user_rate_burst',
    'ai.run.global_rate_per_second','ai.run.global_rate_burst','ai.run.queued_backpressure_threshold',
    'ai.run.active_backpressure_threshold','ai.run.max_queued_seconds','ai.run.text_timeout_seconds',
    'ai.run.image_timeout_seconds','ai.run.audio_timeout_seconds','ai.run.video_timeout_seconds',
    'ai.run.max_model_rounds','ai.run.max_tool_calls','ai.run.max_bound_tools',
    'ai.run.max_input_attachments','ai.run.max_output_tokens','ai.run.max_text_input_bytes',
    'ai.run.max_images','ai.media.image_max_bytes','ai.media.audio_max_bytes',
    'ai.media.video_max_bytes','ai.media.download_timeout_seconds','ai.media.transfer_timeout_seconds',
    'ai.media.spool_max_bytes_per_worker','ai.run.event_retention_days'
  ];
BEGIN
  SELECT generation INTO setting_generation FROM system_config_cache_state
    WHERE namespace='system.setting' AND scope_key='global' FOR UPDATE;
  IF setting_generation IS NULL OR setting_generation<1 THEN RAISE EXCEPTION 'system.setting/global generation missing'; END IF;
  IF EXISTS(SELECT 1 FROM system_config_cache_outbox WHERE namespace='system.setting' AND scope_key='global' AND generation>setting_generation) THEN
    RAISE EXCEPTION 'system.setting/global future outbox conflict';
  END IF;
  SELECT generation INTO runtime_generation FROM system_config_cache_state
    WHERE namespace='ai.runtime' AND scope_key='global' FOR UPDATE;
  IF runtime_generation IS NULL AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='system_setting'::regclass AND conname='ck_system_setting_ai_policy') THEN
    RAISE EXCEPTION 'ai.runtime/global generation missing on replay';
  END IF;
  IF runtime_generation IS NOT NULL THEN
    IF runtime_generation<1 OR NOT EXISTS(SELECT 1 FROM system_config_cache_outbox WHERE namespace='ai.runtime' AND scope_key='global' AND generation=runtime_generation)
      OR EXISTS(SELECT 1 FROM system_config_cache_outbox WHERE namespace='ai.runtime' AND scope_key='global' AND generation>runtime_generation) THEN
      RAISE EXCEPTION 'ai.runtime/global generation/outbox drift';
    END IF;
  ELSIF EXISTS(SELECT 1 FROM system_config_cache_outbox WHERE namespace='ai.runtime' AND scope_key='global') THEN
    RAISE EXCEPTION 'ai.runtime/global orphan outbox';
  END IF;

  EXECUTE format('ALTER TABLE pg_temp._admin_ai_seed_setting ADD CONSTRAINT ck_system_setting_ai_policy CHECK (
    NOT (setting_key = ANY (%L::text[])) OR (
      CASE WHEN value ~ ''^[0-9]{1,19}$'' THEN value::numeric BETWEEN 1 AND 9223372036854775807 ELSE false END
      AND value_type=2 AND status=1 AND is_builtin=1 AND deleted_at IS NULL))',expected);
  SELECT pg_get_constraintdef(oid) INTO policy_check FROM pg_constraint
    WHERE conrelid='pg_temp._admin_ai_seed_setting'::regclass AND conname='ck_system_setting_ai_policy';
  IF EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='system_setting'::regclass AND conname='ck_system_setting_ai_policy') THEN
    IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='system_setting'::regclass AND conname='ck_system_setting_ai_policy'
      AND contype='c' AND convalidated AND pg_get_constraintdef(oid)=policy_check) THEN
      RAISE EXCEPTION 'AI policy check drift';
    END IF;
  ELSE
    EXECUTE 'ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_ai_policy '||policy_check;
  END IF;
  FOR item IN SELECT * FROM (VALUES
    ('ai.run.user_inflight_limit','2','用户跨平台同时在途 Run 上限'),
    ('ai.run.user_rate_per_second','1','用户分布式令牌桶每秒速率'),
    ('ai.run.user_rate_burst','5','用户分布式令牌桶突发容量'),
    ('ai.run.global_rate_per_second','200','全局分布式令牌桶每秒速率'),
    ('ai.run.global_rate_burst','400','全局分布式令牌桶突发容量'),
    ('ai.run.queued_backpressure_threshold','1000','排队背压阈值'),
    ('ai.run.active_backpressure_threshold','2000','活跃背压阈值'),
    ('ai.run.max_queued_seconds','60','排队最长等待秒数'),
    ('ai.run.text_timeout_seconds','180','文本生成超时秒数'),
    ('ai.run.image_timeout_seconds','300','图片生成超时秒数'),
    ('ai.run.audio_timeout_seconds','300','音频生成超时秒数'),
    ('ai.run.video_timeout_seconds','3600','视频生成超时秒数'),
    ('ai.run.max_model_rounds','8','模型最大轮次数'),
    ('ai.run.max_tool_calls','16','工具调用总次数上限'),
    ('ai.run.max_bound_tools','16','Agent 绑定工具上限'),
    ('ai.run.max_input_attachments','4','单次输入附件上限'),
    ('ai.run.max_output_tokens','4096','输出 token 上限'),
    ('ai.run.max_text_input_bytes','131072','文本输入字节上限'),
    ('ai.run.max_images','4','单次生成图片数量上限'),
    ('ai.media.image_max_bytes','10485760','图片媒体字节上限'),
    ('ai.media.audio_max_bytes','104857600','音频媒体字节上限'),
    ('ai.media.video_max_bytes','524288000','视频媒体字节上限'),
    ('ai.media.download_timeout_seconds','300','媒体下载超时秒数'),
    ('ai.media.transfer_timeout_seconds','900','媒体转存超时秒数'),
    ('ai.media.spool_max_bytes_per_worker','2147483648','单 Worker 媒体暂存字节上限'),
    ('ai.run.event_retention_days','7','Run 事件保留天数')
  ) AS v(setting_key,value,description) LOOP
    INSERT INTO system_setting(setting_key,value,value_type,description,status,is_builtin,created_at,updated_at)
    SELECT item.setting_key,item.value,2,item.description,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
    WHERE NOT EXISTS (
      SELECT 1 FROM system_setting s WHERE s.setting_key=item.setting_key AND s.deleted_at IS NULL
    );
    GET DIAGNOSTICS inserted_rows=ROW_COUNT;
    inserted_count := inserted_count+inserted_rows;
  END LOOP;

  IF (SELECT count(*) FROM system_setting WHERE setting_key = ANY(expected) AND deleted_at IS NULL) <> cardinality(expected) THEN
    RAISE EXCEPTION 'AI setting seed is incomplete';
  END IF;
  FOR item IN SELECT setting_key FROM system_setting WHERE setting_key = ANY(expected) AND deleted_at IS NULL LOOP
    IF (SELECT value_type FROM system_setting WHERE setting_key=item.setting_key AND deleted_at IS NULL) <> 2
       OR (SELECT status FROM system_setting WHERE setting_key=item.setting_key AND deleted_at IS NULL) <> 1
       OR (SELECT is_builtin FROM system_setting WHERE setting_key=item.setting_key AND deleted_at IS NULL) <> 1 THEN
      RAISE EXCEPTION 'AI setting metadata drift: %', item.setting_key;
    END IF;
  END LOOP;
  IF inserted_count>0 THEN
    IF setting_generation=9223372036854775807 OR runtime_generation=9223372036854775807 THEN
      RAISE EXCEPTION 'AI policy generation overflow';
    END IF;
    UPDATE system_config_cache_state SET generation=setting_generation+1,updated_at=CURRENT_TIMESTAMP
      WHERE namespace='system.setting' AND scope_key='global';
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation)
      VALUES('system.setting','global',setting_generation+1);
    IF runtime_generation IS NOT NULL THEN
      UPDATE system_config_cache_state SET generation=runtime_generation+1,updated_at=CURRENT_TIMESTAMP
        WHERE namespace='ai.runtime' AND scope_key='global';
      INSERT INTO system_config_cache_outbox(namespace,scope_key,generation)
        VALUES('ai.runtime','global',runtime_generation+1);
    END IF;
  END IF;
  IF runtime_generation IS NULL THEN
    INSERT INTO system_config_cache_state(namespace,scope_key,generation) VALUES('ai.runtime','global',1);
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation) VALUES('ai.runtime','global',1);
  END IF;
END $seed$;

DO $schedule$
DECLARE item record; existing_count integer; actual record;
BEGIN
  FOR item IN SELECT * FROM (VALUES
    ('ai.run.recover','AI Run 恢复巡检','AI Run recovery scan','ai.run.recover','* * * * *','Asia/Shanghai','{}'::jsonb,0),
    ('ai.run.event.cleanup','AI Run 事件清理','AI Run event cleanup','ai.run.event.cleanup','0 3 * * *','Asia/Shanghai','{}'::jsonb,0)
  ) AS v(builtin_key,name,description,task_type,cron_expression,timezone,params,status) LOOP
    SELECT count(*) INTO existing_count FROM system_scheduler_schedule
      WHERE builtin_key=item.builtin_key AND deleted_at IS NULL;
    IF existing_count = 0 THEN
      INSERT INTO system_scheduler_schedule(name,description,task_type,cron_expression,timezone,params,status,builtin_key,created_at,updated_at)
      VALUES(item.name,item.description,item.task_type,item.cron_expression,item.timezone,item.params,item.status,item.builtin_key,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
    ELSIF existing_count <> 1 THEN
      RAISE EXCEPTION 'AI scheduler builtin key is duplicated: %', item.builtin_key;
    END IF;
    SELECT name,description,task_type,cron_expression,timezone,params,status,next_run_at INTO actual
      FROM system_scheduler_schedule WHERE builtin_key=item.builtin_key AND deleted_at IS NULL;
    IF actual.name IS DISTINCT FROM item.name OR actual.description IS DISTINCT FROM item.description
       OR actual.task_type IS DISTINCT FROM item.task_type OR actual.cron_expression IS DISTINCT FROM item.cron_expression
       OR actual.timezone IS DISTINCT FROM item.timezone OR actual.params IS DISTINCT FROM item.params
       OR actual.status IS DISTINCT FROM item.status OR actual.next_run_at IS NOT NULL THEN
      RAISE EXCEPTION 'AI scheduler definition drift: %', item.builtin_key;
    END IF;
  END LOOP;
END $schedule$;
