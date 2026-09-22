BEGIN;

DO $$
DECLARE
    changed_count BIGINT;
    next_generation BIGINT;
BEGIN
    WITH changed AS (
        INSERT INTO system_setting (
            setting_key,
            value,
            value_type,
            description,
            is_enabled,
            is_builtin,
            created_at,
            updated_at
        )
        VALUES
            ('app.legal.user_agreement', '', 1, 'Published user agreement HTML', 1, 1, statement_timestamp(), statement_timestamp()),
            ('app.legal.privacy_policy', '', 1, 'Published privacy policy HTML', 1, 1, statement_timestamp(), statement_timestamp())
        ON CONFLICT (setting_key) WHERE deleted_at IS NULL DO UPDATE
        SET
            value_type = EXCLUDED.value_type,
            description = EXCLUDED.description,
            is_enabled = EXCLUDED.is_enabled,
            is_builtin = EXCLUDED.is_builtin,
            updated_at = EXCLUDED.updated_at
        WHERE system_setting.value_type <> EXCLUDED.value_type
           OR system_setting.description <> EXCLUDED.description
           OR system_setting.is_enabled <> EXCLUDED.is_enabled
           OR system_setting.is_builtin <> EXCLUDED.is_builtin
        RETURNING 1
    )
    SELECT count(*) INTO changed_count FROM changed;

    IF changed_count > 0 THEN
        UPDATE system_config_cache_generation
        SET generation = generation + 1,
            updated_at = statement_timestamp()
        WHERE namespace = 'system.setting'
          AND scope_key = 'global'
        RETURNING generation INTO next_generation;

        IF next_generation IS NULL THEN
            RAISE EXCEPTION 'system.setting/global cache generation is missing';
        END IF;

        INSERT INTO system_config_cache_outbox (
            namespace,
            scope_key,
            generation,
            attempts,
            available_at,
            last_error,
            created_at,
            updated_at
        )
        VALUES (
            'system.setting',
            'global',
            next_generation,
            0,
            statement_timestamp(),
            '',
            statement_timestamp(),
            statement_timestamp()
        );
    END IF;
END
$$;

COMMIT;
