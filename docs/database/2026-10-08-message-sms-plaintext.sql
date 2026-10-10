-- Forward migration: align SMS phone data with the plaintext email contract.
-- scope: 0=phone, 1=prefix. action: 0=deny, 1=allow.
-- Existing encrypted rows are refused: SQL cannot safely decrypt them without
-- the application key. The current database has zero SMS rules/logs.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $migration$
DECLARE
    rule_scope_type text;
    rule_action_type text;
    changed boolean := false;
    generation_value bigint;
    changed_at timestamptz := clock_timestamp();
    legacy_count bigint;
BEGIN
    IF to_regclass(current_schema() || '.message_sms_recipient_rule') IS NULL
       OR to_regclass(current_schema() || '.message_sms_log') IS NULL THEN
        RAISE EXCEPTION 'SMS recipient rule and log tables are required';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(current_schema() || ':sms-plaintext-20261008', 0));
    LOCK TABLE message_sms_recipient_rule, message_sms_log IN ACCESS EXCLUSIVE MODE;

    SELECT data_type INTO rule_scope_type FROM information_schema.columns
      WHERE table_schema=current_schema() AND table_name='message_sms_recipient_rule' AND column_name='scope';
    SELECT data_type INTO rule_action_type FROM information_schema.columns
      WHERE table_schema=current_schema() AND table_name='message_sms_recipient_rule' AND column_name='action';
    IF rule_scope_type IS NULL OR rule_action_type IS NULL THEN
        RAISE EXCEPTION 'SMS recipient rule scope/action columns are missing';
    END IF;

    SELECT count(*) INTO legacy_count FROM information_schema.columns
      WHERE table_schema=current_schema() AND table_name='message_sms_recipient_rule'
        AND column_name='pattern_ciphertext';
    IF legacy_count > 0 THEN
        IF EXISTS (SELECT 1 FROM message_sms_recipient_rule) THEN
            RAISE EXCEPTION 'message_sms_recipient_rule contains unrecoverable encrypted rows';
        END IF;
        ALTER TABLE message_sms_recipient_rule
          DROP CONSTRAINT IF EXISTS ck_message_sms_recipient_rule_scope,
          DROP CONSTRAINT IF EXISTS ck_message_sms_recipient_rule_action,
          DROP CONSTRAINT IF EXISTS message_sms_recipient_rule_scope_check,
          DROP CONSTRAINT IF EXISTS message_sms_recipient_rule_action_check;
        DROP INDEX IF EXISTS ux_message_sms_recipient_rule_pattern_action_active;
        ALTER TABLE message_sms_recipient_rule ADD COLUMN IF NOT EXISTS pattern VARCHAR(32);
        ALTER TABLE message_sms_recipient_rule DROP COLUMN IF EXISTS pattern_ciphertext;
        ALTER TABLE message_sms_recipient_rule DROP COLUMN IF EXISTS pattern_hint;
        ALTER TABLE message_sms_recipient_rule DROP COLUMN IF EXISTS pattern_hmac;
        IF rule_scope_type IN ('character varying','text') THEN
            IF EXISTS (SELECT 1 FROM message_sms_recipient_rule WHERE scope NOT IN ('phone','prefix')) THEN
                RAISE EXCEPTION 'message_sms_recipient_rule contains unknown scope';
            END IF;
            ALTER TABLE message_sms_recipient_rule ALTER COLUMN scope TYPE SMALLINT
              USING CASE scope WHEN 'phone' THEN 0 WHEN 'prefix' THEN 1 END;
        ELSIF rule_scope_type <> 'smallint' THEN
            RAISE EXCEPTION 'unsupported SMS recipient rule scope type %', rule_scope_type;
        END IF;
        IF rule_action_type IN ('character varying','text') THEN
            IF EXISTS (SELECT 1 FROM message_sms_recipient_rule WHERE action NOT IN ('allow','deny')) THEN
                RAISE EXCEPTION 'message_sms_recipient_rule contains unknown action';
            END IF;
            ALTER TABLE message_sms_recipient_rule ALTER COLUMN action TYPE SMALLINT
              USING CASE action WHEN 'deny' THEN 0 WHEN 'allow' THEN 1 END;
        ELSIF rule_action_type <> 'smallint' THEN
            RAISE EXCEPTION 'unsupported SMS recipient rule action type %', rule_action_type;
        END IF;
        ALTER TABLE message_sms_recipient_rule ALTER COLUMN pattern SET NOT NULL;
        ALTER TABLE message_sms_recipient_rule
          ADD CONSTRAINT message_sms_recipient_rule_scope_check CHECK (scope IN (0,1)),
          ADD CONSTRAINT message_sms_recipient_rule_action_check CHECK (action IN (0,1)),
          ADD CONSTRAINT message_sms_recipient_rule_pattern_check CHECK (
            (scope=0 AND pattern ~ '^[+]861[3-9][0-9]{9}$') OR
            (scope=1 AND pattern ~ '^[+]86[0-9]{3,10}$')
          );
        CREATE UNIQUE INDEX ux_message_sms_recipient_rule_pattern_action_active
          ON message_sms_recipient_rule(scope, pattern, action) WHERE deleted_at IS NULL;
        changed := true;
    ELSE
        IF rule_scope_type <> 'smallint' OR rule_action_type <> 'smallint' THEN
            RAISE EXCEPTION 'SMS recipient rule plaintext columns are partially migrated';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='message_sms_recipient_rule'::regclass AND conname='message_sms_recipient_rule_pattern_check') THEN
            ALTER TABLE message_sms_recipient_rule ADD CONSTRAINT message_sms_recipient_rule_pattern_check CHECK (
              (scope=0 AND pattern ~ '^[+]861[3-9][0-9]{9}$') OR
              (scope=1 AND pattern ~ '^[+]86[0-9]{3,10}$')
            );
        END IF;
    END IF;

    SELECT count(*) INTO legacy_count FROM information_schema.columns
      WHERE table_schema=current_schema() AND table_name='message_sms_log'
        AND column_name='to_phone_ciphertext';
    IF legacy_count > 0 THEN
        IF EXISTS (SELECT 1 FROM message_sms_log) THEN
            RAISE EXCEPTION 'message_sms_log contains unrecoverable encrypted rows';
        END IF;
        DROP INDEX IF EXISTS ix_message_sms_log_to_phone_hmac_id_desc;
        ALTER TABLE message_sms_log ADD COLUMN IF NOT EXISTS to_phone VARCHAR(32);
        ALTER TABLE message_sms_log DROP COLUMN IF EXISTS to_phone_ciphertext;
        ALTER TABLE message_sms_log DROP COLUMN IF EXISTS to_phone_hint;
        ALTER TABLE message_sms_log DROP COLUMN IF EXISTS to_phone_hmac;
        ALTER TABLE message_sms_log ALTER COLUMN to_phone SET NOT NULL;
        ALTER TABLE message_sms_log
          ADD CONSTRAINT message_sms_log_to_phone_check CHECK (to_phone ~ '^[+]861[3-9][0-9]{9}$');
        changed := true;
    ELSE
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_log' AND column_name='to_phone') THEN
            RAISE EXCEPTION 'message_sms_log plaintext phone column is missing';
        END IF;
    END IF;

    IF changed THEN
        SELECT generation INTO generation_value FROM system_config_cache_generation
          WHERE namespace='message.sms' AND scope_key='global' FOR UPDATE;
        IF generation_value IS NULL OR generation_value < 1 THEN
            RAISE EXCEPTION 'message.sms/global generation is missing or invalid';
        END IF;
        UPDATE system_config_cache_generation SET generation=generation+1,updated_at=changed_at
          WHERE namespace='message.sms' AND scope_key='global' RETURNING generation INTO generation_value;
        INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,attempts,available_at,created_at,updated_at)
          VALUES ('message.sms','global',generation_value,0,changed_at,changed_at,changed_at);
    END IF;
END
$migration$;
COMMIT;
