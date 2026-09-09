BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

LOCK TABLE
    message_mail_config,
    message_mail_template,
    message_mail_recipient_rule
IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF to_regclass('message_mail_config') IS NULL
       OR to_regclass('message_mail_template') IS NULL
       OR to_regclass('message_mail_recipient_rule') IS NULL THEN
        RAISE EXCEPTION 'mail configuration tables are incomplete';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'message_mail_config'
          AND column_name = 'platform_id'
    ) AND (SELECT count(*) FROM message_mail_config WHERE deleted_at IS NULL) > 1 THEN
        RAISE EXCEPTION 'multiple active mail configurations cannot be made global';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'message_mail_template'
          AND column_name = 'platform_id'
    ) AND EXISTS (
        SELECT 1
        FROM message_mail_template
        WHERE deleted_at IS NULL
        GROUP BY scene
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate active mail template scenes cannot be made global';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'message_mail_recipient_rule'
          AND column_name = 'platform_id'
    ) AND EXISTS (
        SELECT 1
        FROM message_mail_recipient_rule
        WHERE deleted_at IS NULL
        GROUP BY scope, pattern, action
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate active mail recipient rules cannot be made global';
    END IF;
END
$$;

DROP INDEX IF EXISTS ux_message_mail_config_platform_active;
DROP INDEX IF EXISTS ux_message_mail_template_platform_scene_active;
DROP INDEX IF EXISTS ux_message_mail_rule_platform_scope_pattern_action_active;

ALTER TABLE message_mail_config
    DROP CONSTRAINT IF EXISTS message_mail_config_platform_id_fkey,
    DROP COLUMN IF EXISTS platform_id;

ALTER TABLE message_mail_template
    DROP CONSTRAINT IF EXISTS message_mail_template_platform_id_fkey,
    DROP COLUMN IF EXISTS platform_id;

ALTER TABLE message_mail_recipient_rule
    DROP CONSTRAINT IF EXISTS message_mail_recipient_rule_platform_id_fkey,
    DROP COLUMN IF EXISTS platform_id;

CREATE UNIQUE INDEX IF NOT EXISTS ux_message_mail_config_active_singleton
    ON message_mail_config ((true))
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_message_mail_template_scene_active
    ON message_mail_template (scene)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_message_mail_rule_scope_pattern_action_active
    ON message_mail_recipient_rule (scope, pattern, action)
    WHERE deleted_at IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name IN ('message_mail_config', 'message_mail_template', 'message_mail_recipient_rule')
          AND column_name = 'platform_id'
    ) THEN
        RAISE EXCEPTION 'mail configuration platform ownership was not removed';
    END IF;

    IF to_regclass('ux_message_mail_config_active_singleton') IS NULL
       OR to_regclass('ux_message_mail_template_scene_active') IS NULL
       OR to_regclass('ux_message_mail_rule_scope_pattern_action_active') IS NULL THEN
        RAISE EXCEPTION 'global mail uniqueness indexes are incomplete';
    END IF;
END
$$;

COMMIT;
