-- Offline forward seed. Use the same-name runner with API/Worker stopped.
-- No ledger/config/menu writes and no runtime-startup DDL or seed.
-- P1/P2 payment tables must already have been installed and verified separately.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
LOCK TABLE system_scheduler_schedule IN SHARE ROW EXCLUSIVE MODE;

DO $payment_reconcile$
DECLARE
    required_table text;
BEGIN
    FOREACH required_table IN ARRAY ARRAY[
        'payment_config', 'payment_config_version', 'payment_recharge_package',
        'payment_order', 'payment_wallet', 'payment_transaction', 'payment_callback_event'
    ] LOOP
        IF to_regclass(format('%I.%I', current_schema(), required_table)) IS NULL THEN
            RAISE EXCEPTION 'payment reconcile prerequisite table missing: %', required_table;
        END IF;
    END LOOP;

    IF EXISTS (
        SELECT 1 FROM system_scheduler_schedule
        WHERE builtin_key = 'payment.order.reconcile'
          AND (deleted_at IS NOT NULL OR task_type <> 'payment.order.reconcile' OR params <> '{}'::jsonb)
    ) OR (SELECT count(*) FROM system_scheduler_schedule WHERE builtin_key = 'payment.order.reconcile') > 1
      OR EXISTS (
        SELECT 1 FROM system_scheduler_schedule
        WHERE task_type = 'payment.order.reconcile'
          AND builtin_key IS DISTINCT FROM 'payment.order.reconcile'
    ) THEN
        RAISE EXCEPTION 'payment reconcile schedule conflicts with existing facts';
    END IF;

    INSERT INTO system_scheduler_schedule (
        name, description, task_type, cron_expression, timezone, params,
        is_enabled, next_run_at, builtin_key, created_at, updated_at
    )
    SELECT '支付订单巡检', '恢复待处理回调、核实待支付订单并关闭已过期未支付交易',
           'payment.order.reconcile', '* * * * *', 'UTC', '{}'::jsonb,
           1, date_trunc('minute', CURRENT_TIMESTAMP) + interval '1 minute',
           'payment.order.reconcile', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
    WHERE NOT EXISTS (
        SELECT 1 FROM system_scheduler_schedule WHERE builtin_key = 'payment.order.reconcile'
    );
END $payment_reconcile$;
