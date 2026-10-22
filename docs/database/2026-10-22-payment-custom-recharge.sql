-- 钱包自定义充值：订单无套餐关联；最低金额使用系统设置与既有 generation/outbox。
-- 执行前停止 API/Worker 并备份，使用 psql --single-transaction --set=ON_ERROR_STOP=1。
-- 不改历史订单、套餐定价、支付凭据、权限或资金流水，不在进程启动时执行。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

DO $migration$
DECLARE
    inserted_count integer;
    next_generation bigint;
    expected_check text;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('payment.custom_recharge.migration', 0));
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'payment_order'::regclass AND conname = 'fk_payment_order_package'
          AND confrelid = 'payment_recharge_package'::regclass AND contype = 'f' AND convalidated
    ) THEN
        RAISE EXCEPTION 'payment order package foreign key is missing or incompatible';
    END IF;
    PERFORM generation FROM system_config_cache_state
        WHERE namespace = 'system.setting' AND scope_key = 'global' FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'system.setting/global generation is missing';
    END IF;

    ALTER TABLE payment_order ALTER COLUMN package_id DROP NOT NULL;
    CREATE TEMP TABLE payment_custom_order_contract (LIKE payment_order) ON COMMIT DROP;
    ALTER TABLE payment_custom_order_contract ADD CONSTRAINT expected_check CHECK (package_id IS NULL OR package_id > 0);
    SELECT pg_get_constraintdef(oid) INTO expected_check FROM pg_constraint
        WHERE conrelid = 'pg_temp.payment_custom_order_contract'::regclass AND conname = 'expected_check';
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'payment_order'::regclass AND conname = 'ck_payment_order_package_id') THEN
        EXECUTE format('ALTER TABLE payment_order ADD CONSTRAINT ck_payment_order_package_id %s', expected_check);
    ELSIF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'payment_order'::regclass AND conname = 'ck_payment_order_package_id' AND contype = 'c' AND convalidated AND pg_get_constraintdef(oid) = expected_check) THEN
        RAISE EXCEPTION 'existing order package CHECK is incompatible';
    END IF;
    COMMENT ON COLUMN payment_order.package_id IS '套餐充值关联套餐；自定义金额充值为 NULL，金额在订单中冻结';

    INSERT INTO system_setting(setting_key, value, value_type, description, is_enabled, is_builtin, created_at, updated_at)
    SELECT 'payment.recharge.minimum_amount', '10', 2, '最低自定义充值金额（元，正整数）', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
    WHERE NOT EXISTS (SELECT 1 FROM system_setting WHERE setting_key = 'payment.recharge.minimum_amount');
    GET DIAGNOSTICS inserted_count = ROW_COUNT;

    IF NOT EXISTS (
        SELECT 1 FROM system_setting WHERE setting_key = 'payment.recharge.minimum_amount'
          AND value_type = 2 AND is_enabled = 1 AND is_builtin = 1 AND deleted_at IS NULL
          AND CASE WHEN value ~ '^[0-9]{1,11}$' THEN value::numeric BETWEEN 1 AND 92233720368 ELSE false END
    ) THEN
        RAISE EXCEPTION 'existing minimum recharge setting is incompatible; inspect instead of overwriting';
    END IF;
    CREATE TEMP TABLE payment_custom_setting_contract (LIKE system_setting) ON COMMIT DROP;
    ALTER TABLE payment_custom_setting_contract ADD CONSTRAINT expected_check CHECK (
            setting_key <> 'payment.recharge.minimum_amount' OR
            CASE WHEN value ~ '^[0-9]{1,11}$' THEN
                value::numeric >= 1 AND value::numeric <= 92233720368 AND value_type = 2
                AND is_enabled = 1 AND is_builtin = 1 AND deleted_at IS NULL
            ELSE false END
    );
    SELECT pg_get_constraintdef(oid) INTO expected_check FROM pg_constraint
        WHERE conrelid = 'pg_temp.payment_custom_setting_contract'::regclass AND conname = 'expected_check';
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_setting'::regclass AND conname = 'ck_system_setting_recharge_minimum_amount') THEN
        EXECUTE format('ALTER TABLE system_setting ADD CONSTRAINT ck_system_setting_recharge_minimum_amount %s', expected_check);
    ELSIF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'system_setting'::regclass AND conname = 'ck_system_setting_recharge_minimum_amount' AND contype = 'c' AND convalidated AND pg_get_constraintdef(oid) = expected_check) THEN
        RAISE EXCEPTION 'existing recharge minimum CHECK is incompatible';
    END IF;

    IF inserted_count > 0 THEN
        UPDATE system_config_cache_state SET generation = generation + 1, updated_at = CURRENT_TIMESTAMP
            WHERE namespace = 'system.setting' AND scope_key = 'global' RETURNING generation INTO next_generation;
        INSERT INTO system_config_cache_outbox(namespace, scope_key, generation, attempts, available_at, created_at, updated_at)
            VALUES ('system.setting', 'global', next_generation, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
    END IF;
END
$migration$;

-- Worker 的既有 outbox relay 恢复后发布新代际；不执行 FLUSHDB 或删除任意 Redis key。
