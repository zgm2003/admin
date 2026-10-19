-- 平台路由隔离：只迁移回调 URL，不修改支付凭据、版本坐标、订单或菜单授权。
-- 停止全部 API/Worker 后必须由同名 PowerShell runner 执行；runner 负责旧待支付订单预检和 Redis 同步。
-- 禁止直接运行 SQL 或由 API/Worker 启动自动运行。
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
SELECT pg_advisory_xact_lock(hashtextextended(current_schema() || ':platform-api-routes-2026-10-18', 0));
LOCK TABLE payment_order IN SHARE MODE;
LOCK TABLE payment_config_version IN ACCESS EXCLUSIVE MODE;

DO $migration$
DECLARE changed bigint; current_generation bigint;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='payment_config_version'::regclass
    AND tgname='payment_config_version_immutable' AND NOT tgisinternal AND tgenabled='O') THEN
    RAISE EXCEPTION 'payment credential immutable trigger is missing or disabled';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='payment_config_version'::regclass
    AND conname='ck_payment_config_version_notify_url') THEN
    RAISE EXCEPTION 'payment callback constraint is missing';
  END IF;
  IF EXISTS (
    SELECT 1
    FROM payment_order order_row
    JOIN payment_config_version version_row
      ON version_row.config_id = order_row.config_id AND version_row.version = order_row.config_version
    WHERE order_row.status = 1
      AND version_row.notify_url ~ '/api/v1/payment/callback/alipay/'
  ) THEN
    RAISE EXCEPTION 'pending payment orders still target the legacy callback URL; settle or close them before migration';
  END IF;
  IF EXISTS (SELECT 1 FROM payment_config_version WHERE NOT (
    notify_url ~ ('^https://[^/?#@]+/api/(admin/)?v1/payment/callback/alipay/' || config_id || '/' || version || '$')
  )) THEN
    RAISE EXCEPTION 'unexpected callback URL or coordinates; no rows migrated';
  END IF;
  SELECT generation INTO current_generation FROM system_config_cache_state
    WHERE namespace='payment.config' AND scope_key='global' FOR UPDATE;
  IF current_generation IS NULL OR current_generation < 1 THEN
    RAISE EXCEPTION 'payment.config/global cache generation is missing';
  END IF;

  ALTER TABLE payment_config_version DROP CONSTRAINT ck_payment_config_version_notify_url;
  -- ACCESS EXCLUSIVE 锁覆盖整个临时例外；失败时触发器及所有数据随事务恢复。
  ALTER TABLE payment_config_version DISABLE TRIGGER payment_config_version_immutable;
  UPDATE payment_config_version
    SET notify_url=regexp_replace(notify_url, '/api/v1/payment/callback/alipay/', '/api/admin/v1/payment/callback/alipay/')
    WHERE notify_url ~ ('^https://[^/?#@]+/api/v1/payment/callback/alipay/' || config_id || '/' || version || '$');
  GET DIAGNOSTICS changed = ROW_COUNT;
  ALTER TABLE payment_config_version ENABLE TRIGGER payment_config_version_immutable;
  ALTER TABLE payment_config_version ADD CONSTRAINT ck_payment_config_version_notify_url CHECK (
    notify_url ~ ('^https://[^/?#@]+/api/admin/v1/payment/callback/alipay/' || config_id || '/' || version || '$')
  );

  IF changed > 0 THEN
    UPDATE system_config_cache_state SET generation=current_generation+1, updated_at=CURRENT_TIMESTAMP
      WHERE namespace='payment.config' AND scope_key='global';
    INSERT INTO system_config_cache_outbox(namespace,scope_key,generation,attempts,available_at,created_at,updated_at)
      VALUES ('payment.config','global',current_generation+1,0,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
  END IF;
  RAISE NOTICE 'platform API migration: % credential versions changed, generation % -> %',
    changed,current_generation,current_generation+CASE WHEN changed>0 THEN 1 ELSE 0 END;
END $migration$;
