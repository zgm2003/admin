-- Manual forward migration. Use psql -X -v ON_ERROR_STOP=1 --single-transaction --file=...
-- Existing expires_at, payment status, amounts and settlement facts never change.
DO $migration$
DECLARE
    expected_check TEXT;
    existing_check TEXT;
    inserted_count BIGINT;
    repaired_count BIGINT;
    previous_generation BIGINT;
    next_generation BIGINT;
BEGIN
    PERFORM set_config('lock_timeout', '5s', true);
    LOCK TABLE system_setting IN SHARE ROW EXCLUSIVE MODE;

    -- Ask PostgreSQL to normalize the exact CHECK expression for drift checks.
    CREATE TEMP TABLE admin_order_expiry_expected (LIKE system_setting) ON COMMIT DROP;
    ALTER TABLE admin_order_expiry_expected ADD CONSTRAINT ck_expected CHECK (
        setting_key <> 'payment.order.expiry_minutes' OR
        CASE WHEN value ~ '^[0-9]{1,4}$' THEN
            value::INTEGER >= 1 AND value::INTEGER <= 1440 AND value_type = 2
            AND status = 1 AND is_builtin = 1 AND deleted_at IS NULL
        ELSE FALSE END
    );
    SELECT pg_get_constraintdef(oid) INTO STRICT expected_check FROM pg_constraint
    WHERE conrelid = 'pg_temp.admin_order_expiry_expected'::REGCLASS AND conname = 'ck_expected';
    SELECT pg_get_constraintdef(oid) INTO existing_check FROM pg_constraint
    WHERE conrelid = 'system_setting'::REGCLASS AND conname = 'ck_system_setting_payment_order_expiry';
    IF existing_check IS NULL THEN
        EXECUTE 'ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_payment_order_expiry ' || expected_check;
    ELSIF existing_check <> expected_check THEN
        RAISE EXCEPTION 'Payment order expiry constraint drift';
    END IF;
    ALTER TABLE system_setting VALIDATE CONSTRAINT ck_system_setting_payment_order_expiry;
    DROP TABLE pg_temp.admin_order_expiry_expected;

    INSERT INTO system_setting (
        setting_key, value, value_type, description, status, is_builtin, created_at, updated_at
    ) SELECT 'payment.order.expiry_minutes', '30', 2,
        '支付订单有效期（分钟，1～1440，仅影响新订单）', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
    WHERE NOT EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'payment.order.expiry_minutes');
    GET DIAGNOSTICS inserted_count = ROW_COUNT;

    -- Repair old pre-expiry schedules once. Preserve post-expiry failure backoff
    -- and active claims; new Worker code caps subsequent pre-expiry schedules.
    UPDATE payment_order SET next_check_at = expires_at, updated_at = CURRENT_TIMESTAMP
    WHERE status = 1 AND next_check_at > expires_at AND updated_at < expires_at
        AND (check_lease_until IS NULL OR check_lease_until <= CURRENT_TIMESTAMP);
    GET DIAGNOSTICS repaired_count = ROW_COUNT;

    IF inserted_count > 0 THEN
        SELECT generation INTO STRICT previous_generation FROM system_config_cache_state
        WHERE namespace = 'system.setting' AND scope_key = 'global' FOR UPDATE;
        IF previous_generation < 1 OR previous_generation = 9223372036854775807 THEN
            RAISE EXCEPTION 'Setting cache generation cannot advance';
        END IF;
        next_generation := previous_generation + 1;
        UPDATE system_config_cache_state SET generation = next_generation, updated_at = CURRENT_TIMESTAMP
        WHERE namespace = 'system.setting' AND scope_key = 'global';
        INSERT INTO system_config_cache_outbox (
            namespace, scope_key, generation, attempts, available_at, created_at, updated_at
        ) VALUES ('system.setting', 'global', next_generation, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    END IF;
    RAISE NOTICE 'Payment order expiry setting inserted=%, old schedules repaired=%, new setting generation=%',
        inserted_count, repaired_count, next_generation;
END
$migration$;
