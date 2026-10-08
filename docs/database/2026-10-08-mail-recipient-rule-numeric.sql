-- Explicit offline forward migration; run via the same-name PowerShell runner.
-- scope: 0=email, 1=domain. action: 0=deny, 1=allow.
-- No changes to rule identity/timestamps, CSV text tokens, permissions or quotas.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $migration$
DECLARE
    target oid;
    scope_type text;
    action_type text;
    generation_value bigint;
    changed_at timestamptz := clock_timestamp();
BEGIN
    target := to_regclass(format('%I.message_mail_recipient_rule', current_schema()));
    IF target IS NULL THEN
        RAISE EXCEPTION 'message_mail_recipient_rule is missing in the selected schema';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(current_schema() || ':mail-rule-numeric-20261008', 0));
    LOCK TABLE message_mail_recipient_rule IN ACCESS EXCLUSIVE MODE;

    SELECT data_type INTO scope_type FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='message_mail_recipient_rule' AND column_name='scope';
    SELECT data_type INTO action_type FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='message_mail_recipient_rule' AND column_name='action';
    IF (SELECT count(*) FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='message_mail_recipient_rule'
          AND column_name IN ('scope','action') AND is_nullable='NO' AND column_default IS NULL) <> 2 THEN
        RAISE EXCEPTION 'scope/action must be required without implicit defaults';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_index i
        WHERE i.indrelid=target AND i.indisunique AND i.indisvalid AND i.indisready
          AND i.indnkeyatts=3 AND i.indnatts=3
          AND pg_get_indexdef(i.indexrelid,1,true)='scope'
          AND pg_get_indexdef(i.indexrelid,2,true)='pattern'
          AND pg_get_indexdef(i.indexrelid,3,true)='action'
          AND pg_get_expr(i.indpred,i.indrelid)='(deleted_at IS NULL)'
    ) THEN
        RAISE EXCEPTION 'active scope/pattern/action unique index is missing or invalid';
    END IF;

    SELECT generation INTO generation_value FROM system_config_cache_generation
        WHERE namespace='message.mail' AND scope_key='global' FOR UPDATE;
    IF generation_value IS NULL OR generation_value < 1 THEN
        RAISE EXCEPTION 'message.mail/global generation is missing or invalid';
    END IF;

    IF scope_type='smallint' AND action_type='smallint' THEN
        IF EXISTS (SELECT 1 FROM message_mail_recipient_rule
                   WHERE scope NOT IN (0,1) OR action NOT IN (0,1)) THEN
            RAISE EXCEPTION 'unknown numeric recipient rule value';
        END IF;
        IF (SELECT count(*) FROM pg_constraint WHERE conrelid=target AND contype='c' AND convalidated
            AND ((conname='message_mail_recipient_rule_scope_check' AND pg_get_constraintdef(oid)='CHECK ((scope = ANY (ARRAY[0, 1])))')
              OR (conname='message_mail_recipient_rule_action_check' AND pg_get_constraintdef(oid)='CHECK ((action = ANY (ARRAY[0, 1])))'))) <> 2 THEN
            RAISE EXCEPTION 'numeric recipient rule constraints do not match the contract';
        END IF;
        RETURN;
    END IF;
    IF scope_type IS DISTINCT FROM 'character varying' OR action_type IS DISTINCT FROM 'character varying' THEN
        RAISE EXCEPTION 'unsupported or partially migrated scope/action column types';
    END IF;
    IF EXISTS (SELECT 1 FROM message_mail_recipient_rule
               WHERE scope NOT IN ('email','domain') OR action NOT IN ('allow','deny')) THEN
        RAISE EXCEPTION 'unknown recipient rule value; refusing to guess a mapping';
    END IF;

    ALTER TABLE message_mail_recipient_rule
        DROP CONSTRAINT IF EXISTS message_mail_recipient_rule_scope_check,
        DROP CONSTRAINT IF EXISTS message_mail_recipient_rule_action_check;
    ALTER TABLE message_mail_recipient_rule
        ALTER COLUMN scope TYPE smallint USING CASE scope WHEN 'email' THEN 0 WHEN 'domain' THEN 1 END,
        ALTER COLUMN action TYPE smallint USING CASE action WHEN 'deny' THEN 0 WHEN 'allow' THEN 1 END;
    ALTER TABLE message_mail_recipient_rule
        ADD CONSTRAINT message_mail_recipient_rule_scope_check CHECK(scope IN (0,1)),
        ADD CONSTRAINT message_mail_recipient_rule_action_check CHECK(action IN (0,1));

    UPDATE system_config_cache_generation SET generation=generation+1, updated_at=changed_at
        WHERE namespace='message.mail' AND scope_key='global' RETURNING generation INTO generation_value;
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,attempts,available_at,created_at,updated_at)
        VALUES ('message.mail','global',generation_value,0,changed_at,changed_at,changed_at);
END
$migration$;
COMMIT;
