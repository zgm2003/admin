-- 5 = media file; 6 = media image/video. Object keys and record IDs are preserved.
-- One atomic block: a validation/write failure rolls back DDL, rows and cache invalidation.
DO $migration$
DECLARE
  changed_rows bigint;
  next_generation bigint;
  needs_split boolean;
BEGIN
  SET LOCAL lock_timeout = '5s';
  SET LOCAL statement_timeout = '30s';
  LOCK TABLE system_setting IN SHARE ROW EXCLUSIVE MODE;
  PERFORM 1 FROM system_config_cache_state
    WHERE namespace = 'system.setting' AND scope_key = 'global' FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'system.setting cache state is missing'; END IF;
  SELECT pg_get_constraintdef(oid) !~ '\m6\M' INTO needs_split
    FROM pg_constraint WHERE conrelid = 'system_setting'::regclass AND conname = 'ck_system_setting_value_type';
  IF NOT FOUND THEN RAISE EXCEPTION 'system.setting value type constraint is missing'; END IF;

  IF (SELECT count(*) FROM system_setting WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL AND value_type IN (5,6)) <> 1 THEN
    RAISE EXCEPTION 'default avatar media setting is missing or has an invalid type';
  END IF;
  IF (SELECT count(*) FROM system_setting WHERE setting_key = 'message.mail.recipient_rule.import_template_object_key' AND deleted_at IS NULL AND value_type = 5) <> 1 THEN
    RAISE EXCEPTION 'mail import template file setting is missing or has an invalid type';
  END IF;
  IF EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL
      AND value <> '' AND value !~ '^[a-z][a-z0-9_-]*/\.admin-storage/v2/p[1-9][0-9]*/r[1-9][0-9]*/c[1-9][0-9]*/v[1-9][0-9]*/[0-9]{4}/[0-9]{2}/[0-9]{2}/[a-f0-9]{32}\.(png|jpg|jpeg|gif|webp)$') THEN
    RAISE EXCEPTION 'default avatar object key is invalid';
  END IF;

  IF NOT needs_split THEN
    IF EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'app.brand.default_avatar' AND deleted_at IS NULL AND value_type <> 6) THEN
      RAISE EXCEPTION 'migrated default avatar has an invalid type';
    END IF;
    RETURN;
  END IF;

  ALTER TABLE system_setting DROP CONSTRAINT ck_system_setting_value_type;
  ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_value_type CHECK (value_type IN (1,2,3,4,5,6));
  COMMENT ON COLUMN system_setting.value_type IS '1=字符串, 2=数字, 3=布尔, 4=JSON, 5=媒体-文件对象键, 6=媒体-图片/视频对象键';

  UPDATE system_setting SET value_type = 6, updated_at = CURRENT_TIMESTAMP
    WHERE value_type = 5 AND (setting_key = 'app.brand.default_avatar'
      OR (setting_key <> 'message.mail.recipient_rule.import_template_object_key'
        AND value ~* '\.(png|jpg|jpeg|gif|webp|avif|bmp|svg|mp4|webm|mov|m4v|ogv)$'));
  GET DIAGNOSTICS changed_rows = ROW_COUNT;
  IF changed_rows > 0 THEN
    UPDATE system_config_cache_state SET generation = generation + 1, updated_at = CURRENT_TIMESTAMP
      WHERE namespace = 'system.setting' AND scope_key = 'global'
      RETURNING generation INTO next_generation;
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,available_at,created_at,updated_at)
      VALUES ('system.setting','global',next_generation,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
  END IF;
END
$migration$;
