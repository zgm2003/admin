-- Admin AI policy slim: 26 admin-editable AI settings -> 5.
-- Model capability stays in ai_provider_model.limits, agent behavior is fixed
-- in ai/limits.go (phase 1), engineering limits are constants and deployment
-- capacity comes from environment variables.
--
-- This migration is forward-only and idempotent:
--   * it soft-deletes the 21 removed settings only when they are still active;
--   * it replaces ck_system_setting_ai_policy with the 5-key definition;
--   * it advances system.setting/global generation exactly once, and only when
--     at least one setting was actually removed.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

DO $slim$
DECLARE
  retained text[] := ARRAY[
    'ai.run.user_inflight_limit',
    'ai.run.user_rate_per_second',
    'ai.run.user_rate_burst',
    'ai.run.max_queued_seconds',
    'ai.run.event_retention_days'
  ];
  removed text[] := ARRAY[
    'ai.run.global_rate_per_second',
    'ai.run.global_rate_burst',
    'ai.run.queued_backpressure_threshold',
    'ai.run.active_backpressure_threshold',
    'ai.run.text_timeout_seconds',
    'ai.run.image_timeout_seconds',
    'ai.run.audio_timeout_seconds',
    'ai.run.video_timeout_seconds',
    'ai.run.max_model_rounds',
    'ai.run.max_tool_calls',
    'ai.run.max_bound_tools',
    'ai.run.max_input_attachments',
    'ai.run.max_output_tokens',
    'ai.run.max_text_input_bytes',
    'ai.run.max_images',
    'ai.media.image_max_bytes',
    'ai.media.audio_max_bytes',
    'ai.media.video_max_bytes',
    'ai.media.download_timeout_seconds',
    'ai.media.transfer_timeout_seconds',
    'ai.media.spool_max_bytes_per_worker'
  ];
  setting_generation bigint;
  deleted_count integer;
BEGIN
  IF (SELECT count(*) FROM system_setting WHERE setting_key = ANY(retained) AND deleted_at IS NULL) <> cardinality(retained) THEN
    RAISE EXCEPTION 'AI policy retained settings are incomplete';
  END IF;
  IF EXISTS (
    SELECT 1 FROM system_setting
    WHERE setting_key = ANY(retained) AND deleted_at IS NULL
      AND (value_type <> 2 OR status <> 1 OR is_builtin <> 1 OR value !~ '^[0-9]{1,19}$')
  ) THEN
    RAISE EXCEPTION 'AI policy retained setting metadata drift';
  END IF;

  LOCK TABLE system_setting, system_config_cache_state, system_config_cache_outbox IN SHARE ROW EXCLUSIVE MODE;

  -- The old check requires every listed key to keep deleted_at NULL, so it must
  -- be removed before the soft delete and recreated with the 5-key definition.
  EXECUTE 'ALTER TABLE system_setting DROP CONSTRAINT IF EXISTS ck_system_setting_ai_policy';

  UPDATE system_setting
     SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
   WHERE setting_key = ANY(removed) AND deleted_at IS NULL;
  GET DIAGNOSTICS deleted_count = ROW_COUNT;

  EXECUTE format('ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_ai_policy CHECK (
    NOT (setting_key = ANY (%L::text[])) OR (
      CASE WHEN value ~ ''^[0-9]{1,19}$'' THEN value::numeric BETWEEN 1 AND 9223372036854775807 ELSE false END
      AND value_type = 2 AND status = 1 AND is_builtin = 1 AND deleted_at IS NULL))', retained);

  IF deleted_count > 0 THEN
    SELECT generation INTO setting_generation
      FROM system_config_cache_state
     WHERE namespace = 'system.setting' AND scope_key = 'global'
     FOR UPDATE;
    IF setting_generation IS NULL OR setting_generation < 1 THEN
      RAISE EXCEPTION 'system.setting/global generation missing';
    END IF;
    IF EXISTS (SELECT 1 FROM system_config_cache_outbox
               WHERE namespace = 'system.setting' AND scope_key = 'global'
                 AND generation > setting_generation) THEN
      RAISE EXCEPTION 'system.setting/global future outbox conflict';
    END IF;
    IF setting_generation = 9223372036854775807 THEN
      RAISE EXCEPTION 'system.setting generation overflow';
    END IF;
    UPDATE system_config_cache_state
       SET generation = setting_generation + 1, updated_at = CURRENT_TIMESTAMP
     WHERE namespace = 'system.setting' AND scope_key = 'global';
    INSERT INTO system_config_cache_outbox(namespace, scope_key, generation)
      VALUES ('system.setting', 'global', setting_generation + 1);
  END IF;
END $slim$;
